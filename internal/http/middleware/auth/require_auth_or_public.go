package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/publicaccess"
)

// RequireAuthOrPublicRead gates the read-only route groups that used to be
// registered on either a bare public group or an authenticated group depending
// on a boot-time flag. Registering them once behind this middleware is what
// lets public mode flip without a restart.
//
// It shares resolveSession with RequireAuth, so identity resolution is
// identical by construction. The only difference: where RequireAuth aborts
// with 401, this middleware instead calls c.Next() *anonymously* when public
// mode is currently on. A 500 (invalid user context, misconfigured
// authService) still aborts regardless.
//
// Consequence worth noting: a valid session cookie is resolved and attached
// even while public mode is on, so a logged-in editor keeps write affordances.
// The pre-refactor public group ran no auth middleware and saw every caller as
// anonymous. No read handler currently branches on the context user, so this
// is invisible today, but it is the intended forward-looking behaviour.
func RequireAuthOrPublicRead(authService *auth.AuthService, authCookies *AuthCookies, authDisabled bool, public publicaccess.ReadGate) gin.HandlerFunc {
	return func(c *gin.Context) {
		r := resolveSession(c, authService, authCookies, authDisabled)
		if r.user != nil {
			c.Next()
			return
		}
		if r.status == http.StatusUnauthorized && public != nil && public.Enabled() {
			c.Next() // public mode: anonymous read is allowed
			return
		}
		c.AbortWithStatusJSON(r.status, r.body)
	}
}
