# SeeBOM – Kubernetes Deployment Guide

> **Updated:** 2026-03-12

## Prerequisites

- Kubernetes cluster (1.27+)
- [ClickHouse Operator](https://github.com/Altinity/clickhouse-operator) installed
- Helm 3.x
- Container images pushed to a registry (e.g. `ghcr.io/your-org/seebom/*`)

---

## 1. SBOMs – Getting Data Into the Cluster

SBOMs are loaded from a **PersistentVolumeClaim** (PVC) mounted at `/data/sboms`. There are three ways to populate this PVC. See also [`examples/kubernetes/README.md`](../examples/kubernetes/README.md) for complete Helm value examples.

### Option A: Seed Job (recommended for large repos)

A Kubernetes Job runs at install time, does a shallow `git clone`, and flat-copies all SBOM files into the PVC. This is the most reliable method for large repos (e.g. `cncf/sbom` at ~14 GB with 6500+ SBOMs).

```yaml
gitSync:
  enabled: false

seedJob:
  sbomRepo: "https://github.com/cncf/sbom.git"
  sbomBranch: main
  # Also download the CNCF license exceptions into the PVC:
  cncfExceptionsURL: "https://raw.githubusercontent.com/cncf/foundation/main/license-exceptions/exceptions.json"

sbomSource:
  storageSize: 20Gi
```

The seed job flattens nested directory structures (`org/project/version/file.spdx.json` → `org_project_version_file.spdx.json`) to avoid filename collisions.

**To refresh SBOMs**, delete the seed job and re-run Helm upgrade:
```bash
kubectl delete job -n seebom -l app.kubernetes.io/component=seed-sboms
helm upgrade seebom deploy/helm/seebom/ -n seebom -f my-values.yaml
```

### Option B: git-sync (for small repos < 1 GB)

A [git-sync](https://github.com/kubernetes/git-sync) sidecar continuously pulls SBOMs from a Git repo into the PVC. Good for small repos that update frequently.

```yaml
gitSync:
  enabled: true
  repo: "https://github.com/your-org/sbom-repo.git"
  branch: main
  depth: 1
  period: "6h"
  timeout: 120
```

For **private repos**, create a Secret with your deploy key:

```bash
kubectl create secret generic git-sync-credentials \
  --from-file=ssh-key=/path/to/id_rsa \
  --from-literal=GIT_SYNC_SSH_KEY_FILE=/etc/git-secret/ssh-key
```

```yaml
gitSync:
  enabled: true
  repo: "git@github.com:your-org/sbom-repo.git"
  secretName: git-sync-credentials
```

> **⚠️  Limitation:** git-sync runs as a sidecar and struggles with very large repos (multi-GB). For repos like `cncf/sbom` (~14 GB), the sidecar times out or OOM-kills. Use the **seed job** (Option A) instead.

### Option C: Pre-populated PVC (manual / CI pipeline)

If your SBOMs are not in a Git repo, or come from a CI pipeline, S3, or an artifact registry:

```yaml
gitSync:
  enabled: false
# No seedJob — you manage the PVC content yourself.

sbomSource:
  pvcName: my-preloaded-sbom-pvc
```

Populate the PVC however you prefer (kubectl cp, CI job, initContainer, etc.), then trigger the Ingestion Watcher:
```bash
kubectl create job --from=cronjob/seebom-ingestion-watcher manual-ingest -n seebom
```

### Supported File Types

The Ingestion Watcher scans the SBOM directory **recursively** for:

| Pattern | Type |
|---------|------|
| `*.spdx.json` | SPDX SBOM |
| `*.openvex.json` | OpenVEX statement |
| `*.vex.json` | OpenVEX statement |

Files are **deduplicated by SHA256 hash** — uploading the same file twice will not create duplicates.

> **Note:** VEX files should be placed alongside SBOM files in the same directory. They are never truncated by `SBOM_LIMIT`.

### Volume Scheduling (multi-node clusters)

When using a PVC (`gitSync.enabled: false`), the SBOM volume is `ReadWriteOnce` (RWO) — it can only be mounted on a single node. The Helm chart automatically adds **pod affinity** (`seebom.io/sbom-volume` label) to all workloads that mount this PVC (Parsing Workers, API Gateway, Ingestion Watcher, Seed Job), ensuring they are scheduled on the same node.

This is transparent and requires no manual configuration. If you need to spread pods across nodes, switch to `ReadWriteMany` (RWX) storage and remove the affinity by setting `gitSync.enabled: true` with an `emptyDir` approach.

> **Note:** The directory scanner automatically skips `lost+found` (common on ext4-formatted block volumes), `.git`, and other hidden directories. No manual cleanup of the PVC is needed.

---

## 2. License Exceptions – Whitelisting Known Violations

License exceptions suppress specific license violations. They are stored in a **ConfigMap** that is mounted read-only into the API Gateway and Workers.

### Edit the default ConfigMap

After `helm install`, edit the ConfigMap directly:

```bash
kubectl edit configmap seebom-license-exceptions
```

The JSON format follows the [CNCF exceptions format](https://github.com/cncf/foundation/blob/main/license-exceptions/exceptions.json):

```json
{
  "version": "1.0.0",
  "lastUpdated": "2026-03-07",
  "blanketExceptions": [
    {
      "id": "blanket-mpl-2.0",
      "license": "MPL-2.0",
      "status": "approved",
      "approvedDate": "2026-03-07",
      "comment": "MPL-2.0 is file-level copyleft, acceptable for unmodified deps."
    }
  ],
  "exceptions": [
    {
      "id": "exc-001",
      "package": "github.com/hashicorp/golang-lru",
      "license": "MPL-2.0",
      "status": "approved",
      "approvedDate": "2026-03-07",
      "comment": "Widely used LRU cache, unmodified."
    }
  ]
}
```

### Apply changes

After editing, restart the API Gateway to pick up changes:

```bash
kubectl rollout restart deployment seebom-api-gateway
```

> **Note:** Violations are filtered at query time, so no re-ingestion is needed.

---

## 3. License Policy – Defining Permissive vs. Copyleft

The license policy defines which SPDX IDs are classified as **permissive**, **copyleft**, or **unknown**. Any license not listed falls into `unknown`.

### Edit the default ConfigMap

```bash
kubectl edit configmap seebom-license-policy
```

The format:

```json
{
  "permissive": [
    "MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause",
    "ISC", "Unlicense", "0BSD", "CC0-1.0", "Zlib"
  ],
  "copyleft": [
    "GPL-2.0-only", "GPL-3.0-only", "AGPL-3.0-only",
    "LGPL-2.1-only", "MPL-2.0", "EPL-2.0"
  ]
}
```

### Apply changes

After editing, restart **both** the API Gateway and Workers:

```bash
kubectl rollout restart deployment seebom-api-gateway seebom-parsing-worker
```

> **Note:** Policy changes affect new ingestions. To reclassify existing data,
> trigger a full re-scan (truncate tables + re-ingest).

---

## 4. Custom Theme – Rebranding the UI

The entire UI color scheme is defined via 60+ CSS Custom Properties and can be overridden **without rebuilding Angular**.

### Enable the theme ConfigMap

```yaml
# values-production.yaml
ui:
  customTheme:
    enabled: true
```

### Apply a custom theme

```bash
kubectl create configmap seebom-custom-theme \
  --from-file=custom-theme.css=./my-theme.css \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl rollout restart deployment seebom-ui
```

### Example theme file

```css
/* my-theme.css – override any CSS variable */
:root {
  --accent: #0066cc;
  --nav-bg: #002244;
  --nav-brand: #ff9900;
  --severity-critical: #ff4444;
  --license-permissive: #22c55e;
}
```

See `ui/src/assets/custom-theme.example.css` for all available variables.

> **Note:** The UI also includes a built-in **Dark Mode toggle** (top right of navbar). It persists the user's preference in `localStorage` and respects `prefers-color-scheme`.

---

## 5. Full Deployment Example

```bash
# 1. Install the Helm chart
helm install seebom ./deploy/helm/seebom \
  -f values-production.yaml \
  --set image.registry=ghcr.io \
  --set image.repository=your-org/seebom \
  --set image.tag=0.1.0 \
  --set gitSync.repo=https://github.com/your-org/sbom-repo.git \
  --set parsingWorker.replicas=10 \
  --set parsingWorker.skipOSV=false \
  --set ui.customTheme.enabled=true

# 2. Override license exceptions from a local file
kubectl create configmap seebom-license-exceptions \
  --from-file=license-exceptions.json=./my-exceptions.json \
  --dry-run=client -o yaml | kubectl apply -f -

# 3. Override license policy from a local file
kubectl create configmap seebom-license-policy \
  --from-file=license-policy.json=./my-policy.json \
  --dry-run=client -o yaml | kubectl apply -f -

# 4. Apply a custom theme
kubectl create configmap seebom-custom-theme \
  --from-file=custom-theme.css=./my-theme.css \
  --dry-run=client -o yaml | kubectl apply -f -

# 5. Restart to pick up new configs
kubectl rollout restart deployment seebom-api-gateway seebom-parsing-worker seebom-ui

# 6. Trigger initial ingestion manually
kubectl create job --from=cronjob/seebom-ingestion-watcher seebom-initial-ingest
```

---

## 6. Verifying the Deployment

```bash
# Check all pods are running
kubectl get pods -l app.kubernetes.io/name=seebom

# Check ingestion progress
kubectl exec -it $(kubectl get pod -l app.kubernetes.io/component=api-gateway -o name | head -1) \
  -- wget -qO- http://localhost:8080/api/v1/stats/dashboard

# View loaded policy
kubectl exec -it $(kubectl get pod -l app.kubernetes.io/component=api-gateway -o name | head -1) \
  -- wget -qO- http://localhost:8080/api/v1/license-policy

# View active exceptions
kubectl exec -it $(kubectl get pod -l app.kubernetes.io/component=api-gateway -o name | head -1) \
  -- wget -qO- http://localhost:8080/api/v1/license-exceptions
```

---

## 7. Local Development

Copy `.env.example` to `.env` and adjust:

```bash
cp .env.example .env
```

| Variable | Default | Description |
|----------|---------|-------------|
| `SBOM_SOURCE_DIR` | `./sboms` | Path to your SBOM files (can point to an external repo checkout) |
| `SBOM_LIMIT` | `0` | Max SBOMs to enqueue per watcher run. `0` = unlimited. VEX files are never limited. |
| `WORKER_REPLICAS` | `1` | Number of parallel parsing worker containers |
| `WORKER_BATCH_SIZE` | `50` | Jobs claimed per polling cycle per worker |
| `SKIP_OSV` | `false` | Skip OSV vulnerability API calls. Set `true` for fast initial bulk load. |
| `CUSTOM_THEME` | (example file) | Path to a custom CSS theme file for the UI |

### Useful Make targets

| Command | Description |
|---------|-------------|
| `make dev` | Start full stack via Docker Compose |
| `make dev-down` | Stop all containers |
| `make dev-restart` | Restart with new `.env` values (keeps data) |
| `make dev-reset` | Destroy data volumes and restart fresh |
| `make re-ingest` | Re-trigger the Ingestion Watcher (scans for new files) |
| `make re-scan` | Wipe all data and re-process everything (e.g. after enabling OSV) |
| `make dev-status` | Show container status + ingestion progress |
| `make ch-shell` | Open a ClickHouse CLI |

---

## Summary

| What | Where | How to Change |
|---|---|---|
| **SBOMs** | Git repo → git-sync → emptyDir volume | Push to Git, CronJob re-syncs every 6h |
| **VEX files** | Same directory as SBOMs | Place `*.openvex.json` or `*.vex.json` alongside SBOMs |
| **License Exceptions** | `seebom-license-exceptions` ConfigMap | `kubectl edit configmap` → restart API |
| **License Policy** | `seebom-license-policy` ConfigMap | `kubectl edit configmap` → restart API + Workers |
| **Custom Theme** | `seebom-custom-theme` ConfigMap | `kubectl create configmap` → restart UI |
| **Dark Mode** | Built-in toggle (navbar) | User preference, stored in browser localStorage |
| **ClickHouse password** | `seebom-secret` Secret | `kubectl edit secret` |

