package render

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// schemeRe matches a URL scheme such as "https:" or "mailto:".
var schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)

// linkTransformer rewrites Markdown link and image URLs the way the SPA
// resolves them (MarkdownLink.tsx, assetSrc.ts), so a crawler following them
// lands on the same page or asset. Raw HTML is left as written; the sanitizer
// still checks its URLs.
type linkTransformer struct{}

func (linkTransformer) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	opt := optionsFrom(pc)
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Link:
			n.Destination = resolveHref(opt, n.Destination)
		case *ast.Image:
			n.Destination = resolveSrc(opt, n.Destination)
		}
		return ast.WalkContinue, nil
	})
}

// resolveHref maps asset links and page links (relative ones below the
// current page) to base-path URLs. Fragments, URLs with a scheme and
// protocol-relative URLs are returned unchanged; the sanitizer filters them.
func resolveHref(opt *Options, raw []byte) []byte {
	dest := string(util.URLEscape(raw, true))
	if strings.HasPrefix(dest, "assets/") || strings.HasPrefix(dest, "/assets/") {
		return []byte(opt.BasePath + "/" + strings.TrimPrefix(dest, "/"))
	}
	if dest == "" || strings.HasPrefix(dest, "#") || schemeRe.MatchString(dest) || isProtocolRelative(dest) {
		return raw
	}
	u, err := url.Parse(dest)
	if err != nil {
		return raw
	}
	route, err := markdown.ResolveFolderRelative(opt.RoutePath, dest)
	if err != nil {
		return raw
	}
	out := opt.BasePath + escapeSegments(route)
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.EscapedFragment()
	}
	return []byte(out)
}

// resolveSrc adds the base path to asset and API image sources, like
// normalizeAssetSrc in the SPA. Other sources are returned unchanged.
func resolveSrc(opt *Options, raw []byte) []byte {
	dest := string(util.URLEscape(raw, true))
	switch {
	case strings.HasPrefix(dest, "/assets/"), strings.HasPrefix(dest, "/api/"):
		return []byte(opt.BasePath + dest)
	case strings.HasPrefix(dest, "assets/"):
		return []byte(opt.BasePath + "/" + dest)
	}
	return raw
}

// isProtocolRelative reports whether browsers treat u as "//host...":
// they read a backslash like a slash.
func isProtocolRelative(u string) bool {
	return len(u) >= 2 && (u[0] == '/' || u[0] == '\\') && (u[1] == '/' || u[1] == '\\')
}

// escapeSegments percent-escapes each segment of an unescaped route path.
func escapeSegments(route string) string {
	segments := strings.Split(route, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}
