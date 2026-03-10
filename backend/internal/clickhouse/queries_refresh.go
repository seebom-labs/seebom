package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RefreshLog represents a CVE refresh run.
type RefreshLog struct {
	RefreshID     uuid.UUID
	StartedAt     time.Time
	FinishedAt    time.Time
	PURLsChecked  uint64
	NewVulnsFound uint64
	Status        string // running, completed, failed
}

// SBOMRef is a lightweight reference to an SBOM (id + source_file).
type SBOMRef struct {
	SBOMID     uuid.UUID
	SourceFile string
}

// QueryDistinctPURLs returns all unique PURLs across all ingested SBOMs.
func (c *Client) QueryDistinctPURLs(ctx context.Context) ([]string, error) {
	rows, err := c.Conn.Query(ctx, `
		SELECT DISTINCT purl
		FROM (
			SELECT arrayJoin(package_purls) AS purl
			FROM sbom_packages FINAL
		)
		WHERE purl != ''
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct PURLs: %w", err)
	}
	defer rows.Close()

	var purls []string
	for rows.Next() {
		var purl string
		if err := rows.Scan(&purl); err != nil {
			return nil, fmt.Errorf("failed to scan PURL: %w", err)
		}
		purls = append(purls, purl)
	}
	return purls, nil
}

// QueryExistingVulnKeys returns a set of existing (vuln_id, purl) pairs for deduplication.
func (c *Client) QueryExistingVulnKeys(ctx context.Context) (map[string]struct{}, error) {
	rows, err := c.Conn.Query(ctx, `
		SELECT DISTINCT vuln_id, purl
		FROM vulnerabilities FINAL
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing vuln keys: %w", err)
	}
	defer rows.Close()

	keys := make(map[string]struct{})
	for rows.Next() {
		var vulnID, purl string
		if err := rows.Scan(&vulnID, &purl); err != nil {
			return nil, fmt.Errorf("failed to scan vuln key: %w", err)
		}
		keys[vulnID+"\x00"+purl] = struct{}{}
	}
	return keys, nil
}

// QuerySBOMsByPURL returns all SBOMs that contain a given PURL in their dependency tree.
func (c *Client) QuerySBOMsByPURL(ctx context.Context, purl string) ([]SBOMRef, error) {
	rows, err := c.Conn.Query(ctx, `
		SELECT sbom_id, source_file
		FROM sbom_packages FINAL
		WHERE has(package_purls, ?)
	`, purl)
	if err != nil {
		return nil, fmt.Errorf("failed to query SBOMs by PURL %s: %w", purl, err)
	}
	defer rows.Close()

	var refs []SBOMRef
	for rows.Next() {
		var ref SBOMRef
		if err := rows.Scan(&ref.SBOMID, &ref.SourceFile); err != nil {
			return nil, fmt.Errorf("failed to scan SBOM ref: %w", err)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// InsertRefreshLog writes a CVE refresh log entry.
func (c *Client) InsertRefreshLog(ctx context.Context, log RefreshLog) error {
	return c.Conn.Exec(ctx, `
		INSERT INTO cve_refresh_log (refresh_id, started_at, finished_at, purls_checked, new_vulns_found, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, log.RefreshID, log.StartedAt, log.FinishedAt, log.PURLsChecked, log.NewVulnsFound, log.Status)
}

// QueryLastRefreshTime returns the timestamp of the most recent completed CVE refresh.
func (c *Client) QueryLastRefreshTime(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := c.Conn.QueryRow(ctx, `
		SELECT max(finished_at) FROM cve_refresh_log FINAL WHERE status = 'completed'
	`).Scan(&t)
	if err != nil {
		return time.Time{}, nil // Not an error – just no refresh yet.
	}
	return t, nil
}
