package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any env vars that might interfere.
	for _, key := range []string{
		"CLICKHOUSE_HOST", "CLICKHOUSE_PORT", "CLICKHOUSE_DATABASE",
		"SBOM_DIR", "API_PORT", "WORKER_BATCH_SIZE", "SKIP_OSV", "SBOM_LIMIT",
	} {
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.ClickHouseHost != "localhost" {
		t.Errorf("expected default host localhost, got %s", cfg.ClickHouseHost)
	}
	if cfg.ClickHousePort != 9000 {
		t.Errorf("expected default port 9000, got %d", cfg.ClickHousePort)
	}
	if cfg.ClickHouseDatabase != "seebom" {
		t.Errorf("expected default database seebom, got %s", cfg.ClickHouseDatabase)
	}
	if cfg.SBOMDir != "./sboms" {
		t.Errorf("expected default SBOM dir ./sboms, got %s", cfg.SBOMDir)
	}
	if cfg.APIPort != 8080 {
		t.Errorf("expected default API port 8080, got %d", cfg.APIPort)
	}
	if cfg.SkipOSV != false {
		t.Error("expected default SkipOSV=false")
	}
	if cfg.SBOMLimit != 0 {
		t.Errorf("expected default SBOMLimit=0, got %d", cfg.SBOMLimit)
	}
	if cfg.WorkerID == "" {
		t.Error("expected non-empty WorkerID (should be hostname)")
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	t.Setenv("CLICKHOUSE_HOST", "ch-prod")
	t.Setenv("CLICKHOUSE_PORT", "9440")
	t.Setenv("CLICKHOUSE_DATABASE", "mydb")
	t.Setenv("SBOM_DIR", "/data/sboms")
	t.Setenv("API_PORT", "9090")
	t.Setenv("WORKER_BATCH_SIZE", "100")
	t.Setenv("SKIP_OSV", "true")
	t.Setenv("SBOM_LIMIT", "500")
	t.Setenv("WORKER_ID", "test-worker-1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.ClickHouseHost != "ch-prod" {
		t.Errorf("expected ch-prod, got %s", cfg.ClickHouseHost)
	}
	if cfg.ClickHousePort != 9440 {
		t.Errorf("expected 9440, got %d", cfg.ClickHousePort)
	}
	if cfg.ClickHouseDatabase != "mydb" {
		t.Errorf("expected mydb, got %s", cfg.ClickHouseDatabase)
	}
	if cfg.SBOMDir != "/data/sboms" {
		t.Errorf("expected /data/sboms, got %s", cfg.SBOMDir)
	}
	if cfg.APIPort != 9090 {
		t.Errorf("expected 9090, got %d", cfg.APIPort)
	}
	if cfg.WorkerBatchSize != 100 {
		t.Errorf("expected 100, got %d", cfg.WorkerBatchSize)
	}
	if cfg.SkipOSV != true {
		t.Error("expected SkipOSV=true")
	}
	if cfg.SBOMLimit != 500 {
		t.Errorf("expected 500, got %d", cfg.SBOMLimit)
	}
	if cfg.WorkerID != "test-worker-1" {
		t.Errorf("expected test-worker-1, got %s", cfg.WorkerID)
	}
}
