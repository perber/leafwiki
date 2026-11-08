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
	"errors"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
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

	if err := checkComplexity(content); err != nil {
		return "", err
	}
	out := &cappedWriter{limit: maxOutputBytes}
	if err := markdownRenderer.Convert([]byte(content), out, parser.WithContext(pc)); err != nil {
		if out.exceeded {
			return "", ErrTooComplex
		}
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return balanceHTML(policy.SanitizeBytes(out.buf.Bytes()))
}

// errDisallowedMarkup means balanceHTML found markup outside the policy. The
// sanitizer runs first, so it only happens when the policy and the sanitizer
// disagree; failing closed turns such a mistake into "no rendered body".
var errDisallowedMarkup = errors.New("render: sanitized HTML contains markup outside the policy")

// checkAllowed walks the parsed tree: only allowed elements without a
// namespace, only their allowed attributes, and no comments or doctypes.
func checkAllowed(n *html.Node) error {
	switch n.Type {
	case html.TextNode:
		return nil
	case html.ElementNode:
		attrs, ok := allowedMarkup[n.Data]
		if !ok || n.Namespace != "" {
			return fmt.Errorf("%w: element %q", errDisallowedMarkup, n.Data)
		}
		for _, a := range n.Attr {
			if a.Namespace != "" || !attrs[a.Key] {
				return fmt.Errorf("%w: attribute %q on %q", errDisallowedMarkup, a.Key, n.Data)
			}
		}
	default:
		return fmt.Errorf("%w: node type %d", errDisallowedMarkup, n.Type)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if err := checkAllowed(c); err != nil {
			return err
		}
	}
	return nil
}

// balanceHTML parses sanitized HTML the way a browser does inside #root and
// serializes it again. The sanitizer filters tags but keeps them unbalanced, so
// a stray </div> would close #root and an unclosed <div> would swallow the
// shell behind it. It must run after the sanitizer: the sanitizer drops tags
// without their content, which is what can leave the input unbalanced.
func balanceHTML(sanitized []byte) (string, error) {
	nodes, err := html.ParseFragment(bytes.NewReader(sanitized), &html.Node{
		Type: html.ElementNode, Data: "div", DataAtom: atom.Div,
	})
	if err != nil {
		// x/net/html refuses more than 512 open elements.
		return "", fmt.Errorf("%w: %v", ErrTooComplex, err)
	}
	for _, node := range nodes {
		if err := checkAllowed(node); err != nil {
			return "", err
		}
	}
	var out strings.Builder
	for _, node := range nodes {
		if err := html.Render(&out, node); err != nil {
			return "", fmt.Errorf("balance html: %w", err)
		}
	}
	return out.String(), nil
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
