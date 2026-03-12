# SeeBOM – Kubernetes Deployment Examples

Example Helm values for deploying SeeBOM to a real Kubernetes cluster
(EKS, GKE, AKS, self-hosted, etc.).

## Prerequisites

- Kubernetes ≥ 1.27
- Helm ≥ 3.12
- [Altinity ClickHouse Operator](https://github.com/Altinity/clickhouse-operator) installed
- A StorageClass that supports `ReadWriteOnce` PVCs

## Quick Start

```bash
# 1. Install ClickHouse Operator
kubectl apply -f https://raw.githubusercontent.com/Altinity/clickhouse-operator/master/deploy/operator/clickhouse-operator-install-bundle.yaml

# 2. Create namespace + secret
kubectl create namespace seebom
kubectl create secret generic seebom-secret -n seebom \
  --from-literal=CLICKHOUSE_PASSWORD='<your-password>' \
  --from-literal=GITHUB_TOKEN='<your-github-pat>'

# 3. Customise values
cp examples/kubernetes/values-production.yaml my-values.yaml
vi my-values.yaml

# 4. Install
helm install seebom deploy/helm/seebom/ \
  -n seebom -f my-values.yaml

# Or from the OCI registry:
helm install seebom oci://ghcr.io/seebom-labs/seebom/charts/seebom \
  --version 0.1.3 -n seebom -f my-values.yaml
```

## Values Files

| File | SBOM Source | Best For |
|------|-------------|----------|
| `values-production.yaml` | **Seed job** (shallow clone into PVC) | Large repos (e.g. cncf/sbom ~14 GB), HA replicas |
| `values-minimal.yaml` | **git-sync** (sidecar, continuous pull) | Small repos (< 1 GB), CI/staging |

---

## How SBOMs Are Loaded

SeeBOM reads `.spdx.json` files from a PVC mounted at `/data/sboms`. There are three ways to populate this PVC:

### Method 1: Seed Job (recommended for large repos)

Used by `values-production.yaml` and `values-kind.yaml`.

A Kubernetes Job runs once at install time and:
1. Does a **shallow `git clone`** of your SBOM repo into a temp directory
2. **Flat-copies** all `.spdx.json` and `.openvex.json` files into the PVC (flattening nested directory paths into filenames to avoid collisions)
3. Optionally downloads the [CNCF license exceptions](https://github.com/cncf/foundation/blob/main/license-exceptions/exceptions.json) into the PVC

```yaml
gitSync:
  enabled: false           # disable git-sync

seedJob:
  sbomRepo: "https://github.com/cncf/sbom.git"
  sbomBranch: main
  cncfExceptionsURL: "https://raw.githubusercontent.com/cncf/foundation/main/license-exceptions/exceptions.json"

sbomSource:
  storageSize: 20Gi        # must be large enough for the full repo
```

**Advantages:** Works with any repo size. Single clone, no sidecar overhead. All pods are automatically co-scheduled on the same node via pod affinity (required for RWO volumes on multi-node clusters).

**Refreshing:** Delete the seed job and re-run Helm upgrade:
```bash
kubectl delete job -n seebom -l app.kubernetes.io/component=seed-sboms
helm upgrade seebom deploy/helm/seebom/ -n seebom -f my-values.yaml
```

### Method 2: git-sync (recommended for small repos)

Used by `values-minimal.yaml`.

A [git-sync](https://github.com/kubernetes/git-sync) sidecar runs alongside the Ingestion Watcher and continuously pulls from your Git repo.

```yaml
gitSync:
  enabled: true
  repo: "https://github.com/your-org/sbom-repo.git"
  branch: main
  depth: 1
  period: "6h"             # re-sync every 6 hours
  timeout: 120             # seconds for git operations
  # For private repos:
  # secretName: git-sync-ssh-key

sbomSource:
  storageSize: 5Gi
```

**Advantages:** Automatic updates, no manual refresh needed.

**⚠️  Limitation:** git-sync struggles with very large repos (multi-GB). For repos like `cncf/sbom` (~14 GB, 6500+ files), the sidecar times out or OOM-kills. Use the seed job method instead.

### Method 3: Pre-populated PVC (manual / CI pipeline)

If your SBOMs are not in a Git repo, or you have a custom CI pipeline, you can populate the PVC yourself:

```yaml
gitSync:
  enabled: false

# No seedJob configured — you manage the PVC content yourself.

sbomSource:
  pvcName: seebom-sbom-data
  mountPath: /data/sboms
  storageSize: 10Gi
```

Then copy files into the PVC:

```bash
# Option A: kubectl cp from a running pod
kubectl cp ./my-sboms/ seebom/<any-running-pod>:/data/sboms/ -c <container>

# Option B: Use a one-shot Job
kubectl create job copy-sboms --image=busybox -- \
  sh -c "cp /source/*.spdx.json /data/sboms/"
# (mount both source and target PVCs)

# Option C: Use an initContainer in your CI pipeline
# that pulls SBOMs from S3, GCS, or an artifact registry
```

After populating the PVC, trigger the Ingestion Watcher:
```bash
kubectl create job --from=cronjob/seebom-ingestion-watcher manual-ingest -n seebom
```

---

## File Placement Rules

The Ingestion Watcher scans the SBOM directory **recursively** for:

| Pattern | Type | Example |
|---------|------|---------|
| `*.spdx.json` | SPDX SBOM | `my-project.spdx.json` |
| `*.openvex.json` | OpenVEX statement | `my-project.openvex.json` |
| `*.vex.json` | OpenVEX statement | `my-project.vex.json` |
| `license-exceptions.json` | CNCF exceptions file | (auto-downloaded by seed job) |
| `license-policy.json` | Custom license policy | (optional, overrides ConfigMap) |

Files are **deduplicated by SHA256 hash** — uploading the same file twice will not create duplicate entries.

Nested directories are fine:
```
/data/sboms/
├── org-a/
│   ├── project-1/v1.0/project-1.spdx.json
│   └── project-2/v2.3/project-2.spdx.json
├── org-b/
│   └── service-x.spdx.json
├── license-exceptions.json
└── my-project.openvex.json
```

---

## Exposing the UI

Both values files use `ClusterIP` services. To expose them:

### Option A: Ingress (recommended)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: seebom
  namespace: seebom
spec:
  ingressClassName: nginx
  rules:
    - host: seebom.example.com
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: seebom-api-gateway
                port: { number: 80 }
          - path: /
            pathType: Prefix
            backend:
              service:
                name: seebom-ui
                port: { number: 80 }
```

### Option B: kubectl port-forward

```bash
kubectl port-forward -n seebom svc/seebom-ui 8090:80 &
kubectl port-forward -n seebom svc/seebom-api-gateway 8080:80 &
```

