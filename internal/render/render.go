// Package render turns page Markdown into sanitized HTML for clients that do
// not run the SPA, such as search-engine crawlers (#1292).
//
// The output is untrusted page content placed into the SPA shell, which has no
// Content-Security-Policy, so the sanitizer in policy.go is the only XSS
// barrier. The package knows nothing about HTTP or the tree: the caller passes
// the base path, the page's route path and a wikilink resolver. See ADR-0020.
package render

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// Options configures one RenderHTML call.
type Options struct {
	// BasePath is the router base path ("" or "/wiki"); it prefixes every
	// internal link and asset URL.
	BasePath string
	// RoutePath is the route path of the rendered page ("/docs/setup").
	// Relative links resolve below it: the page is the folder.
	RoutePath string
	// ResolveWikiLink maps a [[Target]] to the route path of exactly one
	// page. It returns ok=false for unknown or ambiguous targets, which
	// render as plain text. Nil renders every wikilink as plain text.
	ResolveWikiLink func(target string) (routePath string, ok bool)
}

var optionsKey = parser.NewContextKey()

// markdownRenderer is built once; goldmark is safe for concurrent use. Raw
// HTML is passed through (WithUnsafe) and removed or kept by the sanitizer,
// which also drops dangerous URL schemes goldmark would otherwise blank.
var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.Linkify,
		extension.Strikethrough,
		extension.TaskList,
		extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
	),
	goldmark.WithParserOptions(
		parser.WithInlineParsers(util.Prioritized(wikiLinkParser{}, 100)),
		parser.WithASTTransformers(util.Prioritized(linkTransformer{}, 100)),
	),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

// RenderHTML renders content (page Markdown without frontmatter) to HTML that
// is safe to embed in the SPA shell. Wikilinks inside code stay literal;
// Mermaid, math and embeds stay code or text.
func RenderHTML(content string, opt Options) (string, error) {
	opt.BasePath = normalizeBasePath(opt.BasePath)
	pc := parser.NewContext()
	pc.Set(optionsKey, &opt)

	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(content), &buf, parser.WithContext(pc)); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return string(policy.SanitizeBytes(buf.Bytes())), nil
}

func optionsFrom(pc parser.Context) *Options {
	if opt, ok := pc.Get(optionsKey).(*Options); ok {
		return opt
	}
	return &Options{}
}

// normalizeBasePath turns "", "/", "wiki" and "/wiki/" into "" or "/wiki".
func normalizeBasePath(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	return "/" + p
}
