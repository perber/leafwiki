package security

import (
	"mime"
	"net/http"
	"path"
	"strings"
)

// inlineFileExts lists the extensions that may be rendered by the browser
// when navigated to directly. Everything else (HTML, SVG, XML, unknown or
// missing extensions, ...) is served as a download, so uploaded content can
// never execute as a document on the wiki origin (stored XSS).
// Embedding via <img>, <video>, <audio> or the PDF embed is unaffected.
var inlineFileExts = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".avif": {}, ".bmp": {}, ".ico": {},
	".pdf": {},
	".mp4": {}, ".webm": {}, ".ogv": {}, ".mov": {}, ".m4v": {},
	".mp3": {}, ".wav": {}, ".ogg": {}, ".oga": {}, ".m4a": {}, ".flac": {}, ".aac": {}, ".opus": {},
}

// SetUntrustedFileHeaders hardens a response that serves user-supplied file
// bytes from the wiki origin. It must be applied by every route that serves
// uploaded assets (live or from a revision), not just /assets:
//   - nosniff, so the declared type is final;
//   - Content-Disposition attachment for every type not on the inline
//     allowlist, so active content is downloaded instead of rendered;
//   - CSP sandbox, so even a document that does render runs in an opaque
//     origin without access to the wiki's cookies or API. PDF is exempt
//     because Chrome refuses to render PDFs in a sandboxed document.
func SetUntrustedFileHeaders(h http.Header, name string) {
	h.Set("X-Content-Type-Options", "nosniff")

	ext := strings.ToLower(path.Ext(name))
	if ext != ".pdf" {
		h.Set("Content-Security-Policy", "sandbox")
	}

	disposition := "attachment"
	if _, ok := inlineFileExts[ext]; ok {
		disposition = "inline"
	}
	if v := mime.FormatMediaType(disposition, map[string]string{"filename": path.Base(name)}); v != "" {
		disposition = v
	}
	h.Set("Content-Disposition", disposition)
}
