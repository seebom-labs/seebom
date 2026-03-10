package github

import "testing"

func TestExtractGitHubRepo(t *testing.T) {
	tests := []struct {
		name      string
		purl      string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{
			name:      "golang github.com package",
			purl:      "pkg:golang/github.com/chaos-mesh/k8s_dns_chaos@v0.0.0",
			wantOwner: "chaos-mesh",
			wantRepo:  "k8s_dns_chaos",
			wantOK:    true,
		},
		{
			name:      "golang with subpath",
			purl:      "pkg:golang/github.com/prometheus/client_golang/prometheus@v1.19.0",
			wantOwner: "prometheus",
			wantRepo:  "client_golang",
			wantOK:    true,
		},
		{
			name:      "pkg:github scheme",
			purl:      "pkg:github/kubernetes/kubernetes@v1.30.0",
			wantOwner: "kubernetes",
			wantRepo:  "kubernetes",
			wantOK:    true,
		},
		{
			name:      "non-github golang package",
			purl:      "pkg:golang/golang.org/x/crypto@v0.21.0",
			wantOwner: "",
			wantRepo:  "",
			wantOK:    false,
		},
		{
			name:      "npm package (not supported)",
			purl:      "pkg:npm/%40angular/core@17.0.0",
			wantOwner: "",
			wantRepo:  "",
			wantOK:    false,
		},
		{
			name:      "empty purl",
			purl:      "",
			wantOwner: "",
			wantRepo:  "",
			wantOK:    false,
		},
		{
			name:      "with qualifiers",
			purl:      "pkg:golang/github.com/gin-gonic/gin@v1.9.1?type=module",
			wantOwner: "gin-gonic",
			wantRepo:  "gin",
			wantOK:    true,
		},
		{
			name:      "with subpath fragment",
			purl:      "pkg:golang/github.com/hashicorp/go-hclog@v1.5.0#subdir",
			wantOwner: "hashicorp",
			wantRepo:  "go-hclog",
			wantOK:    true,
		},
		{
			name:      "missing repo",
			purl:      "pkg:golang/github.com/onlyowner",
			wantOwner: "",
			wantRepo:  "",
			wantOK:    false,
		},
		{
			name:      "azure go-autorest submodule",
			purl:      "pkg:golang/github.com/Azure/go-autorest/autorest@v0.11.29",
			wantOwner: "Azure",
			wantRepo:  "go-autorest",
			wantOK:    true,
		},
		{
			name:      "hamba avro v2",
			purl:      "pkg:golang/github.com/hamba/avro/v2@v2.31.0",
			wantOwner: "hamba",
			wantRepo:  "avro",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, ok := ExtractGitHubRepo(tt.purl)
			if ok != tt.wantOK {
				t.Errorf("ExtractGitHubRepo(%q) ok = %v, want %v", tt.purl, ok, tt.wantOK)
			}
			if owner != tt.wantOwner {
				t.Errorf("ExtractGitHubRepo(%q) owner = %q, want %q", tt.purl, owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("ExtractGitHubRepo(%q) repo = %q, want %q", tt.purl, repo, tt.wantRepo)
			}
		})
	}
}

func TestRepoKey(t *testing.T) {
	tests := []struct {
		owner string
		repo  string
		want  string
	}{
		{"Azure", "go-autorest", "azure/go-autorest"},
		{"OWNER", "REPO", "owner/repo"},
		{"", "repo", "/repo"},
		{"owner", "", "owner/"},
	}

	for _, tt := range tests {
		t.Run(tt.owner+"/"+tt.repo, func(t *testing.T) {
			got := RepoKey(tt.owner, tt.repo)
			if got != tt.want {
				t.Errorf("RepoKey(%q, %q) = %q, want %q", tt.owner, tt.repo, got, tt.want)
			}
		})
	}
}
