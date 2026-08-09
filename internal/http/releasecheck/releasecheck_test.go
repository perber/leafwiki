package releasecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchLatestRelease_ParsesGitHubPayload(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if got := r.Header.Get("Accept"); !strings.Contains(got, "application/vnd.github") {
			t.Errorf("expected GitHub Accept header, got %q", got)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v0.12.0",
			"name": "LeafWiki v0.12.0",
			"html_url": "https://github.com/perber/leafwiki/releases/tag/v0.12.0",
			"published_at": "2026-07-26T15:50:41Z"
		}`))
	}))
	t.Cleanup(server.Close)

	got, err := FetchLatestRelease(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("FetchLatestRelease: %v", err)
	}
	if got.TagName != "v0.12.0" {
		t.Fatalf("tagName=%q, want v0.12.0", got.TagName)
	}
	if got.Name != "LeafWiki v0.12.0" {
		t.Fatalf("name=%q", got.Name)
	}
	if !strings.Contains(got.HTMLURL, "/releases/tag/v0.12.0") {
		t.Fatalf("htmlUrl=%q", got.HTMLURL)
	}
	if got.PublishedAt != "2026-07-26T15:50:41Z" {
		t.Fatalf("publishedAt=%q", got.PublishedAt)
	}
}

func TestFetchLatestRelease_NonOKStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	_, err := FetchLatestRelease(context.Background(), server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected status in error, got %v", err)
	}
}

func TestFetchLatestRelease_MissingTagName(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"no tag"}`))
	}))
	t.Cleanup(server.Close)

	_, err := FetchLatestRelease(context.Background(), server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error for missing tag_name")
	}
}
