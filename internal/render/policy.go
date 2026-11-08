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

// allowedElements and attrRules are the policy's single source: newPolicy
// builds the sanitizer from them and balanceHTML refuses anything outside them.
var allowedElements = []string{
	"b", "blockquote", "br", "code", "dd", "del", "details", "div", "dl", "dt", "em",
	"h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "ins", "kbd", "li", "mark", "ol",
	"p", "pre", "q", "s", "samp", "span", "strike", "strong", "sub", "summary", "sup",
	"table", "tbody", "td", "tfoot", "th", "thead", "tr", "tt", "ul", "var",
}

// attrRule allows attr on elements, matching match when it is set. An element
// that only appears here (a, img, input) is allowed together with the attribute.
type attrRule struct {
	attr     string
	match    *regexp.Regexp
	elements []string
}

var (
	emptyValue = regexp.MustCompile(`^$`)
	attrRules  = []attrRule{
		{attr: "href", match: notProtocolRelative, elements: []string{"a"}},
		{attr: "src", match: notProtocolRelative, elements: []string{"img"}},
		{attr: "alt", elements: []string{"img"}},
		{attr: "title", elements: []string{"a", "img"}},
		{attr: "width", match: bluemonday.NumberOrPercent, elements: []string{"img"}},
		{attr: "height", match: bluemonday.NumberOrPercent, elements: []string{"img"}},
		// Task list checkboxes only; an <input> without a checkbox type is dropped.
		{attr: "type", match: regexp.MustCompile(`^checkbox$`), elements: []string{"input"}},
		{attr: "checked", match: emptyValue, elements: []string{"input"}},
		{attr: "disabled", match: emptyValue, elements: []string{"input"}},
		{attr: "align", match: regexp.MustCompile(`^(left|center|right)$`), elements: []string{"td", "th"}},
		{attr: "colspan", match: bluemonday.Integer, elements: []string{"td", "th"}},
		{attr: "rowspan", match: bluemonday.Integer, elements: []string{"td", "th"}},
		{attr: "start", match: bluemonday.Integer, elements: []string{"ol"}},
		{attr: "open", match: emptyValue, elements: []string{"details"}},
	}
)

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(allowedElements...)
	for _, rule := range attrRules {
		attrs := p.AllowAttrs(rule.attr)
		if rule.match != nil {
			attrs = attrs.Matching(rule.match)
		}
		attrs.OnElements(rule.elements...)
	}
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(true)
	p.RequireParseableURLs(true)
	return p
}

// allowedMarkup is element -> allowed attribute names, built from the same
// tables as the policy.
var allowedMarkup = func() map[string]map[string]bool {
	m := map[string]map[string]bool{}
	for _, el := range allowedElements {
		m[el] = map[string]bool{}
	}
	for _, rule := range attrRules {
		for _, el := range rule.elements {
			if m[el] == nil {
				m[el] = map[string]bool{}
			}
			m[el][rule.attr] = true
		}
	}
	return m
}()
