// Package github extracts GitHub owner/repo from Package URLs (PURLs).
package github

import (
	"strings"
)

// ExtractGitHubRepo extracts the GitHub owner and repo name from a PURL.
// Supports:
//   - pkg:golang/github.com/{owner}/{repo}[/subpath][@version]
//   - pkg:github/{owner}/{repo}[@version]
//
// Returns empty strings and false if the PURL is not a recognizable GitHub package.
func ExtractGitHubRepo(purl string) (owner, repo string, ok bool) {
	if purl == "" {
		return "", "", false
	}

	// Strip version qualifier.
	if idx := strings.Index(purl, "@"); idx > 0 {
		purl = purl[:idx]
	}

	// Strip qualifiers (?...) and subpath (#...).
	if idx := strings.Index(purl, "?"); idx > 0 {
		purl = purl[:idx]
	}
	if idx := strings.Index(purl, "#"); idx > 0 {
		purl = purl[:idx]
	}

	// pkg:golang/github.com/{owner}/{repo}[/subpath...]
	if strings.HasPrefix(purl, "pkg:golang/github.com/") {
		rest := strings.TrimPrefix(purl, "pkg:golang/github.com/")
		return splitOwnerRepo(rest)
	}

	// pkg:github/{owner}/{repo}
	if strings.HasPrefix(purl, "pkg:github/") {
		rest := strings.TrimPrefix(purl, "pkg:github/")
		return splitOwnerRepo(rest)
	}

	return "", "", false
}

// splitOwnerRepo splits "owner/repo[/subpath...]" into owner and repo.
func splitOwnerRepo(s string) (string, string, bool) {
	parts := strings.SplitN(s, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// RepoKey returns the lowercase "owner/repo" key for cache lookups.
func RepoKey(owner, repo string) string {
	return strings.ToLower(owner + "/" + repo)
}
