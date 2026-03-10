package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/seebom-labs/seebom/backend/internal/clickhouse"
	"github.com/seebom-labs/seebom/backend/internal/config"
	"github.com/seebom-labs/seebom/backend/internal/repo"
	"github.com/seebom-labs/seebom/backend/pkg/models"
)

func main() {
	log.Println("SeeBOM Ingestion Watcher starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	chClient, err := clickhouse.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer chClient.Close()

	ctx := context.Background()

	// Scan the SBOM directory for SPDX JSON and VEX files.
	scanner := repo.NewScanner(cfg.SBOMDir)
	files, err := scanner.Scan()
	if err != nil {
		log.Fatalf("Failed to scan SBOM directory: %v", err)
	}

	sbomCount := 0
	vexCount := 0
	for _, f := range files {
		if f.FileType == "vex" {
			vexCount++
		} else {
			sbomCount++
		}
	}
	log.Printf("Found %d SBOM files and %d VEX files in %s", sbomCount, vexCount, cfg.SBOMDir)

	// Apply SBOM_LIMIT only to SBOM files (VEX files are always included).
	if cfg.SBOMLimit > 0 && sbomCount > cfg.SBOMLimit {
		log.Printf("SBOM_LIMIT=%d set, truncating SBOMs from %d to %d", cfg.SBOMLimit, sbomCount, cfg.SBOMLimit)
		var limited []repo.FileInfo
		count := 0
		for _, f := range files {
			if f.FileType == "vex" {
				limited = append(limited, f) // Always include VEX files
			} else if count < cfg.SBOMLimit {
				limited = append(limited, f)
				count++
			}
		}
		files = limited
	}

	// Deduplicate against existing hashes in ClickHouse.
	var newJobs []models.IngestionJob
	for _, f := range files {
		exists, err := chClient.HashExists(ctx, f.SHA256Hash)
		if err != nil {
			log.Printf("WARNING: Failed to check hash for %s: %v", f.RelPath, err)
			continue
		}
		if exists {
			continue
		}

		jobType := models.JobTypeSBOM
		if f.FileType == "vex" {
			jobType = models.JobTypeVEX
		}

		newJobs = append(newJobs, models.IngestionJob{
			CreatedAt:  time.Now(),
			JobID:      uuid.New(),
			SourceFile: f.RelPath,
			SHA256Hash: f.SHA256Hash,
			Status:     models.JobStatusPending,
			JobType:    jobType,
		})
	}

	if len(newJobs) == 0 {
		log.Println("No new SBOM files to ingest. Exiting.")
		return
	}

	log.Printf("Enqueuing %d new ingestion jobs...", len(newJobs))

	// Batch insert jobs into the queue.
	if err := chClient.EnqueueJobs(ctx, newJobs); err != nil {
		log.Fatalf("Failed to enqueue jobs: %v", err)
	}

	log.Printf("Successfully enqueued %d jobs. Watcher done.", len(newJobs))
}
