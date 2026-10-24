package http

import (
	"net/http"
	"net/http/httptest"
)

// Re-export unexported helpers for use in external test package (package http_test).

var BuildCustomStylesheetTag = buildCustomStylesheetTag
var InjectIntoHead = injectIntoHead

// RenderShell renders indexTemplate for a GET request to path, the way the SPA
// fallback route does once the index has been read from the embedded dist.
func RenderShell(indexTemplate string, cfg FrontendConfig, opts RouterOptions, customStylesheetPath, path string) string {
	h := newShellHandler(cfg, opts, customStylesheetPath)
	h.setIndex([]byte(indexTemplate), nil)
	routePath, _ := shellRoutePath(path, opts.BasePath)
	return h.render(ShellRequest{Request: httptest.NewRequest(http.MethodGet, path, nil), RoutePath: routePath, BasePath: opts.BasePath})
}
