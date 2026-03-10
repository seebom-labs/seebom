package license

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCategorize(t *testing.T) {
	tests := []struct {
		input    string
		expected Category
	}{
		{"MIT", CategoryPermissive},
		{"Apache-2.0", CategoryPermissive},
		{"BSD-3-Clause", CategoryPermissive},
		{"GPL-3.0-only", CategoryCopyleft},
		{"AGPL-3.0-or-later", CategoryCopyleft},
		{"MPL-2.0", CategoryCopyleft},
		{"MPL-2.0-no-copyleft-exception", CategoryCopyleft},
		{"Apache-2.0-with-LLVM-exception", CategoryPermissive},
		{"NOASSERTION", CategoryUnknown},
		{"NONE", CategoryUnknown},
		{"", CategoryUnknown},
		{"SomeWeirdLicense", CategoryUnknown},
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

func TestCheck(t *testing.T) {
	names := []string{"pkg-a", "pkg-b", "pkg-c", "pkg-d"}
	licenses := []string{"MIT", "GPL-3.0-only", "MIT", "NOASSERTION"}

	results := Check(names, licenses)

	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}

	// Build a map for easier assertion.
	byLicense := make(map[string]Result)
	for _, r := range results {
		byLicense[r.LicenseID] = r
	}

	mitResult, ok := byLicense["MIT"]
	if !ok {
		t.Fatal("expected MIT result")
	}
	if mitResult.PackageCount != 2 {
		t.Errorf("expected 2 MIT packages, got %d", mitResult.PackageCount)
	}
	if mitResult.Category != CategoryPermissive {
		t.Errorf("expected permissive, got %s", mitResult.Category)
	}
	// Permissive licenses should never have non-compliant packages.
	if len(mitResult.NonCompliantPackages) != 0 {
		t.Errorf("expected 0 non-compliant packages for permissive MIT, got %v", mitResult.NonCompliantPackages)
	}

	gplResult, ok := byLicense["GPL-3.0-only"]
	if !ok {
		t.Fatal("expected GPL result")
	}
	if gplResult.PackageCount != 1 {
		t.Errorf("expected 1 GPL package, got %d", gplResult.PackageCount)
	}
	if len(gplResult.NonCompliantPackages) != 1 || gplResult.NonCompliantPackages[0] != "pkg-b" {
		t.Errorf("expected [pkg-b] non-compliant, got %v", gplResult.NonCompliantPackages)
	}
}

func TestCheckWithExceptions_BlanketExempt(t *testing.T) {
	names := []string{"pkg-a", "pkg-b", "pkg-c"}
	licenses := []string{"MIT", "MPL-2.0", "MPL-2.0"}

	ef := &ExceptionsFile{
		BlanketExceptions: []BlanketException{
			{ID: "blanket-mpl", License: "MPL-2.0", Status: "approved", Comment: "MPL ok"},
		},
	}
	idx := BuildIndex(ef)

	results := CheckWithExceptions(names, licenses, idx)
	byLicense := make(map[string]Result)
	for _, r := range results {
		byLicense[r.LicenseID] = r
	}

	mpl := byLicense["MPL-2.0"]
	if mpl.ExemptionReason == "" {
		t.Error("expected blanket exemption reason for MPL-2.0")
	}
	if mpl.PackageCount != 2 {
		t.Errorf("expected 2 MPL packages, got %d", mpl.PackageCount)
	}
	// Blanket exempt: all packages should be in ExemptedPackages, NOT NonCompliantPackages.
	if len(mpl.ExemptedPackages) != 2 {
		t.Errorf("expected 2 exempted packages, got %v", mpl.ExemptedPackages)
	}
	if len(mpl.NonCompliantPackages) != 0 {
		t.Errorf("expected 0 non-compliant packages for blanket-exempted license, got %v", mpl.NonCompliantPackages)
	}
}

func TestCheckWithExceptions_BlanketPrefixMatch(t *testing.T) {
	// MPL-2.0-no-copyleft-exception should be covered by a blanket exception for MPL-2.0.
	names := []string{"pkg-slug"}
	licenses := []string{"MPL-2.0-no-copyleft-exception"}

	ef := &ExceptionsFile{
		BlanketExceptions: []BlanketException{
			{ID: "blanket-mpl", License: "MPL-2.0", Status: "approved", Comment: "MPL ok"},
		},
	}
	idx := BuildIndex(ef)

	results := CheckWithExceptions(names, licenses, idx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.ExemptionReason == "" {
		t.Error("expected blanket exemption reason for MPL-2.0-no-copyleft-exception via prefix match")
	}
	if len(r.ExemptedPackages) != 1 || r.ExemptedPackages[0] != "pkg-slug" {
		t.Errorf("expected pkg-slug exempted, got %v", r.ExemptedPackages)
	}
	if len(r.NonCompliantPackages) != 0 {
		t.Errorf("expected 0 non-compliant, got %v", r.NonCompliantPackages)
	}
}

func TestCheckWithExceptions_PackageExempt(t *testing.T) {
	names := []string{"pkg-a", "pkg-b"}
	licenses := []string{"GPL-3.0-only", "GPL-3.0-only"}

	ef := &ExceptionsFile{
		Exceptions: []Exception{
			{ID: "exc-1", Package: "pkg-a", License: "GPL-3.0-only", Status: "approved"},
		},
	}
	idx := BuildIndex(ef)

	results := CheckWithExceptions(names, licenses, idx)
	byLicense := make(map[string]Result)
	for _, r := range results {
		byLicense[r.LicenseID] = r
	}

	gpl := byLicense["GPL-3.0-only"]
	if len(gpl.ExemptedPackages) != 1 || gpl.ExemptedPackages[0] != "pkg-a" {
		t.Errorf("expected pkg-a exempted, got %v", gpl.ExemptedPackages)
	}
	if len(gpl.NonCompliantPackages) != 1 || gpl.NonCompliantPackages[0] != "pkg-b" {
		t.Errorf("expected only pkg-b non-compliant, got %v", gpl.NonCompliantPackages)
	}
}

func TestLoadPolicy(t *testing.T) {
	policyJSON := `{
		"permissive": ["MIT", "Apache-2.0", "BSD-3-Clause"],
		"copyleft": ["GPL-3.0-only", "AGPL-3.0-only"]
	}`

	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "license-policy.json")
	if err := os.WriteFile(policyPath, []byte(policyJSON), 0644); err != nil {
		t.Fatal(err)
	}

	permCount, copyleftCount, err := LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("LoadPolicy() error: %v", err)
	}
	if permCount != 3 {
		t.Errorf("expected 3 permissive, got %d", permCount)
	}
	if copyleftCount != 2 {
		t.Errorf("expected 2 copyleft, got %d", copyleftCount)
	}

	// After loading, Categorize should reflect the new policy.
	if got := Categorize("MIT"); got != CategoryPermissive {
		t.Errorf("expected MIT=permissive, got %s", got)
	}
	if got := Categorize("GPL-3.0-only"); got != CategoryCopyleft {
		t.Errorf("expected GPL-3.0-only=copyleft, got %s", got)
	}
	// Not listed = unknown
	if got := Categorize("WTFPL"); got != CategoryUnknown {
		t.Errorf("expected WTFPL=unknown, got %s", got)
	}
}

func TestLoadPolicy_MissingFile(t *testing.T) {
	_, _, err := LoadPolicy("/nonexistent/policy.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestGetPolicy(t *testing.T) {
	p := GetPolicy()
	if p == nil {
		t.Fatal("GetPolicy() returned nil")
	}
	// Should have at least some default entries.
	if len(p.Permissive) == 0 && len(p.Copyleft) == 0 {
		t.Error("expected non-empty policy")
	}
}

func TestLoadExceptions(t *testing.T) {
	exceptionsJSON := `{
		"version": "1.0.0",
		"blanketExceptions": [
			{"id": "b1", "license": "MPL-2.0", "status": "approved", "comment": "ok"}
		],
		"exceptions": [
			{"id": "e1", "package": "github.com/foo/bar", "license": "GPL-3.0-only", "status": "approved"}
		]
	}`

	tmpDir := t.TempDir()
	excPath := filepath.Join(tmpDir, "exceptions.json")
	if err := os.WriteFile(excPath, []byte(exceptionsJSON), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := LoadExceptions(excPath)
	if err != nil {
		t.Fatalf("LoadExceptions() error: %v", err)
	}
	if idx == nil {
		t.Fatal("expected non-nil index")
	}
	if idx.Raw == nil {
		t.Fatal("expected non-nil Raw exceptions file")
	}
	if len(idx.Raw.BlanketExceptions) != 1 {
		t.Errorf("expected 1 blanket exception, got %d", len(idx.Raw.BlanketExceptions))
	}

	// Verify the blanket exception works via CheckWithExceptions.
	results := CheckWithExceptions(
		[]string{"test-pkg"}, []string{"MPL-2.0"}, idx,
	)
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].ExemptionReason == "" {
		t.Error("expected blanket exemption to apply for MPL-2.0")
	}
}

func TestLoadExceptions_MissingFile(t *testing.T) {
	_, err := LoadExceptions("/nonexistent/exceptions.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBuildIndex_Empty(t *testing.T) {
	idx := BuildIndex(&ExceptionsFile{})
	if idx == nil {
		t.Fatal("expected non-nil index for empty file")
	}
	// Verify no exemptions apply.
	results := CheckWithExceptions(
		[]string{"pkg"}, []string{"GPL-3.0-only"}, idx,
	)
	for _, r := range results {
		if r.ExemptionReason != "" {
			t.Errorf("expected no exemption, got %q", r.ExemptionReason)
		}
	}
}

func TestCheck_EmptyInput(t *testing.T) {
	results := Check(nil, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(results))
	}
}

func TestCheck_MismatchedLengths(t *testing.T) {
	// If names and licenses have different lengths, should not panic.
	results := Check([]string{"pkg-a"}, []string{})
	// Should handle gracefully (empty or partial results).
	_ = results
}
