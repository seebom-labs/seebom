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
			idx.blanketLicenses[be.License] = be
		}
	}

	for i := range ef.Exceptions {
		exc := &ef.Exceptions[i]
		if !strings.EqualFold(exc.Status, "approved") {
			continue
		}
		if exc.License != "" && exc.Package != "" {
			key := exc.Package + "\x00" + exc.License
			idx.packageLicense[key] = exc
		} else if exc.Package != "" {
			idx.packageAny[exc.Package] = exc
		}
	}

	return idx
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

	// 2. Check specific package+license.
	key := packageName + "\x00" + licenseID
	if exc, ok := idx.packageLicense[key]; ok {
		return true, fmt.Sprintf("Exception: %s – %s", exc.ID, exc.Comment)
	}

	// 3. Check package-only exceptions (any license).
	if exc, ok := idx.packageAny[packageName]; ok {
		return true, fmt.Sprintf("Exception: %s – %s", exc.ID, exc.Comment)
	}

	return false, ""
}
