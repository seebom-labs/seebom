.PHONY: help dev dev-up dev-down dev-logs dev-reset
.PHONY: ch-shell ch-migrate
.PHONY: backend-build backend-test backend-vet
.PHONY: ui-build ui-dev
.PHONY: ingest worker api
.PHONY: images images-push
.PHONY: sync-labels

SHELL := /bin/bash
REGISTRY ?= ghcr.io
REPO     ?= seebom-labs/seebom
TAG      ?= dev

# ─── Help ────────────────────────────────────────────────────────────────────
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Docker Compose (full stack) ────────────────────────────────────────────
dev: dev-up ## Start entire stack (ClickHouse + all services + UI)
	@echo "🚀 SeeBOM running at:"
	@echo "   UI:          http://localhost:8090"
	@echo "   API:         http://localhost:8080/healthz"
	@echo "   ClickHouse:  http://localhost:8123 (HTTP) / localhost:9000 (TCP)"

dev-up: ## docker compose up --build -d
	docker compose up --build -d

dev-down: ## docker compose down
	docker compose down

dev-restart: ## Restart with new .env values (keeps data)
	docker compose up -d --force-recreate

migrate: ## Run all pending database migrations
	@echo "⏳ Running migrations..."
	@for f in db/migrations/*.sql; do \
		echo "  → $$f"; \
		docker compose exec -T clickhouse clickhouse-client --database=seebom --multiquery < "$$f" 2>/dev/null || true; \
	done
	@echo "✅ Migrations complete."

cve-refresh: migrate ## Run a CVE refresh (check all PURLs for new vulnerabilities)
	@echo "🔍 Starting CVE refresh..."
	@docker compose --profile cve-refresh up --build --force-recreate cve-refresher
	@echo "✅ CVE refresh complete."

re-ingest: ## Re-trigger the Ingestion Watcher (scans for new files)
	docker compose up --force-recreate ingestion-watcher

re-scan: ## Reset all data + queue, then re-ingest (e.g. after enabling OSV)
	@echo "⏳ Running pending migrations..."
	@for f in db/migrations/*.sql; do \
		echo "  → $$f"; \
		docker compose exec -T clickhouse clickhouse-client --database=seebom --multiquery < "$$f" 2>/dev/null || true; \
	done
	@echo "🗑️  Clearing all data tables and queue..."
	@docker compose exec -T clickhouse clickhouse-client --database=seebom \
		--query "TRUNCATE TABLE ingestion_queue"
	@docker compose exec -T clickhouse clickhouse-client --database=seebom \
		--query "TRUNCATE TABLE vulnerabilities"
	@docker compose exec -T clickhouse clickhouse-client --database=seebom \
		--query "TRUNCATE TABLE license_compliance"
	@docker compose exec -T clickhouse clickhouse-client --database=seebom \
		--query "TRUNCATE TABLE sbom_packages"
	@docker compose exec -T clickhouse clickhouse-client --database=seebom \
		--query "TRUNCATE TABLE sboms"
	@docker compose exec -T clickhouse clickhouse-client --database=seebom \
		--query "TRUNCATE TABLE vex_statements" 2>/dev/null || true
	@echo "♻️  Rebuilding services with latest code..."
	@docker compose up --build -d api-gateway parsing-worker
	@echo "♻️  Re-triggering ingestion..."
	@docker compose up --force-recreate ingestion-watcher
	@echo "✅ Full re-scan started. Workers will process all SBOMs from scratch."
	@echo "   Monitor: make dev-status"

dev-logs: ## docker compose logs -f
	docker compose logs -f

dev-reset: dev-down ## Destroy volumes and restart fresh
	docker compose down -v
	docker compose up --build -d

dev-status: ## Show status of all containers + ingestion progress
	@docker compose ps
	@echo ""
	@echo "=== Ingestion Progress ==="
	@docker compose exec -T clickhouse clickhouse-client --database=seebom \
		--query "SELECT latest_status AS status, count() AS cnt FROM (SELECT argMax(status, created_at) AS latest_status FROM ingestion_queue GROUP BY job_id) GROUP BY latest_status ORDER BY latest_status" \
		--format=PrettyCompact 2>/dev/null || echo "(ClickHouse not ready)"
	@echo ""
	@echo "=== Data Summary ==="
	@docker compose exec -T clickhouse clickhouse-client --database=seebom \
		--query "SELECT 'sboms' AS tbl, count() AS cnt FROM sboms FINAL UNION ALL SELECT 'packages', count() FROM sbom_packages FINAL UNION ALL SELECT 'vulns', count() FROM vulnerabilities FINAL UNION ALL SELECT 'licenses', count() FROM license_compliance FINAL" \
		--format=PrettyCompact 2>/dev/null || echo "(ClickHouse not ready)"

# ─── ClickHouse ──────────────────────────────────────────────────────────────
ch-shell: ## Open a ClickHouse client shell
	docker compose exec clickhouse clickhouse-client --database=seebom

ch-migrate: ## Manually run all migrations against running ClickHouse
	@for f in db/migrations/*.sql; do \
		echo "⏳ Running $$f ..."; \
		docker compose exec -T clickhouse clickhouse-client --database=seebom --multiquery < "$$f"; \
	done
	@echo "✅ All migrations applied."

# ─── Local dev (without Docker for backend) ──────────────────────────────────
# Start only ClickHouse, then run Go services locally.

ch-only: ## Start only ClickHouse in Docker
	docker compose up -d clickhouse
	@echo "⏳ Waiting for ClickHouse to be healthy..."
	@until docker compose exec clickhouse clickhouse-client --query "SELECT 1" > /dev/null 2>&1; do sleep 1; done
	@echo "✅ ClickHouse ready at localhost:9000"

backend-build: ## Build all Go binaries
	cd backend && go build ./...

backend-test: ## Run all Go tests
	cd backend && go test ./... -v -count=1

backend-vet: ## Run go vet + go fmt
	cd backend && go fmt ./... && go vet ./...

api: ## Run API Gateway locally (needs ClickHouse)
	cd backend && go run ./cmd/api-gateway/

ingest: ## Run Ingestion Watcher once locally (needs ClickHouse)
	cd backend && go run ./cmd/ingestion-watcher/

worker: ## Run Parsing Worker locally (needs ClickHouse)
	cd backend && go run ./cmd/parsing-worker/

# ─── Angular UI ──────────────────────────────────────────────────────────────
ui-dev: ## Start Angular dev server (hot-reload, proxies to localhost:8080)
	cd ui && npx ng serve --proxy-config proxy.conf.json

ui-build: ## Build Angular for production
	cd ui && npx ng build --configuration=production

# ─── Container Images ────────────────────────────────────────────────────────
images: ## Build all container images locally (TAG=dev)
	docker build -t $(REGISTRY)/$(REPO)/ingestion-watcher:$(TAG) --target ingestion-watcher backend/
	docker build -t $(REGISTRY)/$(REPO)/parsing-worker:$(TAG)    --target parsing-worker    backend/
	docker build -t $(REGISTRY)/$(REPO)/api-gateway:$(TAG)       --target api-gateway       backend/
	docker build -t $(REGISTRY)/$(REPO)/ui:$(TAG) ui/
	@echo "✅ Built 4 images with tag $(TAG)"

images-push: images ## Build and push all images to GHCR (TAG=dev)
	docker push $(REGISTRY)/$(REPO)/ingestion-watcher:$(TAG)
	docker push $(REGISTRY)/$(REPO)/parsing-worker:$(TAG)
	docker push $(REGISTRY)/$(REPO)/api-gateway:$(TAG)
	docker push $(REGISTRY)/$(REPO)/ui:$(TAG)
	@echo "✅ Pushed 4 images to $(REGISTRY)/$(REPO) with tag $(TAG)"

ui-build: ## Build Angular for production
	cd ui && npx ng build --configuration=production

# ─── GitHub ──────────────────────────────────────────────────────────────────
sync-labels: ## Sync GitHub labels from .github/labels.yml (requires gh + yq)
	.github/scripts/sync-labels.sh
