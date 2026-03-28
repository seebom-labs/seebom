---
title: "Testing"
linkTitle: "Testing"
type: docs
weight: 1
description: >
  How to run tests, write new tests, and understand the test structure.
---

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

## Test Structure

Tests live next to the code they test, using Go's `_test.go` convention:

```
backend/internal/
├── config/
│   ├── config.go
│   └── config_test.go
├── github/
│   ├── purl.go
│   ├── purl_test.go
│   ├── resolver.go
│   └── resolver_test.go
├── license/
│   ├── checker.go
│   └── checker_test.go
├── osv/
│   ├── client.go
│   └── client_test.go
├── osvutil/
│   ├── osvutil.go
│   └── osvutil_test.go
├── repo/
│   ├── scanner.go
│   └── scanner_test.go
├── s3/
│   ├── client.go
│   └── client_test.go
├── spdx/
│   ├── parser.go
│   └── parser_test.go
└── vex/
    ├── parser.go
    └── parser_test.go
```

## Current Test Inventory

| Package | Tests | Subtests | What's Covered |
|---------|-------|----------|---------------|
| `config` | 7 | 0 | Default values, env vars, S3 buckets JSON, shared credentials |
| `github/purl` | 2 | 22 | ExtractGitHubRepo (19 PURL patterns incl. well-known Go module mappings: `golang.org/x/*`, `gopkg.in/*`, `go.uber.org/*`, `k8s.io/*`, `oras.land/*`, `dario.cat/*`, `go.yaml.in/*`), RepoKey |
| `github/resolver` | 11 | 0 | Resolve (incl. well-known mapping for golang.org/x/crypto), cache, metadata, preload, license overrides |
| `license` | 24 | 20 | Categorize, Check, policy, exceptions, prefix matching |
| `osv` | 6 | 0 | QueryBatch, errors, cancellation |
| `osvutil` | 5 | 35 | Severity, CVSS, fixed versions |
| `repo` | 5 | 0 | File scanning, SHA256, nested dirs |
| `s3` | 4 | 15 | ClassifyKey, ParseURI, defaults |
| `spdx` | 8 | 7 | Full parse, in-toto attestation envelope unwrapping, invalid JSON, deterministic IDs |
| `vex` | 5 | 8 | Parse, normalizeVulnID, URL patterns |
| **Total** | **77** | **107** | **184 test invocations** |

## Test Patterns

### Table-Driven Tests

```go
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

### httptest Mock Server

```go
func TestQueryBatch_MockServer(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"results": [{"vulns": [...]}]}`))
    }))
    defer server.Close()
    // Use server.URL as base URL...
}
```

### t.TempDir() for Filesystem Tests

```go
func TestScanner_Scan(t *testing.T) {
    tmpDir := t.TempDir()
    os.WriteFile(filepath.Join(tmpDir, "test.spdx.json"), []byte(`{...}`), 0644)
    scanner := NewScanner(tmpDir)
    files, err := scanner.Scan()
    // Assert...
}
```

## Test Requirements

- **No external dependencies.** No running ClickHouse, network, or Docker needed.
- **No test order dependency.** Each test is self-contained.
- **Race-safe.** All tests must pass with `-race` flag.
- **Use subtests** (`t.Run()`) for table-driven tests.

## CI Integration

Tests run automatically on every push/PR:

```yaml
- name: Test
  working-directory: backend
  run: go test ./... -count=1 -race
```

