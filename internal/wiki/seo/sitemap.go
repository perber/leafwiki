package seo

import (
	"context"
	"encoding/xml"
	"io"
	"time"
)

// MaxSitemapURLs is the most URLs one sitemap file may hold (sitemaps.org).
// Larger wikis get the first MaxSitemapURLs routes in navigation order.
const MaxSitemapURLs = 50_000

// PageRoute is a page or section as the sitemap needs it.
type PageRoute struct {
	// RoutePath is the route path without a leading slash (e.g. "docs/setup").
	RoutePath string
	// UpdatedAt is the last change; zero when unknown.
	UpdatedAt time.Time
}

// PageRouteLister lists the routes of every page and section anonymous
// visitors may read, depth-first in navigation order, at most limit of them.
// truncated reports that more exist.
type PageRouteLister interface {
	ListPageRoutes(ctx context.Context, limit int) (routes []PageRoute, truncated bool, err error)
}

// ─── BuildSitemapUseCase ─────────────────────────────────────────────────────

// BuildSitemapInput is the input for BuildSitemapUseCase.
type BuildSitemapInput struct {
	// BasePath is the URL prefix the wiki is served under ("" or e.g. "/wiki").
	BasePath string
}

// SitemapURL is one <url> entry.
type SitemapURL struct {
	// Loc is the absolute, escaped URL of the page.
	Loc string
	// LastMod is the last change; zero leaves <lastmod> out.
	LastMod time.Time
}

// BuildSitemapOutput is the sitemap of a public wiki.
type BuildSitemapOutput struct {
	URLs []SitemapURL
	// Truncated reports that the wiki has more than MaxSitemapURLs routes.
	Truncated bool
}

// BuildSitemapUseCase lists every page and section of a public wiki as a
// sitemap. Ported from BuildSitemap in René Jochum's #1538. The wiki root is
// not listed: it redirects to the first page, which is listed itself.
type BuildSitemapUseCase struct {
	pages        PageRouteLister
	publicOrigin string
}

// NewBuildSitemapUseCase constructs a BuildSitemapUseCase. publicURL is
// --public-url; only its scheme and host are used, the path comes from the
// base path. An empty or unusable publicURL means no sitemap.
func NewBuildSitemapUseCase(pages PageRouteLister, publicURL string) *BuildSitemapUseCase {
	return &BuildSitemapUseCase{pages: pages, publicOrigin: origin(publicURL)}
}

// Execute returns the sitemap, or nil without reading anything when
// --public-url is not set: sitemap entries must be absolute URLs, and they are
// never guessed from request headers (ADR-0018).
func (uc *BuildSitemapUseCase) Execute(ctx context.Context, in BuildSitemapInput) (*BuildSitemapOutput, error) {
	if uc.publicOrigin == "" {
		return nil, nil
	}
	routes, truncated, err := uc.pages.ListPageRoutes(ctx, MaxSitemapURLs)
	if err != nil {
		return nil, err
	}
	out := &BuildSitemapOutput{URLs: make([]SitemapURL, 0, len(routes)), Truncated: truncated}
	for _, route := range routes {
		loc, err := absoluteURL(uc.publicOrigin, in.BasePath, route.RoutePath)
		if err != nil {
			// Not a valid route (empty segment); the tree never produces one,
			// and one bad entry must not cost the whole sitemap.
			continue
		}
		out.URLs = append(out.URLs, SitemapURL{Loc: loc, LastMod: route.UpdatedAt})
	}
	return out, nil
}

const sitemapNamespace = "http://www.sitemaps.org/schemas/sitemap/0.9"

type sitemapEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// WriteXML streams the sitemap document to w, one entry at a time, so a
// large wiki is never held in memory as one XML document. encoding/xml
// escapes every value.
func (o *BuildSitemapOutput) WriteXML(w io.Writer) error {
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	urlset := xml.StartElement{Name: xml.Name{Local: "urlset"}, Attr: []xml.Attr{{Name: xml.Name{Local: "xmlns"}, Value: sitemapNamespace}}}
	url := xml.StartElement{Name: xml.Name{Local: "url"}}
	err := enc.EncodeToken(xml.ProcInst{Target: "xml", Inst: []byte(`version="1.0" encoding="UTF-8"`)})
	if err == nil {
		err = enc.EncodeToken(xml.CharData("\n"))
	}
	if err == nil {
		err = enc.EncodeToken(urlset)
	}
	for i := 0; err == nil && i < len(o.URLs); i++ {
		entry := sitemapEntry{Loc: o.URLs[i].Loc}
		if lastMod := o.URLs[i].LastMod; !lastMod.IsZero() {
			entry.LastMod = lastMod.UTC().Format(time.RFC3339)
		}
		err = enc.EncodeElement(entry, url)
	}
	if err == nil {
		err = enc.EncodeToken(urlset.End())
	}
	if err == nil {
		err = enc.Close()
	}
	return err
}
