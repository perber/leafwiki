package http

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
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
	doc, _ := h.render(ShellRequest{Request: httptest.NewRequest(http.MethodGet, path, nil), RoutePath: routePath, BasePath: opts.BasePath})
	return doc
}

// ServeShell serves a GET request to path through the SPA fallback route with
// indexTemplate as the embedded index, so tests see status and headers
// without a built dist/.
func ServeShell(indexTemplate string, cfg FrontendConfig, opts RouterOptions, path string) *httptest.ResponseRecorder {
	h := newShellHandler(cfg, opts, "")
	h.setIndex([]byte(indexTemplate), nil)
	engine := gin.New()
	engine.NoRoute(h.serve)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}
