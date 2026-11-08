package seo

import "context"

// Page is one page as the use cases of this package read it, shared by
// every use case that reads a single page by its route path.
type Page struct {
	Title string
	// Body is the page's Markdown without frontmatter.
	Body string
	// Properties are the page's frontmatter properties (trimmed keys, case kept).
	Properties map[string]string
}

// PageReader reads a page by its route path. It returns nil and no error when
// there is no such page (unknown path, tree not loaded yet). The page meta and
// the Markdown use cases share it because they need the same read.
type PageReader interface {
	PageByRoutePath(ctx context.Context, routePath string) (*Page, error)
}
