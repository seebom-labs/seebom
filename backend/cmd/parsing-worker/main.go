package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	json "github.com/goccy/go-json"

	"github.com/seebom-labs/seebom/backend/internal/clickhouse"
	"github.com/seebom-labs/seebom/backend/internal/config"
	gh "github.com/seebom-labs/seebom/backend/internal/github"
	"github.com/seebom-labs/seebom/backend/internal/license"
	"github.com/seebom-labs/seebom/backend/internal/osv"
	"github.com/seebom-labs/seebom/backend/internal/osvutil"
	"github.com/seebom-labs/seebom/backend/internal/spdx"
	"github.com/seebom-labs/seebom/backend/internal/vex"
	"github.com/seebom-labs/seebom/backend/pkg/models"
)

func main() {
	log.Println("SeeBOM Parsing Worker starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	chClient, err := clickhouse.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer chClient.Close()

	osvClient := osv.NewClient()

	// Load license policy (permissive/copyleft lists) if available.
	if perm, copy, err := license.LoadPolicy(cfg.LicensePolicyFile); err == nil {
		log.Printf("Loaded license policy from %s (%d permissive, %d copyleft)", cfg.LicensePolicyFile, perm, copy)
	} else {
		log.Printf("Using default license policy (tried %s): %v", cfg.LicensePolicyFile, err)
	}

	// Load license exceptions if available (try config path, then SBOM dir fallback).
	var exceptionsIndex *license.ExceptionIndex
	sbomDirExceptionsPath := cfg.SBOMDir + "/license-exceptions.json"
	if idx, err := license.LoadExceptionsWithFallback(cfg.ExceptionsFile, sbomDirExceptionsPath); err == nil {
		exceptionsIndex = idx
		log.Printf("Loaded license exceptions (%d blanket, %d specific)",
			len(idx.Raw.BlanketExceptions), len(idx.Raw.Exceptions))
	} else {
		log.Printf("No license exceptions loaded (tried %s, %s): %v", cfg.ExceptionsFile, sbomDirExceptionsPath, err)
	}

	// Initialize GitHub license resolver for unknown licenses.
	var ghResolver *gh.Resolver
	if !cfg.SkipGitHubResolve {
		ghResolver = gh.NewResolver(cfg.GitHubToken)
		// Preload license cache from ClickHouse.
		if cached, err := chClient.QueryGitHubLicenseCache(context.Background()); err == nil && len(cached) > 0 {
			ghResolver.PreloadCache(cached)
			log.Printf("Preloaded %d GitHub license cache entries", len(cached))
		}
		// Preload repo metadata cache.
		if meta, err := chClient.QueryGitHubRepoMetadata(context.Background()); err == nil && len(meta) > 0 {
			ghResolver.PreloadMetadataCache(meta)
			log.Printf("Preloaded %d GitHub repo metadata entries", len(meta))
		}
		if cfg.GitHubToken != "" {
			log.Println("GitHub license resolver enabled (authenticated, 5000 req/h)")
		} else {
			log.Println("GitHub license resolver enabled (unauthenticated, 60 req/h)")
		}
	} else {
		log.Println("GitHub license resolver disabled (SKIP_GITHUB_RESOLVE=true)")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Printf("Worker %s polling for jobs (batch size: %d, skip_osv: %v)...", cfg.WorkerID, cfg.WorkerBatchSize, cfg.SkipOSV)

	// Main polling loop.
	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down.")
			return
		default:
		}

		jobs, err := chClient.ClaimJobs(ctx, cfg.WorkerID, cfg.WorkerBatchSize)
		if err != nil {
			log.Printf("ERROR: Failed to claim jobs: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if len(jobs) == 0 {
			// No work available, wait before polling again.
			time.Sleep(10 * time.Second)
			continue
		}

		log.Printf("Claimed %d jobs, processing...", len(jobs))

		for _, job := range jobs {
			if err := processJob(ctx, cfg, chClient, osvClient, exceptionsIndex, ghResolver, job); err != nil {
				log.Printf("ERROR: Failed to process %s: %v", job.SourceFile, err)
				if failErr := chClient.FailJob(ctx, job, err.Error()); failErr != nil {
					log.Printf("ERROR: Failed to mark job as failed: %v", failErr)
				}
				continue
			}

			if err := chClient.CompleteJob(ctx, job); err != nil {
				log.Printf("ERROR: Failed to mark job as done: %v", err)
			} else {
				log.Printf("Completed: %s", job.SourceFile)
			}
		}
	}
}

func processJob(ctx context.Context, cfg *config.Config, chClient *clickhouse.Client, osvClient *osv.Client, exceptions *license.ExceptionIndex, ghResolver *gh.Resolver, job models.IngestionJob) error {
	absPath := filepath.Join(cfg.SBOMDir, job.SourceFile)

	// Branch on job type.
	if job.JobType == models.JobTypeVEX {
		return processVEXJob(ctx, chClient, absPath, job)
	}

	return processSBOMJob(ctx, cfg, chClient, osvClient, exceptions, ghResolver, absPath, job)
}

func processVEXJob(ctx context.Context, chClient *clickhouse.Client, absPath string, job models.IngestionJob) error {
	result, err := vex.ParseFile(absPath, job.SourceFile)
	if err != nil {
		return err
	}

	log.Printf("  Parsed VEX %s: %d statements (doc: %s)",
		job.SourceFile, len(result.Statements), result.DocumentID)

	// Skip if this VEX document was already ingested (idempotency guard).
	if exists, _ := chClient.VEXDocumentExists(ctx, result.DocumentID); exists {
		log.Printf("  Skipping VEX %s (already ingested, doc: %s)", job.SourceFile, result.DocumentID)
		return nil
	}

	if len(result.Statements) > 0 {
		if err := chClient.InsertVEXStatements(ctx, result.Statements); err != nil {
			return err
		}
	}

	return nil
}

func processSBOMJob(ctx context.Context, cfg *config.Config, chClient *clickhouse.Client, osvClient *osv.Client, exceptions *license.ExceptionIndex, ghResolver *gh.Resolver, absPath string, job models.IngestionJob) error {

	// 1. Parse the SPDX file.
	result, err := spdx.ParseFile(absPath, job.SourceFile, job.SHA256Hash)
	if err != nil {
		return err
	}

	log.Printf("  Parsed %s: %d packages, %d relationships",
		job.SourceFile,
		len(result.Packages.PackageNames),
		len(result.Packages.RelSourceIndices))

	// 1b. Skip if this SBOM was already fully ingested (idempotency guard).
	if exists, _ := chClient.SBOMExists(ctx, result.SBOM.SBOMID); exists {
		log.Printf("  Skipping %s (already ingested, sbom_id=%s)", job.SourceFile, result.SBOM.SBOMID)
		return nil
	}

	// 2. Insert SBOM metadata.
	if err := chClient.InsertSBOM(ctx, &result.SBOM); err != nil {
		return err
	}

	// 3. Insert package arrays.
	if err := chClient.InsertSBOMPackages(ctx, &result.Packages); err != nil {
		return err
	}

	// 4. OSV batch query for vulnerabilities (skippable for fast ingestion).
	if cfg.SkipOSV {
		log.Printf("  SKIP_OSV=true, skipping vulnerability scan for %s (%d PURLs)", job.SourceFile, len(result.Packages.PackagePURLs))
	} else {
		// Filter out empty PURLs first.
		var purls []string
		purlToIndex := make(map[string]int) // track which PURL maps to which query index
		for _, purl := range result.Packages.PackagePURLs {
			if purl != "" {
				purlToIndex[purl] = len(purls)
				purls = append(purls, purl)
			}
		}

		var vulns []models.Vulnerability
		if len(purls) > 0 {
			// Chunk PURLs into batches of 1000 (OSV API limit).
			const chunkSize = 1000
			for i := 0; i < len(purls); i += chunkSize {
				end := i + chunkSize
				if end > len(purls) {
					end = len(purls)
				}
				chunk := purls[i:end]

				osvResp, err := osvClient.QueryBatch(ctx, chunk)
				if err != nil {
					log.Printf("  WARNING: OSV query failed for chunk %d-%d: %v", i, end, err)
					continue
				}

				// Map OSV results back to PURLs and build vulnerability models.
				for j, qr := range osvResp.Results {
					if len(qr.Vulns) == 0 {
						continue
					}
					purl := chunk[j]
					for _, v := range qr.Vulns {
						severity := osvutil.ClassifySeverity(v)
						fixedVersion := osvutil.ExtractFixedVersion(v)
						affectedVersions := osvutil.ExtractAffectedVersions(v)

						// Serialize raw OSV JSON for detail view.
						rawJSON, _ := json.Marshal(v)

						vulns = append(vulns, models.Vulnerability{
							DiscoveredAt:     time.Now(),
							SBOMID:           result.SBOM.SBOMID,
							SourceFile:       job.SourceFile,
							PURL:             purl,
							VulnID:           v.ID,
							Severity:         severity,
							Summary:          v.Summary,
							AffectedVersions: affectedVersions,
							FixedVersion:     fixedVersion,
							OSVJSON:          string(rawJSON),
						})
					}
				}
			}

			if len(vulns) > 0 {
				log.Printf("  Found %d vulnerabilities in %s", len(vulns), job.SourceFile)
				if err := chClient.InsertVulnerabilities(ctx, vulns); err != nil {
					return err
				}
			}
		}
	}

	// 4b. Resolve unknown licenses via GitHub API + check for archived repos.
	if ghResolver != nil {
		resolved := 0
		archived := 0
		for i, lic := range result.Packages.PackageLicenses {
			purl := ""
			if i < len(result.Packages.PackagePURLs) {
				purl = result.Packages.PackagePURLs[i]
			}
			if purl == "" {
				continue
			}

			// For unknown licenses, fetch full metadata (license + archived status)
			if lic == "" || lic == "NOASSERTION" || lic == "NONE" {
				if meta := ghResolver.ResolveWithMetadata(ctx, purl); meta != nil {
					if meta.SPDXID != "" {
						result.Packages.PackageLicenses[i] = meta.SPDXID
						resolved++
					}
					if meta.Archived {
						archived++
					}
				}
			}
		}
		if resolved > 0 {
			log.Printf("  Resolved %d unknown licenses via GitHub API", resolved)
		}
		if archived > 0 {
			log.Printf("  ⚠️  Found %d packages using ARCHIVED GitHub repos", archived)
		}
		// Persist caches to ClickHouse.
		if entries := ghResolver.CacheEntries(); len(entries) > 0 {
			_ = chClient.InsertGitHubLicenseCache(ctx, entries)
		}
		if metaEntries := ghResolver.MetadataCacheEntries(); len(metaEntries) > 0 {
			_ = chClient.InsertGitHubRepoMetadata(ctx, metaEntries)
		}
	}

	// 5. License compliance check.
	licResults := license.CheckWithExceptions(result.Packages.PackageNames, result.Packages.PackageLicenses, exceptions)
	if len(licResults) > 0 {
		licModels := make([]models.LicenseCompliance, len(licResults))
		for i, lr := range licResults {
			nonCompliant := lr.NonCompliantPackages
			if nonCompliant == nil {
				nonCompliant = []string{}
			}
			exempted := lr.ExemptedPackages
			if exempted == nil {
				exempted = []string{}
			}
			licModels[i] = models.LicenseCompliance{
				CheckedAt:            time.Now(),
				SBOMID:               result.SBOM.SBOMID,
				SourceFile:           job.SourceFile,
				LicenseID:            lr.LicenseID,
				Category:             string(lr.Category),
				PackageCount:         lr.PackageCount,
				NonCompliantPackages: nonCompliant,
				ExemptedPackages:     exempted,
				ExemptionReason:      lr.ExemptionReason,
			}
		}
		if err := chClient.InsertLicenseCompliance(ctx, licModels); err != nil {
			return err
		}
	}

	return nil
}
