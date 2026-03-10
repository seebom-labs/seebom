package osv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryBatch_EmptyPURLs(t *testing.T) {
	client := NewClient()
	resp, err := client.QueryBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestQueryBatch_MockServer(t *testing.T) {
	// Create a mock OSV server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != BatchEndpoint {
			t.Errorf("expected path %s, got %s", BatchEndpoint, r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"vulns": [
						{
							"id": "CVE-2024-1234",
							"summary": "Test vulnerability",
							"severity": [{"type": "CVSS_V3", "score": "7.5"}]
						}
					]
				}
			]
		}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
		limiter:    getGlobalLimiter(),
	}

	resp, err := client.QueryBatch(context.Background(), []string{"pkg:npm/test@1.0.0"})
	if err != nil {
		t.Fatalf("QueryBatch() returned error: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if len(resp.Results[0].Vulns) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(resp.Results[0].Vulns))
	}
	if resp.Results[0].Vulns[0].ID != "CVE-2024-1234" {
		t.Errorf("expected CVE-2024-1234, got %s", resp.Results[0].Vulns[0].ID)
	}
}

func TestQueryBatch_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
		limiter:    getGlobalLimiter(),
	}

	_, err := client.QueryBatch(context.Background(), []string{"pkg:npm/test@1.0.0"})
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

func TestQueryBatch_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response that should be cancelled.
		<-r.Context().Done()
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
		limiter:    getGlobalLimiter(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.QueryBatch(ctx, []string{"pkg:npm/test@1.0.0"})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestQueryBatch_NoVulns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results": [{"vulns": []}]}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
		limiter:    getGlobalLimiter(),
	}

	resp, err := client.QueryBatch(context.Background(), []string{"pkg:npm/safe-package@1.0.0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if len(resp.Results[0].Vulns) != 0 {
		t.Errorf("expected 0 vulns for safe package, got %d", len(resp.Results[0].Vulns))
	}
}
