package seo_test

import (
	"context"
	"testing"

	"github.com/perber/wiki/internal/wiki/seo"
)

func TestGetRobotsUseCase_PublicWiki_AllowsCrawling(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		basePath  string
		want      string
	}{
		{name: "without public url: no sitemap line", want: "User-agent: *\nAllow: /\n"},
		{
			name:      "with public url",
			publicURL: "https://wiki.example.com",
			want:      "User-agent: *\nAllow: /\n\nSitemap: https://wiki.example.com/sitemap.xml\n",
		},
		{
			name:      "with public url and base path",
			publicURL: "https://wiki.example.com/wiki/",
			basePath:  "/wiki",
			want:      "User-agent: *\nAllow: /\n\nSitemap: https://wiki.example.com/wiki/sitemap.xml\n",
		},
		{name: "unusable public url: no sitemap line", publicURL: "not a url", want: "User-agent: *\nAllow: /\n"},
		{
			name:      "query, fragment and credentials of the public url are dropped",
			publicURL: "https://admin:s3cret@wiki.example.com/?utm=1#top",
			want:      "User-agent: *\nAllow: /\n\nSitemap: https://wiki.example.com/sitemap.xml\n",
		},
		{
			name:      "base path with trailing slash",
			publicURL: "https://wiki.example.com/wiki",
			basePath:  "/wiki/",
			want:      "User-agent: *\nAllow: /\n\nSitemap: https://wiki.example.com/wiki/sitemap.xml\n",
		},
		{
			// A newline in --public-url must not inject robots.txt directives.
			name:      "newline in public url injects no directive",
			publicURL: "https://wiki.example.com\nDisallow: /",
			want:      "User-agent: *\nAllow: /\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := seo.NewGetRobotsUseCase(tc.publicURL).Execute(context.Background(), seo.GetRobotsInput{BasePath: tc.basePath})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if out.Text != tc.want {
				t.Errorf("Text = %q, want %q", out.Text, tc.want)
			}
		})
	}
}

func TestPrivateRobots_DisallowsEverything(t *testing.T) {
	if want := "User-agent: *\nDisallow: /\n"; seo.PrivateRobots != want {
		t.Errorf("PrivateRobots = %q, want %q", seo.PrivateRobots, want)
	}
}
