# Role & Project Context
You are an expert Senior Software Engineer and Architect specializing in Go, Angular, Kubernetes, and high-performance analytical databases (ClickHouse).

We are building SeeBOM Labs: a standalone, Kubernetes-native Software Bill of Materials (SBOM) visualization and governance platform. It autonomously ingests massive amounts of SPDX JSON files from the CNCF ecosystem, stores them for infinite historical retention, cross-references vulnerabilities via the OSV API, checks license compliance natively with externalized policy and exception files, supports VEX (Vulnerability Exploitability eXchange) via OpenVEX, and displays the results in a high-performance UI.

# Architecture Overview
The platform consists of **4 Go binaries**, an **Angular UI**, and a **ClickHouse** database:

| Binary | Type | Purpose |
|--------|------|---------|
| `ingestion-watcher` | K8s CronJob | Scans SBOM/VEX directory, hash-dedup, enqueues jobs |
| `parsing-worker` | Deployment (N replicas) | Processes SBOMs (SPDX→ClickHouse), VEX files, OSV lookups, license checks |
| `api-gateway` | Deployment | Stateless REST API (16 endpoints) |
| `cve-refresher` | K8s CronJob (daily) | Checks all known PURLs for newly disclosed CVEs without re-scanning SBOMs |

Key shared packages:
- `internal/clickhouse` – ClickHouse client, batch inserts (`insert.go`), queue operations (`queue.go`), and all query logic split across `queries.go`, `queries_search.go`, `queries_refresh.go`, `queries_github_cache.go`
- `internal/config` – Environment-based configuration loader (`config.Load()` reads env vars with sensible defaults)
- `internal/repo` – Directory scanner with SHA256 hashing and file type classification (`.spdx.json` → sbom, `.openvex.json` → vex)
- `internal/osvutil` – Shared OSV helpers (ClassifySeverity, ExtractFixedVersion, ExtractAffectedVersions)
- `internal/license` – License compliance + externalized policy + exceptions with prefix-matching
- `internal/osv` – OSV API client with rate limiting and exponential backoff
- `internal/github` – GitHub API client for resolving unknown licenses from PURL (rate-limited, cached)
- `internal/spdx` – SPDX JSON streaming parser
- `internal/vex` – OpenVEX parser with URL normalization

Data layer (`pkg/`):
- `pkg/models` – ClickHouse data models (SBOM, SBOMPackages, Vulnerability, LicenseCompliance, IngestionJob, VEXStatement)
- `pkg/dto` – API response DTOs with generics (`PaginatedResponse[T]`), used exclusively by `api-gateway` and `internal/clickhouse` queries

# Tech Stack
- **Backend & Workers:** Go (Golang)
- **Database:** ClickHouse (managed via the official ClickHouse Kubernetes Operator)
- **Frontend:** Angular (TypeScript, standalone components, OnPush change detection)
- **Infrastructure:** Kubernetes (Standard Helm Chart, 18 templates)
- **Container Registry:** GitHub Container Registry (ghcr.io/seebom-labs/seebom/*)
- **Go Module Path:** `github.com/seebom-labs/seebom/backend`

# Architectural Directives
**Monorepo Requirement:** This project strictly uses a monorepo architecture. All Go backend code, Angular frontend code, ClickHouse schemas, and Kubernetes Helm charts must reside in this single repository to maintain full contextual visibility for AI-assisted development. Do not suggest splitting this into a polyrepo.

**Deployment Strategy:** We use a hybrid approach. The custom Go workers and Angular UI are deployed using standard Helm templates (Deployments, CronJobs, Services). However, the ClickHouse database must be provisioned using the official ClickHouse Operator within our Helm chart to properly manage its stateful lifecycle. Do not attempt to write a custom Kubernetes Operator in Go for our application logic.

**Config-Driven Governance:** License policy (`license-policy.json`) and license exceptions (`license-exceptions.json`) are externalized as config files – mounted via Docker Compose volumes locally and Kubernetes ConfigMaps in production. The frontend is public, so no write APIs for exceptions exist. Changes require config file updates + re-ingest.

**CVE Refresh Strategy:** New CVEs are discovered via a lightweight daily CronJob (`cve-refresher`) that queries all unique PURLs (~20k) against the OSV API in 1000-PURL batch chunks, deduplicates against existing vulnerabilities, and inserts new findings. This avoids expensive full re-scans of all SBOMs.

# Executable Commands

## Development (Docker Compose)
```
make dev             # Start full stack (alias for dev-up, prints URLs)
make dev-up          # Start full stack (ClickHouse + Backend + UI)
make dev-down        # Stop and remove containers
make dev-reset       # Wipe ClickHouse data and restart from scratch
make dev-restart     # Restart with new .env values (keeps data)
make dev-status      # Show container status + ingestion progress + data summary
make dev-logs        # Follow Docker Compose logs
make re-ingest       # Re-trigger the Ingestion Watcher
make re-scan         # Wipe vulns/licenses and re-process all SBOMs
make cve-refresh     # Run CVE refresh (check all PURLs for new CVEs)
make migrate         # Run all pending database migrations
make ch-shell        # Open ClickHouse SQL shell
```

## Local Development (without Docker for backend)
```
make ch-only         # Start only ClickHouse in Docker
make ch-migrate      # Run migrations against running ClickHouse
make api             # Run API Gateway locally (needs ClickHouse)
make ingest          # Run Ingestion Watcher once locally
make worker          # Run Parsing Worker locally
make ui-dev          # Start Angular dev server (hot-reload, proxies to localhost:8080)
```

## Kind (Local Kubernetes)
```
make kind-up         # Deploy SeeBOM to a local Kind cluster
make kind-down       # Destroy the local Kind cluster
make kind-build      # Build dev images and load into Kind
make kind-deploy     # Build, load, and upgrade Helm release
make kind-reingest   # Truncate data and re-trigger ingestion in Kind
```

## Build & Test
```
Backend Build:   cd backend && go build ./...
Backend Test:    cd backend && go test ./... -v -count=1
Backend Vet:     cd backend && go fmt ./... && go vet ./...
Frontend Install: cd ui && npm install
Frontend Build:  cd ui && npx ng build --configuration=production
Frontend Test:   cd ui && npx ng test            # uses Vitest
```

# Code Style & Database Best Practices

## Go (Backend)
- Use standard idiomatic Go. Handle errors explicitly; never swallow them.
- HTTP routing uses Go 1.22+ stdlib `net/http` with method-pattern registration (e.g., `mux.HandleFunc("GET /api/v1/sboms", ...)`). No web framework.
- Only 3 direct dependencies: `clickhouse-go/v2`, `goccy/go-json`, `google/uuid`. Keep it minimal.
- Multi-target Dockerfile (`backend/Dockerfile`) builds all 4 binaries in one builder stage, then copies each into a separate `alpine:3.21` runtime stage.
- Prioritize high-performance JSON parsing for the massive SPDX documents (`goccy/go-json`).
- When integrating with the OSV API, utilize batch querying endpoints (`/v1/querybatch`) to efficiently process multiple Package URLs (PURLs) at once.
- Shared OSV processing logic belongs in `internal/osvutil`, not duplicated across binaries.
- License categorization logic belongs in `internal/license` with externalized policy files.
- Permissive licenses (MIT, Apache-2.0, BSD) must **never** generate non-compliant package records.

## ClickHouse (Database)
- Treat observability and SBOM histories as a data analytics problem. Use the MergeTree table engine family for all core tables.
- When designing schemas, ensure the ORDER BY clause starts with low-cardinality columns (e.g., timestamp, category) to minimize data scanning and optimize performance.
- Extract frequently queried JSON keys into top-level columns rather than relying entirely on generic Map or String types.
- Avoid single-row inserts; always aggregate and batch inserts in Go.
- Current tables: `sboms`, `sbom_packages`, `vulnerabilities`, `license_compliance`, `ingestion_queue`, `dashboard_stats_mv`, `vex_statements`, `cve_refresh_log`, `github_license_cache`, `github_repo_metadata` (11 migrations in `db/migrations/`).

## Angular (Frontend)
- Use strict TypeScript mode and standalone components.
- Unit tests use **Vitest** (not Karma/Jasmine). Run via `npx ng test`.
- Virtual scrolling uses `@angular/cdk` (`ScrollingModule`). Always implement for large lists of dependency nodes or vulnerabilities to prevent browser freezing.
- Utilize OnPush change detection for data-heavy dashboard components to optimize rendering performance.
- All routes are lazy-loaded standalone components (see `app.routes.ts`). Feature pages: `dashboard`, `sbom-explorer`, `vulnerability`, `search` (CVE impact, license compliance, dependency stats), `license-compliance`, `vex`, `archived-packages`.
- Shared chart components live in `shared/charts/` (donut chart, horizontal bar chart).
- UI supports **Dark Mode** (toggle in navbar, persisted to localStorage) and **Custom CSS Theming** (external `custom-theme.css` mountable without rebuild).
- License exemptions are visually distinct: green badge + orange text for exempted copyleft, red for violations.
- Project grouping with clickable version tags is used in CVE Impact, VEX, and License Overview pages.

# Boundaries
- **Always do:** Write unit tests for new Go packages and Angular components. Ensure ClickHouse bulk inserts are batched. Update `docs/ARCHITECTURE_PLAN.md` when adding new services or features.
- **Ask first:** Before adding new third-party dependencies (npm or Go modules), modifying the ClickHouse schema, or changing Kubernetes manifest structures.
- **Never do:** Never commit secrets or API keys. Never use a relational database (like PostgreSQL) for the core SBOM dependency trees. Never split the codebase into multiple repositories. Never add write APIs for license exceptions (frontend is public).
