---
title: "FAQ"
linkTitle: "FAQ"
type: docs
weight: 6
description: >
  Frequently asked questions about operating and troubleshooting SeeBOM.
---

{{% pageinfo %}}
Common questions about running SeeBOM in Kubernetes — re-ingestion, data resets, and day-to-day operations.
{{% /pageinfo %}}

## How do I force a re-ingestion?

The Ingestion Watcher runs as a **CronJob** (default: every 6 hours). It deduplicates files by SHA-256 hash — only new or changed SBOMs are enqueued. There are two scenarios:

### Ingest new/changed files immediately

Trigger the CronJob manually instead of waiting for the next scheduled run:

```bash
kubectl create job --from=cronjob/<RELEASE>-ingestion-watcher manual-ingest-$(date +%s) -n <NAMESPACE>
```

Replace `<RELEASE>` with your Helm release name (e.g. `seebom`) and `<NAMESPACE>` with your namespace.

**Example:**

```bash
kubectl create job --from=cronjob/seebom-ingestion-watcher manual-ingest-$(date +%s) -n seebom
```

This creates a one-off Job from the CronJob spec. The Watcher scans all sources (S3 buckets + local PVC), skips already-processed files, and enqueues anything new.

### Full re-ingestion from scratch

If you need to **reprocess everything** (e.g. after changing the license policy, enabling OSV scanning, or upgrading parsing logic), you must first truncate all data tables and then trigger re-ingestion:

**Step 1 — Truncate all data tables:**

```bash
kubectl exec -n <NAMESPACE> <CLICKHOUSE_POD> -c clickhouse -- \
  clickhouse-client --database=seebom --password="<PASSWORD>" --multiquery \
  --query "TRUNCATE TABLE ingestion_queue; \
           TRUNCATE TABLE license_compliance; \
           TRUNCATE TABLE vulnerabilities; \
           TRUNCATE TABLE sbom_packages; \
           TRUNCATE TABLE sboms; \
           TRUNCATE TABLE vex_statements;"
```

The default ClickHouse pod name follows this pattern:
`chi-<RELEASE>-clickhouse-<CLUSTER_NAME>-0-0-0`

For a typical installation this would be:

```bash
kubectl exec -n seebom chi-seebom-clickhouse-seebom-cluster-0-0-0 -c clickhouse -- \
  clickhouse-client --database=seebom --password="$CLICKHOUSE_PASSWORD" --multiquery \
  --query "TRUNCATE TABLE ingestion_queue; \
           TRUNCATE TABLE license_compliance; \
           TRUNCATE TABLE vulnerabilities; \
           TRUNCATE TABLE sbom_packages; \
           TRUNCATE TABLE sboms; \
           TRUNCATE TABLE vex_statements;"
```

**Step 2 — Trigger re-ingestion:**

```bash
kubectl create job --from=cronjob/seebom-ingestion-watcher reingest-$(date +%s) -n seebom
```

**Step 3 — Monitor progress:**

```bash
# Watch the watcher job
kubectl logs -n seebom -l job-name=reingest-<TIMESTAMP> -f

# Watch the parsing workers
kubectl logs -n seebom -l app.kubernetes.io/component=parsing-worker --tail=20 -f

# Check dashboard stats via API
kubectl exec -n seebom deploy/seebom-api-gateway -- \
  wget -qO- http://localhost:8080/api/v1/stats/dashboard | jq '.total_sboms, .total_packages'
```

{{% alert title="Note" color="info" %}}
If you use the **Kind** local development setup, you can use the Makefile shortcut instead:

```bash
make kind-reingest
```

This truncates all tables and triggers re-ingestion in one step.
{{% /alert %}}

---

## When do I need a full re-ingestion?

A full re-ingestion (truncate + re-ingest) is required when:

| Scenario | Why |
|---|---|
| Changed the **license policy** (`license-policy.json`) | Existing packages need reclassification |
| Enabled/disabled **OSV scanning** (`skipOSV`) | Vulnerability data needs to be fetched or cleared |
| Enabled/disabled **GitHub license resolution** (`skipGitHubResolve`) | Unknown licenses need re-resolution |
| Upgraded **parsing logic** (new image version) | Existing SBOMs may parse differently (e.g., new in-toto attestation support or improved license resolution with well-known Go module mappings) |
| Changed **license exceptions** | Exception matching is applied during ingestion |
| Added a **GitHub token** (`GITHUB_TOKEN`) | Previously rate-limited resolution may have missed packages — a re-ingestion with the token resolves all licenses |

For these changes, a simple incremental re-trigger will **not** reprocess existing files because the SHA-256 hashes haven't changed.

{{% alert title="Tip" color="success" %}}
Changes to **VEX files** do **not** require a full re-ingestion. VEX matching is applied at query time. Simply add or update your `.openvex.json` files and trigger an incremental re-ingestion to pick them up.
{{% /alert %}}

---

## How do I check ingestion progress?

```bash
# Job queue status
kubectl exec -n seebom <CLICKHOUSE_POD> -c clickhouse -- \
  clickhouse-client --database=seebom --password="$CLICKHOUSE_PASSWORD" \
  --query "SELECT argMax(status, created_at) AS status, count() AS cnt \
           FROM ingestion_queue \
           GROUP BY job_id \
           HAVING status != '' \
           GROUP BY status \
           ORDER BY status" \
  --format=PrettyCompact

# Data summary
kubectl exec -n seebom <CLICKHOUSE_POD> -c clickhouse -- \
  clickhouse-client --database=seebom --password="$CLICKHOUSE_PASSWORD" \
  --query "SELECT 'sboms' AS table, count() AS rows FROM sboms FINAL \
           UNION ALL SELECT 'packages', count() FROM sbom_packages FINAL \
           UNION ALL SELECT 'vulns', count() FROM vulnerabilities FINAL \
           UNION ALL SELECT 'licenses', count() FROM license_compliance FINAL" \
  --format=PrettyCompact
```

Or use the API endpoint:

```bash
curl -s http://<API_HOST>/api/v1/stats/dashboard | jq .
```

---

## How do I trigger a CVE refresh?

The CVE Refresher runs as a CronJob (default: daily at 2 AM). To trigger it immediately:

```bash
kubectl create job --from=cronjob/seebom-cve-refresher manual-cve-refresh-$(date +%s) -n seebom
```

This checks all known PURLs against the [OSV database](https://osv.dev) for newly disclosed vulnerabilities.
