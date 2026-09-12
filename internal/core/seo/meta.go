package seo

import (
	"html"
	"strings"

	"github.com/perber/wiki/internal/core/excerpt"
	"github.com/perber/wiki/internal/core/markdown"
)

// PageMeta holds the resolved SEO metadata for a single matched page.
type PageMeta struct {
	SiteName     string
	PageTitle    string
	Description  string
	CanonicalURL string // absolute URL; empty when unknown
}

// BuildPageMeta resolves metadata for a page's head tags. rawContent is the
// page's full markdown file including frontmatter; passing an already-stripped
// body works too, it just has no frontmatter to read.
//
// The description prefers an author-written "description" frontmatter field and
// otherwise falls back to an excerpt of the body, derived the same way
// search/list excerpts are. A frontmatter value is used as written: unlike an
// excerpt it is deliberate, so it is not truncated.
func BuildPageMeta(siteName, pageTitle, rawContent, canonicalURL string) PageMeta {
	// On a parse error ParseFrontmatter hands back the input unchanged, so the
	// excerpt is still taken over the whole file rather than nothing.
	fm, body, hasFrontmatter, err := markdown.ParseFrontmatter(rawContent)

	description := ""
	if err == nil && hasFrontmatter {
		if value, ok := fm.ExtraFields["description"].(string); ok {
			description = strings.TrimSpace(value)
		}
	}
	if description == "" {
		description = excerpt.FromBody(body)
	}

	return PageMeta{
		SiteName:     siteName,
		PageTitle:    pageTitle,
		Description:  description,
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
