# SeeBOM – Testing Guide

> **Updated:** 2026-03-28

## Quick Start

```bash
# Run all tests
cd backend && go test ./... -count=1

# Run with verbose output
go test ./... -v -count=1

# Run tests for a specific package
go test ./internal/vex/ -v -count=1

# Run a single test by name
go test ./internal/vex/ -run TestNormalizeVulnID -v

# Run with race detection (used in CI)
go test ./... -count=1 -race

# Check coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Test Structure

### File Location

Tests live next to the code they test, using Go's `_test.go` convention:

```
backend/internal/
├── config/
│   ├── config.go
│   └── config_test.go          ← tests for config.Load()
├── github/
│   ├── purl.go
│   ├── purl_test.go            ← tests for ExtractGitHubRepo (19 PURL patterns incl. well-known Go module mappings)
│   ├── resolver.go
│   └── resolver_test.go        ← tests for Resolve, ResolveWithMetadata, cache, well-known overrides (httptest mock)
├── license/
│   ├── checker.go
│   ├── checker_test.go         ← tests for Categorize, Check, LoadPolicy, etc.
│   └── exceptions.go
├── osv/
│   ├── client.go
│   └── client_test.go          ← tests for QueryBatch (with httptest mock)
├── osvutil/
│   ├── osvutil.go
│   └── osvutil_test.go         ← tests for ClassifySeverity, ParseCVSSScore, ExtractFixedVersion, ExtractAffectedVersions
├── repo/
│   ├── scanner.go
│   └── scanner_test.go         ← tests for Scan (with t.TempDir())
├── s3/
│   ├── client.go
│   └── client_test.go          ← tests for ClassifyKey, ParseURI, ObjectInfo
├── spdx/
│   ├── parser.go
│   └── parser_test.go          ← tests for Parse (plain SPDX + in-toto attestation envelope)
└── vex/
    ├── parser.go
    └── parser_test.go          ← tests for Parse, normalizeVulnID
```

### No Tests Needed

These packages contain only data types (structs) with no logic:

- `pkg/models/` – SBOM, Vulnerability, VEXStatement structs
- `pkg/dto/` – API response DTOs

---

## Current Test Inventory

| Package | Top-Level | Subtests | What's Covered |
|---------|-----------|----------|---------------|
| `config` | 7 | 0 | Default values, custom env vars, S3 buckets JSON, single S3 bucket, shared S3 credentials, invalid S3 JSON, S3BucketNames |
| `github/purl` | 2 | 22 | ExtractGitHubRepo (19 PURL patterns: golang github.com, subpath, pkg:github scheme, well-known Go module mappings for golang.org/x/crypto, gopkg.in/yaml.v3, go.uber.org/zap, k8s.io/client-go, oras.land/oras-go/v2 with version suffix stripping, dario.cat/mergo, go.yaml.in/yaml/v4, unknown non-github, npm, empty, qualifiers, fragments, missing repo, azure submodule, hamba v2), RepoKey (4 patterns) |
| `github/resolver` | 11 | 0 | Resolve (happy path, cache hit, non-GitHub PURL, well-known mapping resolves golang.org/x/crypto), ResolveWithMetadata (archived repo, not-found, non-GitHub, cache hit), PreloadCache, PreloadMetadataCache, CacheEntries, MetadataCacheEntries |
| `license` | 24 | 20 | Categorize (14 SPDX IDs incl. BSD-3-Clause, ISC, 0BSD, NOASSERTION, NONE), Check, CheckWithExceptions (blanket + package + prefix), LoadPolicy, LoadExceptions, LoadExceptionsWithFallback (4 scenarios), BuildIndex (empty, All CNCF Projects promoted to blanket, compound OR, compound AND), IsExempt substring matching (package+license, package-any), SplitLicenses (7 patterns), GoTempNamesFiltered, edge cases |
| `osv` | 6 | 0 | Empty input, mock server, server error, context cancellation, no-vulns response, HydrateVulns cache |
| `osvutil` | 5 | 35 | ClassifySeverity (16 CVSS scenarios incl. vector strings, database-specific fallback), ParseCVSSScore (8 inputs incl. vectors), ComputeCVSSv3BaseScore (4 scenarios), ExtractFixedVersion (4 scenarios), ExtractAffectedVersions (3 scenarios) |
| `repo` | 5 | 0 | File scanning (SBOM + VEX detection), empty dir, nested dirs, SHA256 consistency, nonexistent dir |
| `s3` | 4 | 15 | ClassifyKey (9 patterns incl. _spdx.json, case-insensitive), ParseURI (6 patterns), BucketConfig defaults, ObjectInfo URI |
| `spdx` | 8 | 7 | Full parse, in-toto attestation envelope unwrapping, invalid JSON, empty packages, deterministic SBOM ID, license fallback, GoTempModuleName, CleanPackageName (8 patterns) |
| `vex` | 5 | 8 | Full parse, invalid JSON, empty doc, normalizeVulnID (9 URL patterns), URL-based vuln @id |
| **Total** | **77** | **107** | **184 test invocations** |

---

## How to Write Tests

### 1. Naming Convention

```go
// Test function: Test<FunctionName>_<Scenario>
func TestParse_InvalidJSON(t *testing.T) { ... }
func TestQueryBatch_ServerError(t *testing.T) { ... }
func TestScanner_Scan_EmptyDir(t *testing.T) { ... }

// Table-driven subtests: use t.Run()
func TestCategorize(t *testing.T) {
    tests := []struct {
        input    string
        expected Category
    }{
        {"MIT", CategoryPermissive},
        {"GPL-3.0-only", CategoryCopyleft},
    }
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got := Categorize(tt.input)
            if got != tt.expected {
                t.Errorf("Categorize(%q) = %q, want %q", tt.input, got, tt.expected)
            }
        })
    }
}
```

### 2. Test Patterns Used in This Project

#### Table-Driven Tests (preferred for multiple inputs)

Used in: `license/checker_test.go`, `vex/parser_test.go`

```go
func TestNormalizeVulnID(t *testing.T) {
    tests := []struct {
        input string
        want  string
    }{
        {"CVE-2024-1234", "CVE-2024-1234"},
        {"https://pkg.go.dev/vuln/GO-2025-4188", "GO-2025-4188"},
        {"", ""},
    }
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got := normalizeVulnID(tt.input)
            if got != tt.want {
                t.Errorf("normalizeVulnID(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

#### Inline JSON Test Data (for parsers)

Used in: `spdx/parser_test.go`, `vex/parser_test.go`

```go
const testSPDXJSON = `{
    "spdxVersion": "SPDX-2.3",
    "name": "test-document",
    ...
}`

func TestParse(t *testing.T) {
    result, err := Parse(strings.NewReader(testSPDXJSON), "test.spdx.json", "abc123")
    if err != nil {
        t.Fatalf("Parse() returned error: %v", err)
    }
    // Assert fields...
}
```

#### httptest Mock Server (for HTTP clients)

Used in: `osv/client_test.go`

```go
func TestQueryBatch_MockServer(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"results": [{"vulns": [...]}]}`))
    }))
    defer server.Close()

    client := &Client{
        baseURL:    server.URL,
        httpClient: server.Client(),
        limiter:    getGlobalLimiter(),
    }

    resp, err := client.QueryBatch(context.Background(), []string{"pkg:npm/test@1.0.0"})
    // Assert...
}
```

#### t.TempDir() for Filesystem Tests

Used in: `repo/scanner_test.go`, `license/checker_test.go`

```go
func TestScanner_Scan(t *testing.T) {
    tmpDir := t.TempDir()  // Automatically cleaned up after test

    os.WriteFile(filepath.Join(tmpDir, "test.spdx.json"), []byte(`{...}`), 0644)
    os.WriteFile(filepath.Join(tmpDir, "cve.openvex.json"), []byte(`{...}`), 0644)

    scanner := NewScanner(tmpDir)
    files, err := scanner.Scan()
    // Assert file count, types, hashes...
}
```

#### t.Setenv() for Environment Variables

Used in: `config/config_test.go`

```go
func TestLoad_CustomEnv(t *testing.T) {
    t.Setenv("CLICKHOUSE_HOST", "ch-prod")   // Automatically restored after test
    t.Setenv("SKIP_OSV", "true")

    cfg, err := Load()
    // Assert config values...
}
```

### 3. What Every Test Must Cover

For each function, write tests for:

| Scenario | Example |
|----------|---------|
| **Happy path** | `TestParse` – valid SPDX JSON → correct result |
| **Invalid input** | `TestParse_InvalidJSON` – malformed JSON → error |
| **Empty input** | `TestCheck_EmptyInput` – nil slices → no panic, empty result |
| **Edge cases** | `TestNormalizeVulnID` – URLs, empty string, plain IDs |
| **Error conditions** | `TestLoadPolicy_MissingFile` – file not found → error |
| **Determinism** | `TestParse_DeterministicSBOMID` – same input → same output |

### 4. Test Requirements

- **No external dependencies.** Tests must not require a running ClickHouse, network access, or Docker. Use `httptest` for HTTP, `t.TempDir()` for files, `t.Setenv()` for env vars.
- **No test order dependency.** Each test must be self-contained and pass in isolation.
- **Use `t.Fatalf` for setup failures.** If a precondition fails, stop immediately. Use `t.Errorf` for assertion failures (allows multiple failures per test).
- **Use `t.Run()` for subtests.** Table-driven tests must use subtests for clear output.
- **No `init()` in test files.** All setup happens inside the test function.
- **Prefer `strings.NewReader` over files** for parser tests (faster, no I/O).
- **Race-safe.** All tests must pass with `-race` flag.

---

## CI Integration

Tests run automatically on every push/PR via GitHub Actions (`.github/workflows/ci.yml`):

```yaml
- name: Test
  working-directory: backend
  run: go test ./... -count=1 -race
```

The `-race` flag enables Go's race detector – this catches concurrent access bugs in the workers/queue code.

---

## Adding Tests for New Features

When adding a new feature, follow this checklist:

1. **Create `<package>_test.go`** next to the source file
2. **Write at minimum:**
   - One happy-path test
   - One error/invalid-input test
   - One edge-case test
3. **Run locally:** `go test ./internal/<package>/ -v -count=1`
4. **Check coverage:** `go test ./internal/<package>/ -coverprofile=c.out && go tool cover -func=c.out`
5. **Run full suite:** `go test ./... -count=1 -race`
6. **CI will enforce:** all tests must pass before merge

---

## Packages That Still Need More Tests

The `internal/clickhouse/` package (client, queue, insert, queries) contains no unit tests because it requires a running ClickHouse instance. Future work:

- **Option A:** Integration tests with [testcontainers-go](https://golang.testcontainers.org/) spinning up a ClickHouse container
- **Option B:** Interface-based mocking of the ClickHouse client for query logic tests

The `cmd/` packages (main functions) are tested implicitly through integration via Docker Compose but have no unit tests. These are thin orchestration layers that wire together the tested internal packages.

