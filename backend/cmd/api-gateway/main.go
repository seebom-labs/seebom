package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/seebom-labs/seebom/backend/internal/clickhouse"
	"github.com/seebom-labs/seebom/backend/internal/config"
	"github.com/seebom-labs/seebom/backend/internal/license"
)

func main() {
	log.Println("SeeBOM API Gateway starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	chClient, err := clickhouse.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer chClient.Close()

	exceptionsPath := cfg.ExceptionsFile

	// Load license policy (permissive/copyleft classification).
	if perm, copy, err := license.LoadPolicy(cfg.LicensePolicyFile); err == nil {
		log.Printf("Loaded license policy: %d permissive, %d copyleft", perm, copy)
	} else {
		log.Printf("Using default license policy: %v", err)
	}

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Dashboard stats.
	mux.HandleFunc("GET /api/v1/stats/dashboard", func(w http.ResponseWriter, r *http.Request) {
		stats, err := chClient.QueryDashboardStats(r.Context())
		if err != nil {
			log.Printf("ERROR: dashboard stats: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch dashboard stats")
			return
		}
		writeJSON(w, http.StatusOK, stats)
	})

	// List SBOMs with pagination.
	mux.HandleFunc("GET /api/v1/sboms", func(w http.ResponseWriter, r *http.Request) {
		page := parseUint64(r.URL.Query().Get("page"), 1)
		pageSize := parseUint64(r.URL.Query().Get("page_size"), 50)

		resp, err := chClient.QuerySBOMs(r.Context(), page, pageSize)
		if err != nil {
			log.Printf("ERROR: list sboms: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch SBOMs")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	// List vulnerabilities with pagination and optional VEX filtering.
	mux.HandleFunc("GET /api/v1/vulnerabilities", func(w http.ResponseWriter, r *http.Request) {
		page := parseUint64(r.URL.Query().Get("page"), 1)
		pageSize := parseUint64(r.URL.Query().Get("page_size"), 50)
		vexFilter := r.URL.Query().Get("vex_filter") // "effective" to exclude not_affected

		resp, err := chClient.QueryVulnerabilities(r.Context(), page, pageSize, vexFilter)
		if err != nil {
			log.Printf("ERROR: list vulnerabilities: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch vulnerabilities")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	// License compliance overview.
	mux.HandleFunc("GET /api/v1/licenses/compliance", func(w http.ResponseWriter, r *http.Request) {
		items, err := chClient.QueryLicenseCompliance(r.Context())
		if err != nil {
			log.Printf("ERROR: license compliance: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch license compliance")
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	// SBOM dependency tree.
	mux.HandleFunc("GET /api/v1/sboms/{id}/dependencies", func(w http.ResponseWriter, r *http.Request) {
		sbomID := r.PathValue("id")
		if sbomID == "" {
			writeError(w, http.StatusBadRequest, "Missing SBOM ID")
			return
		}

		nodes, err := chClient.QuerySBOMDependencies(r.Context(), sbomID)
		if err != nil {
			log.Printf("ERROR: sbom dependencies for %s: %v", sbomID, err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch dependencies")
			return
		}
		writeJSON(w, http.StatusOK, nodes)
	})

	// SBOM detail with severity breakdown.
	mux.HandleFunc("GET /api/v1/sboms/{id}/detail", func(w http.ResponseWriter, r *http.Request) {
		sbomID := r.PathValue("id")
		if sbomID == "" {
			writeError(w, http.StatusBadRequest, "Missing SBOM ID")
			return
		}
		detail, err := chClient.QuerySBOMDetail(r.Context(), sbomID)
		if err != nil {
			log.Printf("ERROR: sbom detail for %s: %v", sbomID, err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch SBOM detail")
			return
		}
		writeJSON(w, http.StatusOK, detail)
	})

	// Vulnerabilities for a specific SBOM.
	mux.HandleFunc("GET /api/v1/sboms/{id}/vulnerabilities", func(w http.ResponseWriter, r *http.Request) {
		sbomID := r.PathValue("id")
		if sbomID == "" {
			writeError(w, http.StatusBadRequest, "Missing SBOM ID")
			return
		}
		vulns, err := chClient.QuerySBOMVulnerabilities(r.Context(), sbomID)
		if err != nil {
			log.Printf("ERROR: sbom vulns for %s: %v", sbomID, err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch SBOM vulnerabilities")
			return
		}
		writeJSON(w, http.StatusOK, vulns)
	})

	// Licenses for a specific SBOM.
	mux.HandleFunc("GET /api/v1/sboms/{id}/licenses", func(w http.ResponseWriter, r *http.Request) {
		sbomID := r.PathValue("id")
		if sbomID == "" {
			writeError(w, http.StatusBadRequest, "Missing SBOM ID")
			return
		}
		licenses, err := chClient.QuerySBOMLicenses(r.Context(), sbomID)
		if err != nil {
			log.Printf("ERROR: sbom licenses for %s: %v", sbomID, err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch SBOM licenses")
			return
		}
		writeJSON(w, http.StatusOK, licenses)
	})

	// Projects with license violations (filtered by exceptions).
	mux.HandleFunc("GET /api/v1/projects/license-violations", func(w http.ResponseWriter, r *http.Request) {
		// Load current exceptions for filtering.
		var excIdx *license.ExceptionIndex
		if idx, err := license.LoadExceptions(exceptionsPath); err == nil {
			excIdx = idx
		}
		violations, err := chClient.QueryProjectsWithLicenseViolations(r.Context(), excIdx)
		if err != nil {
			log.Printf("ERROR: license violations: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch license violations")
			return
		}
		writeJSON(w, http.StatusOK, violations)
	})

	// Projects affected by a specific CVE (including transitive dependencies).
	mux.HandleFunc("GET /api/v1/vulnerabilities/{id}/affected-projects", func(w http.ResponseWriter, r *http.Request) {
		vulnID := r.PathValue("id")
		if vulnID == "" {
			writeError(w, http.StatusBadRequest, "Missing vulnerability ID")
			return
		}
		projects, err := chClient.QueryAffectedProjectsByCVE(r.Context(), vulnID)
		if err != nil {
			log.Printf("ERROR: affected projects for %s: %v", vulnID, err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch affected projects")
			return
		}
		writeJSON(w, http.StatusOK, projects)
	})

	// Dependency usage statistics across all projects.
	mux.HandleFunc("GET /api/v1/stats/dependencies", func(w http.ResponseWriter, r *http.Request) {
		limit := parseUint64(r.URL.Query().Get("limit"), 50)
		stats, err := chClient.QueryDependencyStats(r.Context(), limit)
		if err != nil {
			log.Printf("ERROR: dependency stats: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch dependency stats")
			return
		}
		writeJSON(w, http.StatusOK, stats)
	})

	// VEX statements list with pagination.
	mux.HandleFunc("GET /api/v1/vex/statements", func(w http.ResponseWriter, r *http.Request) {
		page := parseUint64(r.URL.Query().Get("page"), 1)
		pageSize := parseUint64(r.URL.Query().Get("page_size"), 50)

		resp, err := chClient.QueryVEXStatements(r.Context(), page, pageSize)
		if err != nil {
			log.Printf("ERROR: list vex statements: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch VEX statements")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	// ── License Exceptions (read-only from config file) ────────────────
	mux.HandleFunc("GET /api/v1/license-exceptions", func(w http.ResponseWriter, r *http.Request) {
		idx, err := license.LoadExceptions(exceptionsPath)
		if err != nil {
			writeJSON(w, http.StatusOK, license.ExceptionsFile{
				Version:           "1.0.0",
				BlanketExceptions: []license.BlanketException{},
				Exceptions:        []license.Exception{},
			})
			return
		}
		writeJSON(w, http.StatusOK, idx.Raw)
	})

	// ── License Policy (read-only, permissive/copyleft classification) ─
	mux.HandleFunc("GET /api/v1/license-policy", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, license.GetPolicy())
	})

	// ── Archived GitHub Packages ───────────────────────────────────────
	mux.HandleFunc("GET /api/v1/packages/archived", func(w http.ResponseWriter, r *http.Request) {
		packages, err := chClient.QueryArchivedPackages(r.Context())
		if err != nil {
			log.Printf("ERROR: archived packages: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to fetch archived packages")
			return
		}
		writeJSON(w, http.StatusOK, packages)
	})

	// CORS middleware for Angular dev server.
	handler := corsMiddleware(mux)

	addr := ":" + strconv.Itoa(cfg.APIPort)
	log.Printf("API Gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ERROR: Failed to encode JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func parseUint64(s string, fallback uint64) uint64 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
