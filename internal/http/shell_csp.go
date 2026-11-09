package http

import (
	"regexp"
	"strings"
)

// shellCSP returns the Content-Security-Policy of the SPA shell document, or
// "" when security headers are disabled. See ADR-0024 for why it has no
// default-src and no style-, img-, font-, media- or connect-src: each would
// block legitimate wiki content or admin CSS. The directives below do not fall
// back to default-src, so they hold without it.
//
// script-src 'self' is left out while InjectCodeInHeader contains active
// script: the built index.html has no inline script, so the admin's header
// code is the only source the restriction would block.
func shellCSP(opts RouterOptions) string {
	if opts.DisableSecurityHeaders {
		return ""
	}
	const policy = "object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"
	if containsActiveScript(opts.InjectCodeInHeader) {
		return policy
	}
	return "script-src 'self'; " + policy
}

// inlineHandlerAttr matches an on…= event handler attribute after whitespace,
// a slash or a closing quote (<img/onerror=…>, <img src="x"onerror=…>).
var inlineHandlerAttr = regexp.MustCompile(`[\s/"']on[a-z]+\s*=`)

// containsActiveScript reports whether admin header code would run script:
// a <script> element, a javascript: URL or an inline event handler. It errs
// toward true: a false positive only costs the script restriction for an
// install that set header code, a false negative breaks that code visibly.
func containsActiveScript(code string) bool {
	lower := strings.ToLower(code)
	return strings.Contains(lower, "<script") ||
		strings.Contains(lower, "javascript:") ||
		inlineHandlerAttr.MatchString(lower)
}
