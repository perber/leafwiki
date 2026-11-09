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
//     because Chrome refuses to render PDFs in a sandboxed document;
//   - Content-Type application/octet-stream for script types, so with nosniff
//     a <script src> on the wiki origin cannot load an upload (ADR-0024).
//     Other types keep theirs; an SVG only renders in <img> as an image.
//
// A route that sets Content-Type itself does so before calling this.
func SetUntrustedFileHeaders(h http.Header, name string) {
	h.Set("X-Content-Type-Options", "nosniff")
	if isScriptType(name, h.Get("Content-Type")) {
		h.Set("Content-Type", "application/octet-stream")
	}

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

// scriptExts are the extensions browsers would run as a module or classic
// script when served with a JavaScript type.
var scriptExts = map[string]struct{}{".js": {}, ".mjs": {}, ".cjs": {}}

// isScriptType reports whether a file would be served with a JavaScript MIME
// type: by its name, by the type the system maps its extension to (on Windows
// that comes from the registry), or by the Content-Type already set.
func isScriptType(name, contentType string) bool {
	ext := strings.ToLower(path.Ext(name))
	if _, ok := scriptExts[ext]; ok {
		return true
	}
	return isJavaScriptMIME(mime.TypeByExtension(ext)) || isJavaScriptMIME(contentType)
}

// isJavaScriptMIME covers the JavaScript MIME type essences of the WHATWG
// MIME Sniffing standard (text/javascript, application/x-ecmascript, …).
func isJavaScriptMIME(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	for _, marker := range []string{"javascript", "ecmascript", "jscript", "livescript"} {
		if strings.Contains(mediaType, marker) {
			return true
		}
	}
	return false
}
