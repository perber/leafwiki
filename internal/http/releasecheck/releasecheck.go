package releasecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultLatestReleaseURL is the GitHub Releases API for LeafWiki's latest tag.
const DefaultLatestReleaseURL = "https://api.github.com/repos/perber/leafwiki/releases/latest"

// LatestRelease is the subset of GitHub release fields LeafWiki exposes to clients.
type LatestRelease struct {
	TagName     string `json:"tagName"`
	Name        string `json:"name"`
	HTMLURL     string `json:"htmlUrl"`
	PublishedAt string `json:"publishedAt"`
}

type githubReleaseResponse struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

// FetchLatestRelease loads the latest published release from the GitHub API.
// apiURL defaults to DefaultLatestReleaseURL when empty. client defaults to a
// short-timeout http.Client when nil.
func FetchLatestRelease(ctx context.Context, client *http.Client, apiURL string) (*LatestRelease, error) {
	if apiURL == "" {
		apiURL = DefaultLatestReleaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LeafWiki-UpdateCheck")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read latest release response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw githubReleaseResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode latest release response: %w", err)
	}
	if strings.TrimSpace(raw.TagName) == "" {
		return nil, fmt.Errorf("github releases API response missing tag_name")
	}

	return &LatestRelease{
		TagName:     raw.TagName,
		Name:        raw.Name,
		HTMLURL:     raw.HTMLURL,
		PublishedAt: raw.PublishedAt,
	}, nil
}
