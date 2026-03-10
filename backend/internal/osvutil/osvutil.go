// Package osvutil provides shared utility functions for processing OSV vulnerability data.
package osvutil

import (
	"fmt"
	"github.com/seebom-labs/seebom/backend/internal/osv"
)

// ClassifySeverity maps OSV severity data to a simple category (CRITICAL/HIGH/MEDIUM/LOW).
func ClassifySeverity(v osv.VulnEntry) string {
	for _, s := range v.Severity {
		if s.Type == "CVSS_V3" {
			score := ParseCVSSScore(s.Score)
			switch {
			case score >= 9.0:
				return "CRITICAL"
			case score >= 7.0:
				return "HIGH"
			case score >= 4.0:
				return "MEDIUM"
			default:
				return "LOW"
			}
		}
	}
	if len(v.Severity) > 0 {
		return "MEDIUM"
	}
	return "LOW"
}

// ParseCVSSScore extracts a numeric score from a CVSS vector string or numeric string.
func ParseCVSSScore(score string) float64 {
	var f float64
	_, err := fmt.Sscanf(score, "%f", &f)
	if err == nil {
		return f
	}
	return 5.0
}

// ExtractFixedVersion finds the first "fixed" version from OSV affected data.
func ExtractFixedVersion(v osv.VulnEntry) string {
	for _, a := range v.Affected {
		for _, r := range a.Ranges {
			for _, e := range r.Events {
				if e.Fixed != "" {
					return e.Fixed
				}
			}
		}
	}
	return ""
}

// ExtractAffectedVersions collects all explicitly listed affected versions.
func ExtractAffectedVersions(v osv.VulnEntry) []string {
	var versions []string
	for _, a := range v.Affected {
		versions = append(versions, a.Versions...)
	}
	if versions == nil {
		versions = []string{}
	}
	return versions
}
