package seo

import (
	"context"
	"strconv"
	"strings"
)

// ─── GetPageMarkdownUseCase ──────────────────────────────────────────────────

// GetPageMarkdownInput is the input for GetPageMarkdownUseCase.
type GetPageMarkdownInput struct {
	// RoutePath is the shell request's route path, starting with "/".
	RoutePath string
	// Accept is the request's Accept header; several header lines are joined
	// with ",".
	Accept string
}

// GetPageMarkdownOutput is the Markdown representation of a page.
type GetPageMarkdownOutput struct {
	// Markdown is the page's content without frontmatter.
	Markdown string
}

// GetPageMarkdownUseCase answers a request that asks for Markdown with the
// page's Markdown, for agents and tools that read Markdown better than HTML.
type GetPageMarkdownUseCase struct {
	pages PageReader
}

// NewGetPageMarkdownUseCase constructs a GetPageMarkdownUseCase.
func NewGetPageMarkdownUseCase(pages PageReader) *GetPageMarkdownUseCase {
	return &GetPageMarkdownUseCase{pages: pages}
}

// Execute returns the page's Markdown, or nil when the request does not
// prefer Markdown or is a permalink (then nothing is read), or the route is
// not a page.
func (uc *GetPageMarkdownUseCase) Execute(ctx context.Context, in GetPageMarkdownInput) (*GetPageMarkdownOutput, error) {
	routePath := strings.TrimPrefix(in.RoutePath, "/")
	if routePath == "" || !prefersMarkdown(in.Accept) {
		return nil, nil
	}
	// A permalink shows its target page in the SPA, not the page at this
	// path; it has no Markdown representation (ADR-0018).
	if _, ok := permalinkID(routePath); ok {
		return nil, nil
	}
	page, err := uc.pages.PageByRoutePath(ctx, routePath)
	if err != nil || page == nil {
		return nil, err
	}
	return &GetPageMarkdownOutput{Markdown: page.Body}, nil
}

// prefersMarkdown reports whether accept asks for text/markdown at least as
// much as for text/html (RFC 9110, section 12.5.1). text/markdown must be
// listed explicitly with q > 0: "*/*" and "text/*" match it too, but every
// browser sends them. text/html gets the q of its most specific match
// (text/html, then text/*, then */*). A tie goes to Markdown. Ranges with a
// malformed q value are ignored.
//
// Quoted parameter values are not parsed: a "," or ";" inside quotes splits
// the header there, so `text/html;q=0.5;ext="x,text/markdown;y=z"` selects
// Markdown. That only lets a client pick a representation it could ask for
// directly, so the simple split is kept.
func prefersMarkdown(accept string) bool {
	var markdown, html acceptMatch
	for element := range strings.SplitSeq(accept, ",") {
		mediaRange, q, ok := parseAcceptElement(element)
		if !ok {
			continue
		}
		switch mediaRange {
		case "text/markdown":
			markdown.add(3, q)
		case "text/html":
			html.add(3, q)
		case "text/*":
			html.add(2, q)
		case "*/*":
			html.add(1, q)
		}
	}
	return markdown.specificity == 3 && markdown.q > 0 && markdown.q >= html.q
}

// acceptMatch is the most specific media range matching a type seen so far;
// among equally specific ranges the highest q wins.
type acceptMatch struct {
	specificity int
	q           float64
}

func (m *acceptMatch) add(specificity int, q float64) {
	if specificity > m.specificity || (specificity == m.specificity && q > m.q) {
		m.specificity, m.q = specificity, q
	}
}

// parseAcceptElement returns the lower-cased media range of one Accept
// element and its q value (1 when absent). Media type parameters other than
// q are ignored. ok is false for a malformed range or q value.
func parseAcceptElement(element string) (mediaRange string, q float64, ok bool) {
	mediaRange, params, _ := strings.Cut(element, ";")
	mediaRange = strings.ToLower(strings.TrimSpace(mediaRange))
	typ, subtype, found := strings.Cut(mediaRange, "/")
	if !found || typ == "" || subtype == "" {
		return "", 0, false
	}
	q = 1
	for param := range strings.SplitSeq(params, ";") {
		key, value, _ := strings.Cut(param, "=")
		if !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}
		if q, ok = parseQValue(strings.TrimSpace(value)); !ok {
			return "", 0, false
		}
		// Parameters after q are accept extensions, not media type parameters.
		break
	}
	return mediaRange, q, true
}

// parseQValue parses a qvalue: "0" or "1", optionally followed by "." and
// up to three digits, at most 1 (RFC 9110, section 12.4.2). The strict
// grammar keeps out "NaN", "1e0" and the like that strconv would accept.
func parseQValue(s string) (float64, bool) {
	if s == "" || len(s) > 5 || (s[0] != '0' && s[0] != '1') {
		return 0, false
	}
	if len(s) > 1 {
		if s[1] != '.' {
			return 0, false
		}
		for _, c := range s[2:] {
			if c < '0' || c > '9' || (s[0] == '1' && c != '0') {
				return 0, false
			}
		}
	}
	q, err := strconv.ParseFloat(s, 64)
	return q, err == nil
}
