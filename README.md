# 🔍 SeeBOM Labs

**Kubernetes-native Software Bill of Materials (SBOM) Visualization & Governance Platform**

Ingest 1000+ SPDX SBOMs, scan for vulnerabilities via OSV, enforce license compliance, and apply VEX statements — all visualized in a fast Angular dashboard backed by ClickHouse analytics.

---

## Quick Start

### Prerequisites

| Tool | Minimum Version |
|------|----------------|
| Docker + Docker Compose | v2.20+ |
| Go | 1.24+ (only for local dev) |
| Node.js | 22+ (only for local dev) |

### Option A: Full Stack via Docker Compose (Recommended)

```bash
# 1. Clone the repo
git clone https://github.com/seebom-labs/seebom.git && cd seebom

# 2. Place your SPDX files in the sboms/ directory
#    (an example file is included: sboms/_example.spdx.json)

# 3. Start everything
make dev

# Or without make:
docker compose up --build -d
```

This starts:
- **ClickHouse** on `localhost:9000` (TCP) / `localhost:8123` (HTTP)
- **API Gateway** on `localhost:8080`
- **Ingestion Watcher** (runs once, scans `sboms/` for new files)
- **Parsing Worker** (processes queued SBOM/VEX files)
- **Angular UI** on `localhost:8090`

Open **http://localhost:8090** in your browser.

### Configuration (`.env`)

Copy `.env.example` to `.env` and adjust:

```bash
cp .env.example .env
```

| Variable | Default | Description |
|----------|---------|-------------|
| `SBOM_SOURCE_DIR` | `./sboms` | Path to your SBOM files (can point to an external repo checkout) |
| `SBOM_LIMIT` | `0` | Max SBOMs to enqueue per watcher run. `0` = unlimited. Use `50`–`200` for local dev. |
| `WORKER_REPLICAS` | `1` | Number of parallel parsing worker containers |
| `WORKER_BATCH_SIZE` | `50` | Jobs claimed per polling cycle per worker |
| `SKIP_OSV` | `false` | Skip OSV vulnerability API calls. Set `true` for fast initial bulk load (licenses only), then re-run with `false`. |
| `SKIP_GITHUB_RESOLVE` | `false` | Skip GitHub license resolution for packages with `NOASSERTION`/empty licenses. |
| `GITHUB_TOKEN` | *(empty)* | GitHub personal access token for license resolution. Increases rate limit from 60 to 5000 req/h. No scopes needed. |

**After changing `.env`:**

```bash
# Apply new values (keeps ClickHouse data):
docker compose up -d --force-recreate

# Or full reset (wipes ClickHouse data):
make dev-reset
```

### Configuration Files

Two JSON config files control license governance. Edit them and restart the affected services.

| File | Mounted in | Purpose |
|------|-----------|---------|
| `sboms/license-policy.json` | API Gateway, Workers | Defines which SPDX IDs are **permissive** vs. **copyleft**. Anything not listed = `unknown`. |
| `sboms/license-exceptions.json` | API Gateway, Workers | Exempts specific licenses (blanket) or package+license combos from violation reporting. [CNCF format](https://github.com/cncf/foundation/blob/main/license-exceptions/exceptions.json). |

### Custom Theme (CSS)

The entire UI color scheme is defined via CSS Custom Properties and can be overridden **without rebuilding** Angular.

**Local (Docker Compose):** Create a CSS file and set `CUSTOM_THEME` in `.env`:

```bash
# .env
CUSTOM_THEME=./my-theme.css
```

```css
/* my-theme.css */
:root {
  --accent: #e94560;
  --nav-bg: #1a1a2e;
  --nav-brand: #e94560;
  --severity-critical: #ff4444;
  --license-permissive: #22c55e;
}
```

```bash
docker compose up -d --force-recreate ui
```

**Kubernetes:** Enable the theme ConfigMap in Helm values:

```yaml
ui:
  customTheme:
    enabled: true
```

Then edit the ConfigMap:

```bash
kubectl create configmap seebom-custom-theme \
  --from-file=custom-theme.css=./my-theme.css \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl rollout restart deployment seebom-ui
```

See `ui/src/assets/custom-theme.example.css` for all available variables.

```bash
# After editing config files:
docker compose up -d --force-recreate api-gateway parsing-worker
```

See [docs/DEPLOYMENT_GUIDE.md](docs/DEPLOYMENT_GUIDE.md) for Kubernetes deployment instructions.

### Option B: Local Development (Hot Reload)

Use this when you want to iterate on code quickly:

```bash
# 1. Start only ClickHouse
make ch-only

# 2. Run the migrations (first time only)
make ch-migrate

# 3. In separate terminals:

# Terminal 1: API Gateway
make api

# Terminal 2: Run Ingestion Watcher (once)
make ingest

# Terminal 3: Start Parsing Worker
make worker

# Terminal 4: Angular dev server (hot reload, proxied to API)
make ui-dev
```

Open **http://localhost:4200** — Angular proxies `/api/*` to `localhost:8080`.

---

## Architecture

```
sboms/*.spdx.json + *.openvex.json
       │
       ▼
┌─────────────────────────┐
│   Ingestion Watcher     │  CronJob: scans files, deduplicates by SHA256,
│   (Go binary)           │  enqueues jobs into ClickHouse queue
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│   Parsing Workers (N)   │  Stateless: claims jobs, parses SPDX/VEX,
│   (Go binary)           │  queries OSV for vulns, resolves unknown licenses
│                         │  via GitHub API, checks license compliance,
│                         │  batch-INSERTs into ClickHouse
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│   CVE Refresher         │  CronJob (daily): checks all PURLs for new CVEs
│   (Go binary)           │  without re-scanning all SBOMs
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│   ClickHouse            │  11 tables: sboms, sbom_packages, vulnerabilities,
│                         │  license_compliance, vex_statements, ingestion_queue,
│                         │  dashboard_stats_mv, cve_refresh_log, github_license_cache,
│                         │  github_repo_metadata
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│   API Gateway           │  17 REST endpoints, stateless
│   (Go binary)           │
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│   Angular UI            │  10 lazy-loaded pages, virtual scrolling,
│                         │  OnPush change detection, dark mode,
│                         │  CSS custom properties theming
└─────────────────────────┘
```

See [docs/ARCHITECTURE_PLAN.md](docs/ARCHITECTURE_PLAN.md) for the full blueprint.  
See [docs/DEPLOYMENT_GUIDE.md](docs/DEPLOYMENT_GUIDE.md) for Kubernetes deployment.  
See [docs/RELEASE.md](docs/RELEASE.md) for building and publishing container images.  
See [docs/TESTING.md](docs/TESTING.md) for writing and running tests.

---

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make dev` | Start full stack via Docker Compose |
| `make dev-down` | Stop all containers |
| `make dev-restart` | Restart with new `.env` values (keeps data) |
| `make dev-logs` | Follow all container logs |
| `make dev-reset` | Destroy data volumes and restart fresh |
| `make re-ingest` | Re-trigger the Ingestion Watcher (scans for new files) |
| `make re-scan` | Wipe all data and re-process everything (e.g. after enabling OSV) |
| `make cve-refresh` | Check all known PURLs for new CVEs (without re-scanning SBOMs) |
| `make migrate` | Run all pending database migrations |
| `make ch-only` | Start only ClickHouse (for local dev) |
| `make ch-migrate` | Run SQL migrations against ClickHouse |
| `make ch-shell` | Open ClickHouse CLI |
| `make api` | Run API Gateway locally |
| `make ingest` | Run Ingestion Watcher locally |
| `make worker` | Run Parsing Worker locally |
| `make ui-dev` | Start Angular dev server with API proxy |
| `make backend-build` | Build all Go binaries |
| `make backend-test` | Run all Go tests |
| `make backend-vet` | Run go vet + go fmt |
| `make ui-build` | Build Angular for production |

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/healthz` | Health check |
| GET | `/api/v1/stats/dashboard` | Dashboard stats (VEX effective/suppressed counts) |
| GET | `/api/v1/stats/dependencies?limit=N` | Top N dependencies across all projects |
| GET | `/api/v1/sboms?page=&page_size=` | Paginated SBOM list |
| GET | `/api/v1/sboms/{id}/detail` | SBOM detail with severity breakdown |
| GET | `/api/v1/sboms/{id}/vulnerabilities` | Vulnerabilities for a specific SBOM |
| GET | `/api/v1/sboms/{id}/licenses` | License breakdown for a specific SBOM |
| GET | `/api/v1/sboms/{id}/dependencies` | Dependency tree |
| GET | `/api/v1/vulnerabilities?page=&vex_filter=` | Paginated vulnerabilities (optional: `vex_filter=effective`) |
| GET | `/api/v1/vulnerabilities/{id}/affected-projects` | All projects affected by a CVE |
| GET | `/api/v1/licenses/compliance` | Global license compliance overview |
| GET | `/api/v1/projects/license-violations` | Projects with license violations (filtered by exceptions) |
| GET | `/api/v1/license-exceptions` | Active license exceptions (read-only, from config file) |
| GET | `/api/v1/license-policy` | Active license classification policy (permissive/copyleft lists) |
| GET | `/api/v1/vex/statements?page=&page_size=` | Paginated VEX statements |
| GET | `/api/v1/packages/archived` | Packages using archived GitHub repos (no longer maintained) |

---

## Adding Your SBOMs

1. Place `.spdx.json` files in the `sboms/` directory (or set `SBOM_SOURCE_DIR` in `.env`)
2. Place `.openvex.json` or `.vex.json` files in the same directory
3. Re-trigger ingestion (see below)
4. The Parsing Worker will automatically process new files

The Ingestion Watcher deduplicates by SHA256 hash — it will skip files that have already been processed.

### Re-triggering Ingestion

```bash
# Run the watcher again (scans for new files, exits when done):
docker compose up ingestion-watcher

# If you changed SBOM_LIMIT or SBOM_SOURCE_DIR, force-recreate:
docker compose up --force-recreate ingestion-watcher

# To re-ingest everything from scratch (wipes all data):
make dev-reset
```

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.24, net/http (stdlib) |
| Database | ClickHouse (MergeTree family) |
| Frontend | Angular 19, CDK Virtual Scrolling |
| Vuln Scanning | OSV.dev API |
| VEX | OpenVEX Spec v0.2.0 |
| Deployment | Helm 3, Docker Compose |

---

## License

[LICENSE](LICENSE)

