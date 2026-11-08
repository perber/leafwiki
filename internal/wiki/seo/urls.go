package seo

import (
	"errors"
	"net/url"
	"strings"
)

var (
	errInvalidRoutePath = errors.New("seo: page has an invalid route path")
	errUnsafeWikiPath   = errors.New("seo: path would leave the wiki")
)

// wikiPath returns the escaped, server-relative path of a page: the base path
// and every route path segment escaped. Escaping turns CR, LF, "?", "#" and
// "\" into percent escapes; a path starting with "//" (a protocol-relative
// URL to another host) is refused.
func wikiPath(basePath, routePath string) (string, error) {
	var b strings.Builder
	b.WriteString((&url.URL{Path: strings.TrimRight(basePath, "/")}).EscapedPath())
	for _, segment := range strings.Split(routePath, "/") {
		if segment == "" {
			return "", errInvalidRoutePath
		}
		b.WriteString("/")
		b.WriteString(url.PathEscape(segment))
	}
	location := b.String()
	if !strings.HasPrefix(location, "/") || strings.HasPrefix(location, "//") {
		return "", errUnsafeWikiPath
	}
	return location, nil
}

// origin returns "scheme://host" of --public-url when it is an absolute
// http(s) URL, or "". Absolute URLs are built from it and the base path only,
// never from Host or X-Forwarded-* headers (ADR-0018).
func origin(publicURL string) string {
	u, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// absoluteURL returns the absolute URL of a path on this wiki: the public
// origin (see origin) plus the escaped wikiPath.
func absoluteURL(publicOrigin, basePath, routePath string) (string, error) {
	path, err := wikiPath(basePath, routePath)
	if err != nil {
		return "", err
	}
	return publicOrigin + path, nil
}

// permalinkID returns the page ID of a permalink route path (without the
// leading "/"): "p/<id>" or "p/<id>/<slug>", trailing slashes allowed, "p" in
// any case. It mirrors the SPA route "/p/:id/:slug?"
// (ui/leafwiki-ui/src/features/router/router.tsx), which react-router matches
// case-insensitively; it is the only SPA route this package knows
// (ADR-0018), so change both together. Anything else under "p/" falls
// through to the SPA's page route, so it is an ordinary page route here too.
func permalinkID(routePath string) (string, bool) {
	prefix, rest, ok := strings.Cut(routePath, "/")
	if !ok || !strings.EqualFold(prefix, "p") {
		return "", false
	}
	id, slug, _ := strings.Cut(strings.TrimRight(rest, "/"), "/")
	if id == "" || strings.Contains(slug, "/") {
		return "", false
	}
	return id, true
}
