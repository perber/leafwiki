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

// BuildSitemap walks the page tree and returns a sitemap.xml document listing
// every page (sections are structural only and are not listed themselves).
// baseURL must be an absolute origin with no trailing slash, e.g.
// "https://wiki.example.com" or "https://wiki.example.com/wiki" when served
// behind a base path.
func BuildSitemap(root *tree.PageNode, baseURL string) ([]byte, error) {
	baseURL = strings.TrimRight(baseURL, "/")

	set := urlSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	var walk func(*tree.PageNode)
	walk = func(node *tree.PageNode) {
		if node.Kind == tree.NodeKindPage {
			p := strings.TrimPrefix(node.CalculatePath(), "/")
			entry := sitemapURL{Loc: baseURL + "/" + p}
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
