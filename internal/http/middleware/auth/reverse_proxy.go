package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// RemoteUserConfig holds the configuration for reverse-proxy-based authentication.
type RemoteUserConfig struct {
	Enabled        bool
	HeaderName     string
	TrustedProxies *TrustedProxies
	UserService    *coreauth.UserService
}

const (
	ContextAuthSource    = "authSource"
	AuthSourceRemoteUser = "remote_user"
)

// InjectRemoteUser reads a username from a configured HTTP header when the request
// originates from a trusted proxy IP. On success it stores the resolved user in the
// Gin context so that RequireAuth can short-circuit JWT validation.
//
// Behaviour by case:
//   - disabled or untrusted source IP → no-op, normal auth applies
//   - trusted IP, header absent       → no-op (public endpoints remain reachable)
//   - trusted IP, header present, user found     → user set in context
//   - trusted IP, header present, user not found → 401 (proxy claims unknown user)
func InjectRemoteUser(cfg RemoteUserConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		if cfg.TrustedProxies == nil || cfg.UserService == nil {
			slog.Default().Error("reverse proxy auth: misconfigured, TrustedProxies or UserService is nil")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Reverse proxy authentication misconfigured"})
			return
		}

		if !cfg.TrustedProxies.IsTrusted(c.Request.RemoteAddr) {
			slog.Default().Debug("reverse proxy auth: request from untrusted IP, skipping", "remote_addr", c.Request.RemoteAddr)
			c.Next()
			return
		}

		username := strings.TrimSpace(c.GetHeader(cfg.HeaderName))
		if username == "" {
			slog.Default().Debug("reverse proxy auth: trusted proxy sent no user header, skipping", "remote_addr", c.Request.RemoteAddr, "header", cfg.HeaderName)
			// Trusted proxy but no header — let public endpoints work normally;
			// RequireAuth will reject unauthenticated access to protected routes.
			c.Next()
			return
		}

		user, err := cfg.UserService.GetUserByUsername(username)
		if err != nil {
			slog.Default().Warn("reverse proxy auth: user not found", "username", username, "remote_addr", c.Request.RemoteAddr)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "reverse proxy auth: user not found"})
			return
		}

		c.Set("user", user)
		c.Set(ContextAuthSource, AuthSourceRemoteUser)
		c.Next()
	}
}

func IsRemoteUser(c *gin.Context) bool {
	value, ok := c.Get(ContextAuthSource)
	return ok && value == AuthSourceRemoteUser
}
