package license

import (
	"fmt"
	"os"
	"strings"

	json "github.com/goccy/go-json"
)

// ExceptionsFile represents the top-level license-exceptions.json structure,
// modeled after https://github.com/cncf/foundation/blob/main/license-exceptions/exceptions.json
type ExceptionsFile struct {
	Version           string             `json:"version"`
	LastUpdated       string             `json:"lastUpdated"`
	Description       string             `json:"description,omitempty"`
	BlanketExceptions []BlanketException `json:"blanketExceptions"`
	Exceptions        []Exception        `json:"exceptions"`
}

// BlanketException exempts an entire license from violations regardless of package.
type BlanketException struct {
	ID           string `json:"id"`
	License      string `json:"license"`
	Status       string `json:"status"` // approved, revoked
	ApprovedDate string `json:"approvedDate"`
	Scope        string `json:"scope,omitempty"`
	Comment      string `json:"comment,omitempty"`
}

// Exception exempts a specific package+license combination.
type Exception struct {
	ID           string `json:"id"`
	Package      string `json:"package"`           // package name or PURL pattern
	License      string `json:"license"`           // SPDX license ID
	Project      string `json:"project,omitempty"` // optional: restrict to specific project
	Status       string `json:"status"`            // approved, revoked
	ApprovedDate string `json:"approvedDate"`
	Scope        string `json:"scope,omitempty"`
	Results      string `json:"results,omitempty"` // link to approval discussion
	Comment      string `json:"comment,omitempty"`
}

// ExceptionIndex is a pre-computed lookup for fast exception matching.
type ExceptionIndex struct {
	// blanketLicenses are licenses globally exempted (exact match on SPDX ID).
	blanketLicenses map[string]*BlanketException
	// packageLicense maps "package\x00license" → Exception for specific package+license pairs.
	packageLicense map[string]*Exception
	// packageAny maps "package" → Exception for packages exempted regardless of license.
	packageAny map[string]*Exception

	Raw *ExceptionsFile
}

// LoadExceptions reads and indexes a license-exceptions.json file.
func LoadExceptions(path string) (*ExceptionIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read exceptions file %s: %w", path, err)
	}

	var ef ExceptionsFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("failed to parse exceptions file %s: %w", path, err)
	}

	return BuildIndex(&ef), nil
}

// LoadExceptionsWithFallback tries the primary path first, then falls back to
// additional paths. This allows the API Gateway to load exceptions from a
// ConfigMap first, and fall back to a downloaded file in the SBOM PVC.
func LoadExceptionsWithFallback(paths ...string) (*ExceptionIndex, error) {
	var lastErr error
	for _, p := range paths {
		if p == "" {
			continue
		}
		idx, err := LoadExceptions(p)
		if err != nil {
			lastErr = err
			continue
		}
		// Return the first file that loads successfully and has actual content
		if idx.Raw != nil && (len(idx.Raw.BlanketExceptions) > 0 || len(idx.Raw.Exceptions) > 0) {
			return idx, nil
		}
		// File loaded but is empty, keep looking
		lastErr = fmt.Errorf("exceptions file %s is empty", p)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no valid exceptions file paths provided")
	}
	return nil, lastErr
}

// BuildIndex creates an ExceptionIndex from an ExceptionsFile.
func BuildIndex(ef *ExceptionsFile) *ExceptionIndex {
	idx := &ExceptionIndex{
		blanketLicenses: make(map[string]*BlanketException),
		packageLicense:  make(map[string]*Exception),
		packageAny:      make(map[string]*Exception),
		Raw:             ef,
	}

	for i := range ef.BlanketExceptions {
		be := &ef.BlanketExceptions[i]
		if strings.EqualFold(be.Status, "approved") {
			for _, lic := range splitLicenses(be.License) {
				idx.blanketLicenses[lic] = be
			}
		}
	}

	for i := range ef.Exceptions {
		exc := &ef.Exceptions[i]
		if !strings.EqualFold(exc.Status, "approved") {
			continue
		}

		// CNCF exceptions with "All CNCF Projects" are effectively blanket
		// exceptions — they apply globally regardless of which SBOM uses them.
		if strings.EqualFold(exc.Project, "All CNCF Projects") {
			for _, lic := range splitLicenses(exc.License) {
				// Promote to blanket exception so IsExempt matches any package.
				idx.blanketLicenses[lic] = &BlanketException{
					ID:           exc.ID,
					License:      lic,
					Status:       exc.Status,
					ApprovedDate: exc.ApprovedDate,
					Scope:        exc.Scope,
					Comment:      exc.Comment,
				}
			}
			continue
		}

		if exc.License != "" && exc.Package != "" {
			for _, lic := range splitLicenses(exc.License) {
				key := exc.Package + "\x00" + lic
				idx.packageLicense[key] = exc
			}
		} else if exc.Package != "" {
			idx.packageAny[exc.Package] = exc
		}
	}

	return idx
}

// splitLicenses splits compound license expressions into individual SPDX IDs.
// Handles comma-separated ("GPL-2.0-only, GPL-2.0-or-later"),
// OR-separated ("MPL-2.0 OR LGPL-3.0-or-later"),
// and AND-separated ("MPL-2.0 AND BSD-3-Clause") expressions.
func splitLicenses(expr string) []string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}

	// Try comma-separated first (e.g. "GPL-2.0-only, GPL-2.0-or-later").
	if strings.Contains(expr, ",") {
		parts := strings.Split(expr, ",")
		var result []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	// Try SPDX operators: " OR " and " AND ".
	for _, sep := range []string{" OR ", " AND "} {
		if strings.Contains(expr, sep) {
			parts := strings.Split(expr, sep)
			var result []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					result = append(result, p)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}

	return []string{expr}
}

// IsExempt checks if a package+license combination is covered by an exception.
// Returns the matching exception reason or empty string if not exempt.
func (idx *ExceptionIndex) IsExempt(packageName, licenseID string) (exempt bool, reason string) {
	if idx == nil {
		return false, ""
	}

	// 1. Check blanket license exceptions (exact match first).
	if be, ok := idx.blanketLicenses[licenseID]; ok {
		return true, fmt.Sprintf("Blanket exception: %s – %s", be.ID, be.Comment)
	}

	// 1b. Check blanket license exceptions (prefix match for SPDX modifiers).
	// e.g. "MPL-2.0-no-copyleft-exception" should match blanket "MPL-2.0".
	for baseLicense, be := range idx.blanketLicenses {
		if strings.HasPrefix(licenseID, baseLicense+"-") {
			return true, fmt.Sprintf("Blanket exception: %s (via %s) – %s", be.ID, baseLicense, be.Comment)
		}
	}

	// 2. Check specific package+license (exact match).
	key := packageName + "\x00" + licenseID
	if exc, ok := idx.packageLicense[key]; ok {
		return true, fmt.Sprintf("Exception: %s – %s", exc.ID, exc.Comment)
	}

	// 2b. Check package+license with substring matching on package name.
	// CNCF exceptions use short names like "cyphar/filepath-securejoin" but
	// SBOM packages have full names like "github.com/cyphar/filepath-securejoin".
	lowerPkg := strings.ToLower(packageName)
	for compoundKey, exc := range idx.packageLicense {
		parts := strings.SplitN(compoundKey, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		excPkg, excLic := parts[0], parts[1]
		if excLic == licenseID && strings.Contains(lowerPkg, strings.ToLower(excPkg)) {
			return true, fmt.Sprintf("Exception: %s – %s", exc.ID, exc.Comment)
		}
	}

	// 3. Check package-only exceptions (any license, exact match).
	if exc, ok := idx.packageAny[packageName]; ok {
		return true, fmt.Sprintf("Exception: %s – %s", exc.ID, exc.Comment)
	}

	// 3b. Package-only exceptions with substring matching.
	for excPkg, exc := range idx.packageAny {
		if strings.Contains(lowerPkg, strings.ToLower(excPkg)) {
			return true, fmt.Sprintf("Exception: %s – %s", exc.ID, exc.Comment)
		}
	}

	return false, ""
}
