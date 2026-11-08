package seo

import (
	"context"
	"errors"
)

// Page is one page as the use cases of this package read it, shared by
// every use case that reads a single page by its route path.
type Page struct {
	Title string
	// Body is the page's Markdown without frontmatter.
	Body string
	// Properties are the page's frontmatter properties (trimmed keys, case kept).
	Properties map[string]string
}

// PageReader reads a page by its route path. It returns ErrPageNotFound when
// there is no page at the path, and nil and no error when it can't tell (tree
// not loaded yet). The page meta and the Markdown use cases share it because
// they need the same read.
type PageReader interface {
	PageByRoutePath(ctx context.Context, routePath string) (*Page, error)
}

// ErrPageNotFound is returned by PageReader and PermalinkResolver when no
// page is at the route path or has the ID. It is never shown to a visitor:
// the use cases turn it into a result, so it is no LocalizedError.
var ErrPageNotFound = errors.New("seo: page not found")
