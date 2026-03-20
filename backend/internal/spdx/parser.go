package spdx

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	json "github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/seebom-labs/seebom/backend/pkg/models"
)

// goTempModuleRe matches Go temporary module directory names like "tmp.ej9m9OiO2V".
// These appear in SPDX data when Go SBOM tools capture the temp build directory as a package name.
var goTempModuleRe = regexp.MustCompile(`^tmp\.[a-zA-Z0-9]{6,}$`)

// SPDXDocument represents the top-level structure of an SPDX 2.3 JSON document.
// We only extract the fields we need for ingestion.
type SPDXDocument struct {
	SPDXVersion       string             `json:"spdxVersion"`
	Name              string             `json:"name"`
	DocumentNamespace string             `json:"documentNamespace"`
	CreationInfo      SPDXCreationInfo   `json:"creationInfo"`
	Packages          []SPDXPackage      `json:"packages"`
	Relationships     []SPDXRelationship `json:"relationships"`
}

// SPDXCreationInfo holds creation metadata.
type SPDXCreationInfo struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

// SPDXPackage represents a single package entry in SPDX.
type SPDXPackage struct {
	SPDXID           string            `json:"SPDXID"`
	Name             string            `json:"name"`
	VersionInfo      string            `json:"versionInfo"`
	ExternalRefs     []SPDXExternalRef `json:"externalRefs"`
	LicenseConcluded string            `json:"licenseConcluded"`
	LicenseDeclared  string            `json:"licenseDeclared"`
}

// SPDXExternalRef holds external reference data (e.g., purl).
type SPDXExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

// SPDXRelationship represents a relationship between two SPDX elements.
type SPDXRelationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

// ParseResult contains the extracted data from an SPDX document, ready for ClickHouse insertion.
type ParseResult struct {
	SBOM     models.SBOM
	Packages models.SBOMPackages
}

// ParseFile opens and parses an SPDX JSON file using high-performance streaming.
func ParseFile(path, sourceFile, sha256Hash string) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SPDX file %s: %w", path, err)
	}
	defer f.Close()

	return Parse(f, sourceFile, sha256Hash)
}

// cleanPackageName replaces Go temporary build directory names (e.g. "tmp.ej9m9OiO2V")
// with a meaningful name derived from the PURL or SPDX ID.
// These temp names are artifacts of Go CI/CD builds captured by SBOM generators.
func cleanPackageName(name, purl, spdxID string) string {
	if !goTempModuleRe.MatchString(name) {
		return name
	}

	// Try to derive a useful name from the PURL (e.g. "pkg:golang/github.com/foo/bar@v1.0" → "github.com/foo/bar").
	if purl != "" {
		// Strip "pkg:<type>/" prefix.
		if idx := strings.Index(purl, "/"); idx >= 0 {
			cleaned := purl[idx+1:]
			// Strip version suffix (@...).
			if atIdx := strings.LastIndex(cleaned, "@"); atIdx >= 0 {
				cleaned = cleaned[:atIdx]
			}
			if cleaned != "" {
				return cleaned
			}
		}
	}

	// Fall back to SPDX ID (e.g. "SPDXRef-Package-foo" → "foo").
	if strings.HasPrefix(spdxID, "SPDXRef-Package-") {
		return strings.TrimPrefix(spdxID, "SPDXRef-Package-")
	}
	if strings.HasPrefix(spdxID, "SPDXRef-") {
		return strings.TrimPrefix(spdxID, "SPDXRef-")
	}

	return name
}

// Parse reads an SPDX JSON document from a reader and extracts models.
func Parse(r io.Reader, sourceFile, sha256Hash string) (*ParseResult, error) {
	var doc SPDXDocument

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to decode SPDX JSON: %w", err)
	}

	sbomID := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(sha256Hash))
	now := time.Now()

	// Parse creation date.
	creationDate, err := time.Parse(time.RFC3339, doc.CreationInfo.Created)
	if err != nil {
		// Fall back to current time if creation date is malformed.
		creationDate = now
	}

	// Extract tools from creators (filter "Tool:" prefix).
	var tools []string
	for _, c := range doc.CreationInfo.Creators {
		tools = append(tools, c)
	}

	sbom := models.SBOM{
		IngestedAt:        now,
		SBOMID:            sbomID,
		SourceFile:        sourceFile,
		SPDXVersion:       doc.SPDXVersion,
		DocumentName:      doc.Name,
		DocumentNamespace: doc.DocumentNamespace,
		SHA256Hash:        sha256Hash,
		CreationDate:      creationDate,
		CreatorTools:      tools,
	}

	// Build parallel arrays from packages.
	spdxIDToIndex := make(map[string]uint32, len(doc.Packages))

	var (
		spdxIDs  []string
		names    []string
		versions []string
		purls    []string
		licenses []string
	)

	for i, pkg := range doc.Packages {
		idx := uint32(i)
		spdxIDToIndex[pkg.SPDXID] = idx

		spdxIDs = append(spdxIDs, pkg.SPDXID)

		// Find PURL from external references (needed before name cleaning).
		purl := ""
		for _, ref := range pkg.ExternalRefs {
			if ref.ReferenceType == "purl" {
				purl = ref.ReferenceLocator
				break
			}
		}
		purls = append(purls, purl)

		// Clean up Go temp build directory names (e.g. "tmp.ej9m9OiO2V").
		name := cleanPackageName(pkg.Name, purl, pkg.SPDXID)
		names = append(names, name)
		versions = append(versions, pkg.VersionInfo)

		// Prefer declared license, fall back to concluded.
		lic := pkg.LicenseDeclared
		if lic == "" || lic == "NOASSERTION" {
			lic = pkg.LicenseConcluded
		}
		licenses = append(licenses, lic)
	}

	// Build relationship arrays.
	var (
		relSources []uint32
		relTargets []uint32
		relTypes   []string
	)

	for _, rel := range doc.Relationships {
		srcIdx, srcOK := spdxIDToIndex[rel.SPDXElementID]
		tgtIdx, tgtOK := spdxIDToIndex[rel.RelatedSPDXElement]
		if srcOK && tgtOK {
			relSources = append(relSources, srcIdx)
			relTargets = append(relTargets, tgtIdx)
			relTypes = append(relTypes, rel.RelationshipType)
		}
	}

	packages := models.SBOMPackages{
		IngestedAt:       now,
		SBOMID:           sbomID,
		SourceFile:       sourceFile,
		PackageSPDXIDs:   spdxIDs,
		PackageNames:     names,
		PackageVersions:  versions,
		PackagePURLs:     purls,
		PackageLicenses:  licenses,
		RelSourceIndices: relSources,
		RelTargetIndices: relTargets,
		RelTypes:         relTypes,
	}

	return &ParseResult{
		SBOM:     sbom,
		Packages: packages,
	}, nil
}
