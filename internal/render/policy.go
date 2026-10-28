package render

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// notProtocolRelative accepts URLs that don't start with two slashes or
// backslashes after leading whitespace or control characters: browsers read
// those as "//host" and would leave the wiki.
var notProtocolRelative = regexp.MustCompile(`^[\x00-\x20]*(?:[^/\\\x00-\x20]|[/\\](?:[^/\\\x00-\x20]|$))`)

// policy is the allow-list for crawler HTML, built once (bluemonday policies
// are safe for concurrent use after setup).
//
// It is at least as strict as the SPA's rehype-sanitize schema
// (ui/leafwiki-ui/src/features/preview/previewSanitizeSchema.ts, pinned by
// TestPolicy_FrontendSchema_IsPinned): a subset of its GitHub tags, no
// audio/video, and no id, name, class or style anywhere, so page content can't
// clobber #root or #mermaid-renderer or restyle the shell. URLs are http,
// https, mailto or same-origin paths; "//host" is dropped.
var policy = newPolicy()

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(
		"b", "blockquote", "br", "code", "dd", "del", "details", "div", "dl", "dt", "em",
		"h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "ins", "kbd", "li", "mark", "ol",
		"p", "pre", "q", "s", "samp", "span", "strike", "strong", "sub", "summary", "sup",
		"table", "tbody", "td", "tfoot", "th", "thead", "tr", "tt", "ul", "var",
	)
	p.AllowAttrs("href").Matching(notProtocolRelative).OnElements("a")
	p.AllowAttrs("src").Matching(notProtocolRelative).OnElements("img")
	p.AllowAttrs("alt").OnElements("img")
	p.AllowAttrs("title").OnElements("a", "img")
	p.AllowAttrs("width", "height").Matching(bluemonday.NumberOrPercent).OnElements("img")
	// Task list checkboxes only; an <input> without a checkbox type is dropped.
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("checked", "disabled").Matching(regexp.MustCompile(`^$`)).OnElements("input")
	p.AllowAttrs("align").Matching(regexp.MustCompile(`^(left|center|right)$`)).OnElements("td", "th")
	p.AllowAttrs("colspan", "rowspan").Matching(bluemonday.Integer).OnElements("td", "th")
	p.AllowAttrs("start").Matching(bluemonday.Integer).OnElements("ol")
	p.AllowAttrs("open").Matching(regexp.MustCompile(`^$`)).OnElements("details")
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(true)
	p.RequireParseableURLs(true)
	return p
}
