# SeeBOM – Release & Publishing Guide

> **Updated:** 2026-03-09

## Container Images

All container images are published to **GitHub Container Registry (ghcr.io)**:

| Image | Description |
|-------|-------------|
| `ghcr.io/seebom-labs/seebom/ingestion-watcher` | CronJob: scans SBOM/VEX files, enqueues jobs |
| `ghcr.io/seebom-labs/seebom/parsing-worker` | Stateless worker: parses SBOMs, queries OSV, checks licenses |
| `ghcr.io/seebom-labs/seebom/api-gateway` | REST API (16 endpoints) |
| `ghcr.io/seebom-labs/seebom/ui` | Angular frontend (Nginx) |

Images are built for **linux/amd64** and **linux/arm64**.

---

## How to Release

### 1. Tag the release

```bash
git tag v0.1.0
git push origin v0.1.0
```

### 2. What happens automatically

The GitHub Actions workflow (`.github/workflows/release.yml`) triggers on any `v*` tag and:

1. **Builds all 4 container images** (multi-arch: amd64 + arm64)
2. **Pushes them to ghcr.io** with two tags each:
   - `ghcr.io/seebom-labs/seebom/<component>:0.1.0` (version)
   - `ghcr.io/seebom-labs/seebom/<component>:latest`
3. **Packages the Helm chart** with the matching version
4. **Pushes the Helm chart** as an OCI artifact to `oci://ghcr.io/seebom-labs/seebom/charts`

### 3. Verify the release

```bash
# Check images exist
docker pull ghcr.io/seebom-labs/seebom/api-gateway:0.1.0

# Check Helm chart
helm show chart oci://ghcr.io/seebom-labs/seebom/charts/seebom --version 0.1.0
```

---

## Installing from a Release

### Helm (recommended)

```bash
helm install seebom oci://ghcr.io/seebom-labs/seebom/charts/seebom \
  --version 0.1.0 \
  -f values-production.yaml
```

### Override image tag

```bash
helm install seebom oci://ghcr.io/seebom-labs/seebom/charts/seebom \
  --version 0.1.0 \
  --set image.tag=0.1.0
```

---

## CI Workflows

| Workflow | File | Trigger | What it does |
|----------|------|---------|-------------|
| **CI** | `.github/workflows/ci.yml` | Push/PR to `main` | Go build + test + vet, Angular build, Helm lint |
| **Release** | `.github/workflows/release.yml` | Git tag `v*` | Build + push 4 images (multi-arch), package + push Helm chart |

---

## Building Images Locally

For testing before a release:

```bash
# Build all 4 images with tag "dev"
make images

# Build with a specific tag
make images TAG=0.2.0-rc1

# Build and push to GHCR (requires: docker login ghcr.io)
make images-push TAG=0.2.0-rc1
```

### Manual docker build (single image)

```bash
# Backend images (multi-target Dockerfile)
docker build -t my-registry/seebom/api-gateway:test \
  --target api-gateway backend/

docker build -t my-registry/seebom/parsing-worker:test \
  --target parsing-worker backend/

docker build -t my-registry/seebom/ingestion-watcher:test \
  --target ingestion-watcher backend/

# UI image
docker build -t my-registry/seebom/ui:test ui/
```

---

## Image Architecture

The backend uses a **single multi-stage Dockerfile** (`backend/Dockerfile`) with three named targets:

```
golang:1.24-alpine (builder)
  ├── go build → /bin/ingestion-watcher
  ├── go build → /bin/parsing-worker
  └── go build → /bin/api-gateway

alpine:3.21 (ingestion-watcher)  ← FROM builder, COPY binary
alpine:3.21 (parsing-worker)     ← FROM builder, COPY binary
alpine:3.21 (api-gateway)        ← FROM builder, COPY binary
```

The UI uses a separate Dockerfile (`ui/Dockerfile`):

```
node:22-alpine (builder)
  └── ng build → /app/dist/

nginx:1.27-alpine
  └── COPY dist/ → /usr/share/nginx/html/
```

All runtime images run as `nobody:nobody` (backend) or `nginx` (UI) for security.

---

## Versioning

- **Git tags**: `v0.1.0`, `v0.2.0`, etc. (SemVer)
- **Image tags**: `0.1.0` (without `v` prefix) + `latest`
- **Helm chart version**: Matches the Git tag (auto-updated by CI)
- `values.yaml` defaults to the latest released tag

---

## Private Registry

To use a different registry, override in Helm:

```bash
helm install seebom oci://ghcr.io/seebom-labs/seebom/charts/seebom \
  --set image.registry=my-registry.example.com \
  --set image.repository=my-org/seebom \
  --set image.tag=0.1.0
```

Or build + push locally:

```bash
make images-push REGISTRY=my-registry.example.com REPO=my-org/seebom TAG=0.1.0
```

