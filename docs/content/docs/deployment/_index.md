---
title: "Deployment"
linkTitle: "Deployment"
type: docs
weight: 3
description: >
  Kubernetes deployment guide — S3 ingestion, Helm configuration, license governance, theming, and operations.
---

{{% pageinfo %}}
This guide covers deploying SeeBOM to a Kubernetes cluster using Helm.
{{% /pageinfo %}}

## Prerequisites

- Kubernetes cluster (1.27+)
- [ClickHouse Operator](https://github.com/Altinity/clickhouse-operator) installed
- Helm 3.x
- Container images pushed to a registry (e.g. `ghcr.io/seebom-labs/seebom/*`)

---

## 1. SBOMs – Getting Data Into the Cluster

SeeBOM supports multiple SBOM ingestion methods. **S3 bucket ingestion** is the default and recommended approach — it requires no PVCs, no volume scheduling, and scales to any number of SBOMs.

### Option A: S3 Buckets (default, recommended)

Ingest SBOMs directly from S3-compatible buckets (AWS S3, MinIO, GCS). The Ingestion Watcher streams object listings with pagination and the Parsing Workers fetch objects on-demand.

**Single public bucket:**

```yaml
s3:
  buckets: '[{"name":"cncf-subproject-sboms","region":"us-east-1"}]'
```

**Multiple buckets:**

```yaml
s3:
  buckets: '[{"name":"cncf-subproject-sboms","region":"us-east-1"},{"name":"cncf-project-sboms","region":"us-east-1"}]'
```

**Private buckets with credentials:**

```yaml
s3:
  buckets: '[{"name":"my-private-bucket","region":"eu-west-1"}]'
  accessKey: ""   # pass via --set or K8s Secret
  secretKey: ""   # pass via --set or K8s Secret
```

```bash
helm install seebom deploy/helm/seebom/ -n seebom -f my-values.yaml \
  --set s3.accessKey="AKIA..." \
  --set s3.secretKey="..."
```

**MinIO (local S3-compatible):**

```yaml
s3:
  buckets: '[{"name":"sboms","endpoint":"minio.minio.svc:9000","usePathStyle":true,"useSSL":false}]'
  accessKey: "minioadmin"
  secretKey: "minioadmin"
```

**Advantages:**
- No PVC, no volume scheduling, no pod affinity constraints
- Streams object listings — handles 100k+ SBOMs without memory issues
- Works with any S3-compatible storage (AWS, GCS, MinIO, Ceph, DigitalOcean Spaces)

### Option B: Seed Job

```yaml
s3:
  buckets: ""

gitSync:
  enabled: false

seedJob:
  sbomRepo: "https://github.com/cncf/sbom.git"
  sbomBranch: main
```

### Option C: git-sync (small repos < 1 GB)

```yaml
s3:
  buckets: ""

gitSync:
  enabled: true
  repo: "https://github.com/your-org/sbom-repo.git"
  branch: main
```

> **⚠️** git-sync struggles with large repos (multi-GB). Use S3 or the seed job instead.

### Option D: Pre-populated PVC

```yaml
s3:
  buckets: ""
gitSync:
  enabled: false

sbomSource:
  pvcName: my-preloaded-sbom-pvc
```

---

## 2. License Exceptions

License exceptions suppress specific license violations. They are stored in a **ConfigMap** that is mounted read-only into the API Gateway and Workers.

```bash
kubectl edit configmap seebom-license-exceptions
kubectl rollout restart deployment seebom-api-gateway
```

---

## 3. License Policy

The license policy defines which SPDX IDs are classified as **permissive**, **copyleft**, or **unknown**.

```bash
kubectl edit configmap seebom-license-policy
kubectl rollout restart deployment seebom-api-gateway seebom-parsing-worker
```

---

## 4. Custom Theme

```yaml
ui:
  customTheme:
    enabled: true
```

```bash
kubectl create configmap seebom-custom-theme \
  --from-file=custom-theme.css=./my-theme.css \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl rollout restart deployment seebom-ui
```

---

## 5. Site Configuration

```yaml
ui:
  siteConfig:
    enabled: true
    content:
      brandName: "My Platform"
      pageTitle: "My Platform"
      dashboard:
        title: "Overview"
        subtitle: "Software Supply Chain Governance"
```

---

## 6. Full Deployment Example

### S3-based (recommended)

```bash
helm install seebom ./deploy/helm/seebom \
  -f values-production.yaml \
  --set image.tag=0.1.3 \
  --set 's3.buckets=[{"name":"cncf-subproject-sboms","region":"us-east-1"}]' \
  --set s3.accessKey="AKIA..." \
  --set s3.secretKey="..." \
  --set parsingWorker.replicas=10
```

---

## 7. Verifying the Deployment

```bash
kubectl get pods -l app.kubernetes.io/name=seebom

kubectl exec -it $(kubectl get pod -l app.kubernetes.io/component=api-gateway -o name | head -1) \
  -- wget -qO- http://localhost:8080/api/v1/stats/dashboard
```

---

## Summary

| What | Where | How to Change |
|---|---|---|
| **SBOMs (S3)** | S3-compatible buckets | Configure `s3.buckets` in Helm values |
| **SBOMs (volume)** | PVC via seed job or git-sync | Push to Git, seed job clones |
| **VEX files** | Same S3 bucket or directory | Place `*.openvex.json` alongside SBOMs |
| **License Exceptions** | ConfigMap | `kubectl edit configmap` → restart API |
| **License Policy** | ConfigMap | `kubectl edit configmap` → restart API + Workers |
| **Custom Theme** | ConfigMap | `kubectl create configmap` → restart UI |
| **Site Config** | ConfigMap | Helm values `ui.siteConfig.content.*` → restart UI |
| **S3 credentials** | Secret | `--set s3.accessKey=...` or `kubectl edit secret` |

