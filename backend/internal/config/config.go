package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration values, read from environment variables.
type Config struct {
	// ClickHouse connection
	ClickHouseHost     string
	ClickHousePort     int
	ClickHouseDatabase string
	ClickHouseUser     string
	ClickHousePassword string

	// SBOM source directory
	SBOMDir string

	// API Gateway
	APIPort int

	// Worker
	WorkerID        string
	WorkerBatchSize int

	// Feature flags
	SkipOSV           bool   // Skip OSV vulnerability lookups (fast ingestion, licenses only)
	SkipGitHubResolve bool   // Skip GitHub license resolution for unknown licenses
	SBOMLimit         int    // Max number of SBOMs to enqueue (0 = unlimited)
	ExceptionsFile    string // Path to license-exceptions.json
	LicensePolicyFile string // Path to license-policy.json
	GitHubToken       string // GitHub personal access token (optional, increases rate limit)
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		ClickHouseHost:     getEnv("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:     getEnvInt("CLICKHOUSE_PORT", 9000),
		ClickHouseDatabase: getEnv("CLICKHOUSE_DATABASE", "seebom"),
		ClickHouseUser:     getEnv("CLICKHOUSE_USER", "default"),
		ClickHousePassword: getEnv("CLICKHOUSE_PASSWORD", ""),
		SBOMDir:            getEnv("SBOM_DIR", "./sboms"),
		APIPort:            getEnvInt("API_PORT", 8080),
		WorkerID:           getEnv("WORKER_ID", ""),
		WorkerBatchSize:    getEnvInt("WORKER_BATCH_SIZE", 10),
		SkipOSV:            getEnvBool("SKIP_OSV", false),
		SkipGitHubResolve:  getEnvBool("SKIP_GITHUB_RESOLVE", false),
		SBOMLimit:          getEnvInt("SBOM_LIMIT", 0),
		ExceptionsFile:     getEnv("EXCEPTIONS_FILE", "/data/config/license-exceptions.json"),
		LicensePolicyFile:  getEnv("LICENSE_POLICY_FILE", "/data/config/license-policy.json"),
		GitHubToken:        getEnv("GITHUB_TOKEN", ""),
	}

	if cfg.WorkerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname for worker ID: %w", err)
		}
		cfg.WorkerID = hostname
	}

	return cfg, nil
}

// ClickHouseDSN returns the ClickHouse connection string.
func (c *Config) ClickHouseDSN() string {
	return fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s",
		c.ClickHouseUser, c.ClickHousePassword,
		c.ClickHouseHost, c.ClickHousePort, c.ClickHouseDatabase)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvBool(key string, fallback bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return val == "1" || val == "true" || val == "yes"
}
