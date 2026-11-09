package security

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// HeadersConfig configures Headers.
type HeadersConfig struct {
	// NosniffExemptPrefix is the request path prefix served without
	// X-Content-Type-Options: nosniff, i.e. BasePath + "/static/" of the
	// embedded frontend. Go derives the type of those files from the extension,
	// which on Windows reads the registry; a ".js → text/plain" entry plus
	// nosniff would block every module script. Empty means no exemption.
	NosniffExemptPrefix string
}

// Headers sets the response headers that hold for every response, see
// ADR-0024. They are set before the handler runs, so a handler can override
// them and 404s and recovered panics carry them too. The document's
// Content-Security-Policy is not set here: the SPA shell owns it, and
// SetUntrustedFileHeaders owns the one on user files.
func Headers(cfg HeadersConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if cfg.NosniffExemptPrefix == "" || !strings.HasPrefix(c.Request.URL.Path, cfg.NosniffExemptPrefix) {
			h.Set("X-Content-Type-Options", "nosniff")
		}
		c.Next()
	}
}
