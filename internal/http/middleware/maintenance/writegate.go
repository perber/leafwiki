package maintenance

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/restore"
)

// exemptPathSegmentSequences are consecutive path-segment sequences that
// stay reachable even while the gate is engaged, checked as whole segments
// (via pathContainsSegments) rather than a raw substring — a substring check
// against the raw URL would both under- and over-match: any accidental
// substring collision (e.g. --base-path configured as "/api/admin/restore-x",
// or a future unrelated route like "/api/admin/restore-policy") would bypass
// the gate for everything, while a literal path-segment match cannot.
//
//   - {"api","admin","restore"} — the restore trigger endpoint fires before
//     the gate is engaged (validation happens first), and the self-restart
//     recovery endpoint must stay reachable while the gate is engaged; it's
//     the documented way out of a stuck restore.
//   - the four auth endpoints — each only ever calls into AuthService's
//     userService a single time per request (verified: Login, CompleteTOTPLogin,
//     RefreshToken), so none of them can straddle an in-flight
//     AuthService.ReplaceUserStore swap the way a multi-call operation could;
//     letting them through avoids an unrelated 503 on login/logout/refresh
//     during the (usually sub-second) swap window. TOTP setup/confirm/disable
//     are deliberately NOT exempted — they write to the same users.db a
//     restore is about to swap out.
var exemptPathSegmentSequences = [][]string{
	{"api", "admin", "restore"},
	{"api", "auth", "login"},
	{"api", "auth", "refresh-token"},
	{"api", "auth", "logout"},
}

// pathContainsSegments reports whether path contains the given sequence of
// segments consecutively, anywhere — so it works regardless of --base-path
// (the sequence just needs to appear somewhere after whatever prefix), and
// so "restore-policy" (a different segment) never matches a check for
// "restore".
func pathContainsSegments(path string, want []string) bool {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for start := 0; start+len(want) <= len(segments); start++ {
		match := true
		for i, w := range want {
			if segments[start+i] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func isExemptPath(path string) bool {
	for _, want := range exemptPathSegmentSequences {
		if pathContainsSegments(path, want) {
			return true
		}
	}
	return false
}

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

		if isExemptPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		leave, ok := gate.TryEnter()
		if !ok {
			loc, _ := sharederrors.AsLocalizedError(restore.ErrWritesDisabled)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"code":     loc.Code,
					"message":  loc.Message,
					"template": loc.Template,
				},
			})
			return
		}
		defer leave()

		c.Next()
	}
}
