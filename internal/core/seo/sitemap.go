package seo

import (
	"encoding/xml"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/tree"
)

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type urlSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// HomeRoutePath returns the route path of the page the site root ("/") shows.
// The root renders the tree's first child, so when that child is a page it is
// reachable under two URLs - "/" and "/<path>" - and both must point search
// engines at a single canonical URL, the root.
//
// Returns "" when the first child is a section or the tree has no pages; the
// root then has no page of its own and is left without SSR content or a
// canonical, exactly as before.
func HomeRoutePath(root *tree.PageNode) string {
	if root == nil || len(root.Children) == 0 {
		return ""
	}
	first := root.Children[0]
	if first.Kind != tree.NodeKindPage {
		return ""
	}
	return strings.Trim(first.CalculatePath(), "/")
}

// BuildSitemap walks the page tree and returns a sitemap.xml document listing
// every page (sections are structural only and are not listed themselves).
// baseURL must be an absolute origin with no trailing slash, e.g.
// "https://wiki.example.com" or "https://wiki.example.com/wiki" when served
// behind a base path.
//
// The home page is listed at the bare root URL rather than under its own path,
// so the sitemap agrees with the canonical tag served for both URLs.
func BuildSitemap(root *tree.PageNode, baseURL string) ([]byte, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	home := HomeRoutePath(root)

	set := urlSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	var walk func(*tree.PageNode)
	walk = func(node *tree.PageNode) {
		if node.Kind == tree.NodeKindPage {
			p := strings.TrimPrefix(node.CalculatePath(), "/")
			loc := baseURL + "/" + p
			if home != "" && p == home {
				loc = baseURL + "/"
			}
			entry := sitemapURL{Loc: loc}
			if !node.Metadata.UpdatedAt.IsZero() {
				entry.LastMod = node.Metadata.UpdatedAt.UTC().Format(time.RFC3339)
			}
			set.URLs = append(set.URLs, entry)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	if root != nil {
		for _, child := range root.Children {
			walk(child)
		}
	}

	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
