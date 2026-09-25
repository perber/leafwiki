// Package seo renders wiki pages for search engine crawlers, link-preview
// bots, and LLM agents that request a page without executing JavaScript.
// Output here only needs to be readable by those consumers — it is not meant
// to match the client-side react-markdown renderer byte-for-byte, and custom
// syntax such as wiki-links or callout blocks is intentionally left as plain
// Markdown/GFM rather than reimplemented as goldmark extensions.
package seo

import (
	"bytes"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

var renderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

// sanitizePolicy mirrors the safety posture of the client-side renderer,
// which pairs raw-HTML passthrough (rehype-raw) with an allowlist sanitizer
// (rehype-sanitize). Page content is editor-authored and normally trusted,
// but the SSR path serves raw markup directly to every unauthenticated
// visitor before any JavaScript runs, so a stray <script>/onerror= in a page
// must not survive rendering here the way it would in a trusted-editor-only
// context.
var sanitizePolicy = bluemonday.UGCPolicy()

// RenderHTML converts a page's markdown body into sanitized HTML suitable
// for injection into the SSR shell.
func RenderHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := renderer.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return sanitizePolicy.Sanitize(buf.String()), nil
}
