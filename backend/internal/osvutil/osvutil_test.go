package osvutil

import (
	"testing"

	"github.com/seebom-labs/seebom/backend/internal/osv"
)

func TestClassifySeverity_CVSS(t *testing.T) {
	tests := []struct {
		name     string
		entry    osv.VulnEntry
		expected string
	}{
		{
			name: "critical (9.8)",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V3", Score: "9.8"}},
			},
			expected: "CRITICAL",
		},
		{
			name: "high (7.5)",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V3", Score: "7.5"}},
			},
			expected: "HIGH",
		},
		{
			name: "medium (5.3)",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V3", Score: "5.3"}},
			},
			expected: "MEDIUM",
		},
		{
			name: "low (2.1)",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V3", Score: "2.1"}},
			},
			expected: "LOW",
		},
		{
			name: "critical boundary (9.0)",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V3", Score: "9.0"}},
			},
			expected: "CRITICAL",
		},
		{
			name: "high boundary (7.0)",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V3", Score: "7.0"}},
			},
			expected: "HIGH",
		},
		{
			name: "medium boundary (4.0)",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V3", Score: "4.0"}},
			},
			expected: "MEDIUM",
		},
		{
			name: "no CVSS_V3 but has severity",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V2", Score: "5.0"}},
			},
			expected: "MEDIUM",
		},
		{
			name:     "no severity at all",
			entry:    osv.VulnEntry{},
			expected: "LOW",
		},
		{
			name: "CVSS vector string (unparseable as float)",
			entry: osv.VulnEntry{
				Severity: []osv.Severity{{Type: "CVSS_V3", Score: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H"}},
			},
			expected: "MEDIUM", // defaults to 5.0 → MEDIUM
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifySeverity(tt.entry)
			if got != tt.expected {
				t.Errorf("ClassifySeverity() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseCVSSScore(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"9.8", 9.8},
		{"7.5", 7.5},
		{"0.0", 0.0},
		{"10.0", 10.0},
		{"CVSS:3.1/AV:N", 5.0}, // unparseable → default
		{"", 5.0},              // empty → default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseCVSSScore(tt.input)
			if got != tt.expected {
				t.Errorf("ParseCVSSScore(%q) = %f, want %f", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractFixedVersion(t *testing.T) {
	tests := []struct {
		name     string
		entry    osv.VulnEntry
		expected string
	}{
		{
			name:     "no affected data",
			entry:    osv.VulnEntry{},
			expected: "",
		},
		{
			name: "has fixed version",
			entry: osv.VulnEntry{
				Affected: []osv.Affected{{
					Ranges: []osv.Range{{
						Events: []osv.Event{
							{Introduced: "0"},
							{Fixed: "1.2.3"},
						},
					}},
				}},
			},
			expected: "1.2.3",
		},
		{
			name: "multiple ranges, first fixed wins",
			entry: osv.VulnEntry{
				Affected: []osv.Affected{{
					Ranges: []osv.Range{
						{Events: []osv.Event{{Introduced: "0"}, {Fixed: "1.0.0"}}},
						{Events: []osv.Event{{Introduced: "2.0.0"}, {Fixed: "2.1.0"}}},
					},
				}},
			},
			expected: "1.0.0",
		},
		{
			name: "no fixed event",
			entry: osv.VulnEntry{
				Affected: []osv.Affected{{
					Ranges: []osv.Range{{
						Events: []osv.Event{{Introduced: "0"}},
					}},
				}},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFixedVersion(tt.entry)
			if got != tt.expected {
				t.Errorf("ExtractFixedVersion() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExtractAffectedVersions(t *testing.T) {
	tests := []struct {
		name     string
		entry    osv.VulnEntry
		expected int // length of result
	}{
		{
			name:     "no affected data",
			entry:    osv.VulnEntry{},
			expected: 0,
		},
		{
			name: "has versions",
			entry: osv.VulnEntry{
				Affected: []osv.Affected{{
					Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
				}},
			},
			expected: 3,
		},
		{
			name: "multiple affected blocks",
			entry: osv.VulnEntry{
				Affected: []osv.Affected{
					{Versions: []string{"1.0.0"}},
					{Versions: []string{"2.0.0", "2.1.0"}},
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAffectedVersions(tt.entry)
			if got == nil {
				t.Fatal("ExtractAffectedVersions should never return nil")
			}
			if len(got) != tt.expected {
				t.Errorf("ExtractAffectedVersions() returned %d versions, want %d", len(got), tt.expected)
			}
		})
	}
}
