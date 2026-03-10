package license

import (
	"fmt"
	"os"
	"strings"
	"sync"

	json "github.com/goccy/go-json"
)

// Category represents a license compliance category.
type Category string

const (
	CategoryPermissive Category = "permissive"
	CategoryCopyleft   Category = "copyleft"
	CategoryUnknown    Category = "unknown"
)

// PolicyFile represents the license-policy.json structure.
type PolicyFile struct {
	Description string   `json:"description,omitempty"`
	Permissive  []string `json:"permissive"`
	Copyleft    []string `json:"copyleft"`
}

// Policy holds the loaded license classification maps.
type Policy struct {
	permissive map[string]bool
	copyleft   map[string]bool
}

// built-in defaults, used when no policy file is provided.
var defaultPermissive = []string{
	"MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC",
	"Unlicense", "0BSD", "CC0-1.0", "Zlib", "BSL-1.0", "PSF-2.0",
}

var defaultCopyleft = []string{
	"GPL-2.0-only", "GPL-2.0-or-later", "GPL-3.0-only", "GPL-3.0-or-later",
	"LGPL-2.1-only", "LGPL-2.1-or-later", "LGPL-3.0-only", "LGPL-3.0-or-later",
	"AGPL-3.0-only", "AGPL-3.0-or-later", "MPL-2.0",
	"EPL-1.0", "EPL-2.0", "EUPL-1.2", "CPAL-1.0",
}

// global active policy, protected by a mutex for hot-reload safety.
var (
	activePolicy *Policy
	policyMu     sync.RWMutex
)

func init() {
	// Start with defaults.
	activePolicy = buildPolicy(defaultPermissive, defaultCopyleft)
}

func buildPolicy(permissive, copyleft []string) *Policy {
	p := &Policy{
		permissive: make(map[string]bool, len(permissive)),
		copyleft:   make(map[string]bool, len(copyleft)),
	}
	for _, id := range permissive {
		p.permissive[id] = true
	}
	for _, id := range copyleft {
		p.copyleft[id] = true
	}
	return p
}

// LoadPolicy reads a license-policy.json file and replaces the active policy.
// Returns the number of permissive and copyleft licenses loaded.
func LoadPolicy(path string) (int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read policy file %s: %w", path, err)
	}

	var pf PolicyFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return 0, 0, fmt.Errorf("failed to parse policy file %s: %w", path, err)
	}

	p := buildPolicy(pf.Permissive, pf.Copyleft)

	policyMu.Lock()
	activePolicy = p
	policyMu.Unlock()

	return len(pf.Permissive), len(pf.Copyleft), nil
}

// GetPolicy returns the current active policy (for API serialization).
func GetPolicy() *PolicyFile {
	policyMu.RLock()
	defer policyMu.RUnlock()

	pf := &PolicyFile{}
	for id := range activePolicy.permissive {
		pf.Permissive = append(pf.Permissive, id)
	}
	for id := range activePolicy.copyleft {
		pf.Copyleft = append(pf.Copyleft, id)
	}
	return pf
}

// Categorize returns the compliance category for a given SPDX license identifier.
func Categorize(licenseID string) Category {
	id := strings.TrimSpace(licenseID)

	if id == "" || id == "NOASSERTION" || id == "NONE" {
		return CategoryUnknown
	}

	policyMu.RLock()
	p := activePolicy
	policyMu.RUnlock()

	// Exact match first.
	if p.permissive[id] {
		return CategoryPermissive
	}
	if p.copyleft[id] {
		return CategoryCopyleft
	}

	// Prefix match for SPDX modifiers (e.g. "MPL-2.0-no-copyleft-exception" → "MPL-2.0").
	for base := range p.permissive {
		if strings.HasPrefix(id, base+"-") {
			return CategoryPermissive
		}
	}
	for base := range p.copyleft {
		if strings.HasPrefix(id, base+"-") {
			return CategoryCopyleft
		}
	}

	return CategoryUnknown
}

// Result holds the compliance check result for one SBOM.
type Result struct {
	LicenseID            string
	Category             Category
	PackageCount         uint32
	NonCompliantPackages []string
	ExemptedPackages     []string // packages covered by exceptions
	ExemptionReason      string   // reason if entire license is exempted
}

// Check analyzes a list of packages and their licenses and produces compliance results.
// Uses no exceptions – all copyleft/unknown packages are flagged.
func Check(packageNames, packageLicenses []string) []Result {
	return CheckWithExceptions(packageNames, packageLicenses, nil)
}

// CheckWithExceptions analyzes packages/licenses with optional exception rules.
// Exempted packages are tracked separately and don't count as non-compliant.
func CheckWithExceptions(packageNames, packageLicenses []string, exceptions *ExceptionIndex) []Result {
	type licenseAgg struct {
		count         uint32
		packages      []string
		exempted      []string
		category      Category
		blanketExempt bool
		exemptionNote string
	}

	agg := make(map[string]*licenseAgg)

	for i, lic := range packageLicenses {
		name := ""
		if i < len(packageNames) {
			name = packageNames[i]
		}

		cat := Categorize(lic)

		entry, ok := agg[lic]
		if !ok {
			entry = &licenseAgg{category: cat}
			agg[lic] = entry

			// Check blanket exception for this license.
			if exceptions != nil {
				if exempt, reason := exceptions.IsExempt("", lic); exempt {
					entry.blanketExempt = true
					entry.exemptionNote = reason
				}
			}
		}
		entry.count++

		// Determine if this package is exempted or non-compliant.
		if entry.blanketExempt {
			// Blanket exception covers ALL packages for this license.
			entry.exempted = append(entry.exempted, name)
		} else if cat == CategoryPermissive {
			// Permissive licenses are always compliant – don't track packages.
		} else if exceptions != nil {
			// Copyleft or unknown: check per-package exception.
			if exempt, _ := exceptions.IsExempt(name, lic); exempt {
				entry.exempted = append(entry.exempted, name)
			} else {
				entry.packages = append(entry.packages, name)
			}
		} else {
			// No exceptions loaded: all copyleft/unknown packages are non-compliant.
			entry.packages = append(entry.packages, name)
		}
	}

	results := make([]Result, 0, len(agg))
	for licID, entry := range agg {
		results = append(results, Result{
			LicenseID:            licID,
			Category:             entry.category,
			PackageCount:         entry.count,
			NonCompliantPackages: entry.packages,
			ExemptedPackages:     entry.exempted,
			ExemptionReason:      entry.exemptionNote,
		})
	}

	return results
}
