package http

// Re-export unexported helpers for use in external test package (package http_test).

var BuildCustomStylesheetTag = buildCustomStylesheetTag
var InjectIntoHead = injectIntoHead

// RenderShell renders indexTemplate for a GET request to path, the way the SPA
// fallback route does once the index has been read from the embedded dist.
func RenderShell(indexTemplate string, cfg FrontendConfig, opts RouterOptions, customStylesheetPath, path string) string {
	return renderIndex(indexTemplate, cfg, opts, customStylesheetPath)
}
