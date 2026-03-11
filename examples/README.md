# SeeBOM – Deployment Examples

This directory contains ready-to-use example configurations for deploying SeeBOM.

| Directory | Description |
|-----------|-------------|
| [`kind/`](kind/) | Local development with [Kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker) |
| [`kubernetes/`](kubernetes/) | Production / staging deployment on a real Kubernetes cluster |

## SBOM Ingestion Methods

| Method | Config | Best For |
|--------|--------|----------|
| **Seed job** | `gitSync.enabled: false` + `seedJob` | Large repos (cncf/sbom ~14 GB), one-time clone into PVC |
| **git-sync** | `gitSync.enabled: true` | Small repos (< 1 GB), continuous auto-pull |
| **Manual PVC** | `gitSync.enabled: false`, no seedJob | Custom CI, S3/GCS sources, pre-built SBOMs |

See [`kubernetes/README.md`](kubernetes/README.md) for full details on each method.

## Quick Start (Kind)

```bash
# 1. Copy the example and fill in your secrets
cp examples/kind/secrets.env.example local/secrets.env
vi local/secrets.env

# 2. Deploy
make kind-up

# 3. Open
#    UI:  http://localhost:8090
#    API: http://localhost:8080/healthz
```

## Production Deployment

```bash
# 1. Copy and customise values
cp examples/kubernetes/values-production.yaml my-values.yaml
vi my-values.yaml

# 2. Install via Helm
helm install seebom oci://ghcr.io/seebom-labs/seebom/charts/seebom \
  --version 0.1.2 \
  -f my-values.yaml
```

See [`docs/DEPLOYMENT_GUIDE.md`](../docs/DEPLOYMENT_GUIDE.md) for the full guide.

