package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// RemoteUserConfig holds the configuration for reverse-proxy-based authentication.
type RemoteUserConfig struct {
	Enabled bool
	// HeaderName carries the identity a trusted proxy asserts (username or email,
	// resolved via UserService.GetUserByIdentifier).
	HeaderName string
	// AutoCreate, when true, provisions a new user for an identifier that has no
	// matching account instead of rejecting the request with 401. Off by default;
	// see ADR-0010 for why this is a separate opt-in from Enabled.
	AutoCreate bool
	// EmailHeaderName is an optional second header supplying the email address for
	// newly auto-created users. Only consulted when AutoCreate is true; if empty or
	// absent, a non-deliverable placeholder is synthesized instead.
	EmailHeaderName string
	// DefaultRole is the role assigned to auto-created users. Only meaningful when
	// AutoCreate is true.
	DefaultRole    string
	TrustedProxies *TrustedProxies
	// UserService is resolved on every request rather than captured once when
	// the router is built, so it automatically tracks a live restore's
	// AuthService.ReplaceUserStore swap instead of going stale — see
	// Wiki.UserService().
	UserService func() *coreauth.UserService
}

// InjectRemoteUser reads a username or email from a configured HTTP header when the
// request originates from a trusted proxy IP. On success it stores the resolved user
// in the Gin context so that RequireAuth can short-circuit JWT validation.
//
// Behaviour by case:
//   - disabled or untrusted source IP → no-op, normal auth applies
//   - trusted IP, header absent       → no-op (public endpoints remain reachable)
//   - trusted IP, header present, user found         → user set in context
//   - trusted IP, header present, user not found,
//     AutoCreate disabled                            → 401 (proxy claims unknown user)
//   - trusted IP, header present, user not found,
//     AutoCreate enabled                             → user is provisioned and set in context
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

		identifier := strings.TrimSpace(c.GetHeader(cfg.HeaderName))
		if identifier == "" {
			slog.Default().Debug("reverse proxy auth: trusted proxy sent no user header, skipping", "remote_addr", c.Request.RemoteAddr, "header", cfg.HeaderName)
			// Trusted proxy but no header — let public endpoints work normally;
			// RequireAuth will reject unauthenticated access to protected routes.
			c.Next()
			return
		}

		var user *coreauth.User
		var err error
		if cfg.AutoCreate {
			email := strings.TrimSpace(c.GetHeader(cfg.EmailHeaderName))
			user, err = cfg.UserService().GetOrCreateRemoteUser(identifier, email, cfg.DefaultRole)
			if err != nil {
				if errors.Is(err, coreauth.ErrRemoteUserEmailConflict) {
					slog.Default().Warn("reverse proxy auth: auto-create blocked, asserted email belongs to a different user", "identifier", identifier, "remote_addr", c.Request.RemoteAddr)
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "reverse proxy auth: user not found"})
					return
				}
				slog.Default().Error("reverse proxy auth: auto-create failed", "identifier", identifier, "remote_addr", c.Request.RemoteAddr, "error", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "reverse proxy auth: failed to provision user"})
				return
			}
		} else {
			user, err = cfg.UserService().GetUserByIdentifier(identifier)
			if err != nil {
				slog.Default().Warn("reverse proxy auth: user not found", "identifier", identifier, "remote_addr", c.Request.RemoteAddr)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "reverse proxy auth: user not found"})
				return
			}
		}

		c.Set("user", user)
		c.Next()
	}
}
