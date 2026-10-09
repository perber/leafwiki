package assets

import (
	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// assetServeHeaders hardens the static /assets responses; see
// security.SetUntrustedFileHeaders for the rules.
func assetServeHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		security.SetUntrustedFileHeaders(c.Writer.Header(), c.Request.URL.Path)
		c.Next()
	}
}
