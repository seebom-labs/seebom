package github

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"
)

const (
	githubAPIBase = "https://api.github.com"
	// Unauthenticated: 60 req/hour → ~1 req/min to be safe.
	defaultRate = 0.8
	// Authenticated: 5000 req/hour → ~1.3 req/s to be safe.
	authenticatedRate = 1.2
	defaultBurst      = 3
)

// licenseResponse is the GitHub API response for /repos/{owner}/{repo}/license.
type licenseResponse struct {
	License *licenseInfo `json:"license"`
}

type licenseInfo struct {
	SPDXID string `json:"spdx_id"`
	Name   string `json:"name"`
}

// repoResponse is the GitHub API response for /repos/{owner}/{repo}.
type repoResponse struct {
	Archived   bool         `json:"archived"`
	Fork       bool         `json:"fork"`
	PushedAt   string       `json:"pushed_at"`
	Stargazers int          `json:"stargazers_count"`
	License    *repoLicense `json:"license"`
}

type repoLicense struct {
	SPDXID string `json:"spdx_id"`
}

// RepoMetadata contains health indicators for a GitHub repository.
type RepoMetadata struct {
	Repo       string
	Archived   bool
	Fork       bool
	PushedAt   time.Time
	Stargazers int
	SPDXID     string
}

// knownLicenseOverrides maps GitHub repos (lowercase "owner/repo") to their correct
// SPDX license IDs. GitHub's license detection sometimes returns "Other" / NOASSERTION
// for repos with non-standard LICENSE file formats or dual-license setups.
// These have been verified manually.
var knownLicenseOverrides = map[string]string{
	"opencontainers/go-digest": "Apache-2.0",
	"shopspring/decimal":       "MIT",
	"go-yaml/yaml":             "Apache-2.0",
	"go-tomb/tomb":             "BSD-3-Clause",
	"go-inf/inf":               "BSD-3-Clause",
	"go-check/check":           "BSD-2-Clause",
}

// Resolver resolves unknown package licenses by querying the GitHub API.
type Resolver struct {
	token         string
	httpClient    *http.Client
	licenseCache  sync.Map // map[string]string (repo key → SPDX ID or "")
	metadataCache sync.Map // map[string]*RepoMetadata
	limiter       *tokenBucket
}

// NewResolver creates a new GitHub license resolver.
// If token is provided, the rate limit is significantly higher.
func NewResolver(token string) *Resolver {
	rate := defaultRate
	if token != "" {
		rate = authenticatedRate
	}
	return &Resolver{
		token: token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		limiter: newTokenBucket(rate, defaultBurst),
	}
}

// Resolve attempts to find the SPDX license ID for a package via the GitHub API.
// Returns the SPDX ID (e.g., "Apache-2.0") or empty string if not resolvable.
// Results are cached in-memory to avoid duplicate API calls.
func (r *Resolver) Resolve(ctx context.Context, purl string) string {
	owner, repo, ok := ExtractGitHubRepo(purl)
	if !ok {
		return ""
	}

	key := strings.ToLower(owner + "/" + repo)

	// Check in-memory cache.
	if cached, found := r.licenseCache.Load(key); found {
		return cached.(string)
	}

	// Rate-limit the request.
	if err := r.limiter.Wait(ctx); err != nil {
		return ""
	}

	spdxID := r.fetchLicense(ctx, owner, repo)

	// Fallback: use manually verified overrides for repos where GitHub
	// returns "Other" / NOASSERTION despite having a valid LICENSE file.
	if spdxID == "" {
		if override, ok := knownLicenseOverrides[key]; ok {
			spdxID = override
		}
	}

	r.licenseCache.Store(key, spdxID)

	if spdxID != "" {
		log.Printf("  GitHub license resolved: %s/%s → %s", owner, repo, spdxID)
	}

	return spdxID
}

// ResolveWithMetadata fetches license AND repo metadata (archived, fork, last push).
// Use this for comprehensive package health checks.
func (r *Resolver) ResolveWithMetadata(ctx context.Context, purl string) *RepoMetadata {
	owner, repo, ok := ExtractGitHubRepo(purl)
	if !ok {
		return nil
	}

	key := strings.ToLower(owner + "/" + repo)

	// Check in-memory cache.
	if cached, found := r.metadataCache.Load(key); found {
		return cached.(*RepoMetadata)
	}

	// Rate-limit the request.
	if err := r.limiter.Wait(ctx); err != nil {
		return nil
	}

	meta := r.fetchRepoMetadata(ctx, owner, repo)
	if meta != nil {
		r.metadataCache.Store(key, meta)
		r.licenseCache.Store(key, meta.SPDXID) // Also populate license cache

		if meta.Archived {
			log.Printf("  ⚠️  GitHub repo ARCHIVED: %s/%s", owner, repo)
		}
		if meta.SPDXID != "" {
			log.Printf("  GitHub license resolved: %s/%s → %s", owner, repo, meta.SPDXID)
		}
	} else {
		// Cache negative result
		r.metadataCache.Store(key, &RepoMetadata{Repo: key})
	}

	return meta
}

// PreloadCache loads known repo→license mappings (e.g., from ClickHouse).
func (r *Resolver) PreloadCache(entries map[string]string) {
	for k, v := range entries {
		r.licenseCache.Store(strings.ToLower(k), v)
	}
}

// PreloadMetadataCache loads repo metadata from ClickHouse.
func (r *Resolver) PreloadMetadataCache(entries []*RepoMetadata) {
	for _, m := range entries {
		r.metadataCache.Store(strings.ToLower(m.Repo), m)
		if m.SPDXID != "" {
			r.licenseCache.Store(strings.ToLower(m.Repo), m.SPDXID)
		}
	}
}

// CacheEntries returns all cached license entries (for persisting to ClickHouse).
func (r *Resolver) CacheEntries() map[string]string {
	entries := make(map[string]string)
	r.licenseCache.Range(func(key, value any) bool {
		entries[key.(string)] = value.(string)
		return true
	})
	return entries
}

// MetadataCacheEntries returns all cached metadata entries (for persisting to ClickHouse).
func (r *Resolver) MetadataCacheEntries() []*RepoMetadata {
	var entries []*RepoMetadata
	r.metadataCache.Range(func(key, value any) bool {
		if m, ok := value.(*RepoMetadata); ok && m != nil {
			entries = append(entries, m)
		}
		return true
	})
	return entries
}

func (r *Resolver) fetchLicense(ctx context.Context, owner, repo string) string {
	url := fmt.Sprintf("%s/repos/%s/%s/license", githubAPIBase, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Handle rate limiting.
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		r.handleRateLimit(resp)
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		// 404 = private repo or not found – cache empty to avoid retries.
		io.Copy(io.Discard, resp.Body)
		return ""
	}

	var lr licenseResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return ""
	}

	if lr.License == nil || lr.License.SPDXID == "" || lr.License.SPDXID == "NOASSERTION" {
		return ""
	}

	return lr.License.SPDXID
}

// fetchRepoMetadata gets full repo info including archived status.
func (r *Resolver) fetchRepoMetadata(ctx context.Context, owner, repo string) *RepoMetadata {
	url := fmt.Sprintf("%s/repos/%s/%s", githubAPIBase, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		r.handleRateLimit(resp)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil
	}

	var rr repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil
	}

	meta := &RepoMetadata{
		Repo:       strings.ToLower(owner + "/" + repo),
		Archived:   rr.Archived,
		Fork:       rr.Fork,
		Stargazers: rr.Stargazers,
	}

	// Parse pushed_at timestamp
	if rr.PushedAt != "" {
		if t, err := time.Parse(time.RFC3339, rr.PushedAt); err == nil {
			meta.PushedAt = t
		}
	}

	// Extract license from repo response.
	if rr.License != nil && rr.License.SPDXID != "" && rr.License.SPDXID != "NOASSERTION" {
		meta.SPDXID = rr.License.SPDXID
	}

	// Fallback: if the repo API didn't return a usable license, try the dedicated
	// /repos/{owner}/{repo}/license endpoint which does deeper file analysis.
	if meta.SPDXID == "" {
		if err := r.limiter.Wait(ctx); err == nil {
			if spdxID := r.fetchLicense(ctx, owner, repo); spdxID != "" {
				meta.SPDXID = spdxID
			}
		}
	}

	// Last resort: use manually verified overrides for repos where GitHub
	// returns "Other" / NOASSERTION despite having a valid LICENSE file.
	if meta.SPDXID == "" {
		key := strings.ToLower(owner + "/" + repo)
		if override, ok := knownLicenseOverrides[key]; ok {
			meta.SPDXID = override
		}
	}

	return meta
}

func (r *Resolver) handleRateLimit(resp *http.Response) {
	resetStr := resp.Header.Get("X-RateLimit-Reset")
	if resetStr != "" {
		resetUnix, err := strconv.ParseInt(resetStr, 10, 64)
		if err == nil {
			waitTime := time.Until(time.Unix(resetUnix, 0))
			if waitTime > 0 && waitTime < 60*time.Minute {
				log.Printf("  GitHub rate limited, waiting %v until reset", waitTime.Round(time.Second))
				time.Sleep(waitTime + time.Second)
				return
			}
		}
	}
	// Fallback: wait 60 seconds.
	log.Printf("  GitHub rate limited, waiting 60s")
	time.Sleep(60 * time.Second)
}

// tokenBucket is a simple rate limiter (same pattern as osv package).
type tokenBucket struct {
	rate       float64
	burst      int
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}

func (tb *tokenBucket) Wait(ctx context.Context) error {
	for {
		tb.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(tb.lastRefill).Seconds()
		tb.tokens += elapsed * tb.rate
		if tb.tokens > float64(tb.burst) {
			tb.tokens = float64(tb.burst)
		}
		tb.lastRefill = now

		if tb.tokens >= 1.0 {
			tb.tokens -= 1.0
			tb.mu.Unlock()
			return nil
		}
		wait := time.Duration((1.0 - tb.tokens) / tb.rate * float64(time.Second))
		tb.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}
