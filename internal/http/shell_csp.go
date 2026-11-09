package http

// shellCSP returns the Content-Security-Policy of the SPA shell document, or
// "" when security headers are disabled. See ADR-0024 for why it has no
// default-src and no style-, img-, font-, media- or connect-src: each would
// block legitimate wiki content or admin CSS. The directives below do not fall
// back to default-src, so they hold without it.
func shellCSP(opts RouterOptions) string {
	if opts.DisableSecurityHeaders {
		return ""
	}
	return "object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"
}
