package seo

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

var (
	errInvalidRoutePath   = errors.New("seo: first page has an invalid route path")
	errUnsafeRootLocation = errors.New("seo: root redirect location would leave the wiki")
)

// FirstPage is the page a public wiki opens at its root.
type FirstPage struct {
	// RoutePath is the page's route path without a leading slash (e.g. "docs/setup").
	RoutePath string
}

// FirstPageReader reads the page the wiki opens at its root. It returns nil
// and no error when there is none (empty tree, tree not loaded yet).
type FirstPageReader interface {
	FirstPage(ctx context.Context) (*FirstPage, error)
}

// ─── GetRootTargetUseCase ────────────────────────────────────────────────────

// GetRootTargetInput is the input for GetRootTargetUseCase.
type GetRootTargetInput struct {
	// BasePath is the URL prefix the wiki is served under ("" or e.g. "/wiki").
	BasePath string
}

// GetRootTargetOutput is the result of GetRootTargetUseCase.
type GetRootTargetOutput struct {
	// Location is the escaped, server-relative redirect target, or "" when the
	// wiki has no page to redirect to.
	Location string
}

// GetRootTargetUseCase decides where the wiki root redirects to: the first
// page, like the SPA's own root redirect.
type GetRootTargetUseCase struct {
	pages FirstPageReader
}

// NewGetRootTargetUseCase constructs a GetRootTargetUseCase.
func NewGetRootTargetUseCase(pages FirstPageReader) *GetRootTargetUseCase {
	return &GetRootTargetUseCase{pages: pages}
}

// Execute returns the redirect target. The location is built from the base
// path and the page's route path only, never from request headers, so it is
// always a path on this wiki.
func (uc *GetRootTargetUseCase) Execute(ctx context.Context, in GetRootTargetInput) (*GetRootTargetOutput, error) {
	page, err := uc.pages.FirstPage(ctx)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return &GetRootTargetOutput{}, nil
	}
	location, err := rootLocation(in.BasePath, page.RoutePath)
	if err != nil {
		return nil, err
	}
	return &GetRootTargetOutput{Location: location}, nil
}

// rootLocation escapes the base path and every route path segment. Escaping
// turns CR, LF, "?", "#" and "\" into percent escapes; a location starting
// with "//" (a protocol-relative URL to another host) is refused.
func rootLocation(basePath, routePath string) (string, error) {
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
		return "", errUnsafeRootLocation
	}
	return location, nil
}
