package seo

import (
	"html"
	"strings"

	"github.com/perber/wiki/internal/core/excerpt"
)

// PageMeta holds the resolved SEO metadata for a single matched page.
type PageMeta struct {
	SiteName     string
	PageTitle    string
	Description  string
	CanonicalURL string // absolute URL; empty when unknown
}

// BuildPageMeta resolves metadata for a page's head tags. content is the
// page's markdown body (frontmatter already stripped); the description is
// derived from it the same way search/list excerpts are.
func BuildPageMeta(siteName, pageTitle, content, canonicalURL string) PageMeta {
	return PageMeta{
		SiteName:     siteName,
		PageTitle:    pageTitle,
		Description:  excerpt.FromContent(content),
		CanonicalURL: strings.TrimSpace(canonicalURL),
	}
}

// DocumentTitle returns the <title> content for a matched page.
func (m PageMeta) DocumentTitle() string {
	switch {
	case m.PageTitle == "":
		return m.SiteName
	case m.SiteName == "":
		return m.PageTitle
	default:
		return m.PageTitle + " · " + m.SiteName
	}
}

// HeadTags renders the description/canonical/OpenGraph meta tags as escaped
// HTML ready for injection into <head>.
func (m PageMeta) HeadTags() string {
	var b strings.Builder
	writeTag := func(s string) {
		b.WriteString(s)
		b.WriteString("\n    ")
	}

	if m.Description != "" {
		writeTag(`<meta name="description" content="` + html.EscapeString(m.Description) + `">`)
	}
	if m.CanonicalURL != "" {
		writeTag(`<link rel="canonical" href="` + html.EscapeString(m.CanonicalURL) + `">`)
	}
	writeTag(`<meta property="og:type" content="article">`)
	if m.PageTitle != "" {
		writeTag(`<meta property="og:title" content="` + html.EscapeString(m.PageTitle) + `">`)
	}
	if m.Description != "" {
		writeTag(`<meta property="og:description" content="` + html.EscapeString(m.Description) + `">`)
	}
	if m.CanonicalURL != "" {
		writeTag(`<meta property="og:url" content="` + html.EscapeString(m.CanonicalURL) + `">`)
	}

	return strings.TrimRight(b.String(), "\n ")
}
