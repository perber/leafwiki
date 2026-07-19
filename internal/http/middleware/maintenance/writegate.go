package maintenance

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/restore"
)

// restoreAdminPathSegment is exempted from the gate: the restore trigger
// endpoint fires before the gate is engaged (validation happens first), and
// the self-restart recovery endpoint must stay reachable while the gate is
// engaged — it's the documented way out of a stuck restore. Both are POSTs,
// so a method-only exemption (like CSRF's) isn't enough here.
const restoreAdminPathSegment = "/api/admin/restore"

// WriteGateMiddleware returns 503 for any non-GET/HEAD/OPTIONS request while
// gate is engaged (a restore is swapping live files), so a write never lands
// in a directory that's about to be renamed away.
func WriteGateMiddleware(gate *restore.WriteGate) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}

		if strings.Contains(c.Request.URL.Path, restoreAdminPathSegment) {
			c.Next()
			return
		}

		leave, ok := gate.TryEnter()
		if !ok {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "A restore is in progress; writes are temporarily disabled",
			})
			return
		}
		defer leave()

		c.Next()
	}
}
