package assets

import (
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// inlineAssetExts lists the extensions that may be rendered by the browser
// when navigated to directly. Everything else (HTML, SVG, XML, unknown or
// missing extensions, ...) is served as a download, so uploaded content can
// never execute as a document on the wiki origin (stored XSS).
// Embedding via <img>, <video>, <audio> or the PDF embed is unaffected.
var inlineAssetExts = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".avif": {}, ".bmp": {}, ".ico": {},
	".pdf": {},
	".mp4": {}, ".webm": {}, ".ogv": {}, ".mov": {}, ".m4v": {},
	".mp3": {}, ".wav": {}, ".ogg": {}, ".oga": {}, ".m4a": {}, ".flac": {}, ".aac": {}, ".opus": {},
}

// assetServeHeaders hardens the static /assets responses: it disables MIME
// sniffing and forces a download for every type not on the inline allowlist.
func assetServeHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		ext := strings.ToLower(path.Ext(c.Request.URL.Path))
		if _, ok := inlineAssetExts[ext]; !ok {
			c.Header("Content-Disposition", "attachment")
		}
		c.Next()
	}
}
