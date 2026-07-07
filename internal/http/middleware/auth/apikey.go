package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// APIKeyConfig holds the configuration for Bearer API-key authentication.
type APIKeyConfig struct {
	Service *coreauth.APIKeyService
}

// InjectAPIKeyUser reads an "Authorization: Bearer <token>" header carrying a
// LeafWiki API key and, on success, stores the resolved user in the Gin
// context so that RequireAuth can short-circuit JWT/cookie validation — the
// same contract InjectRemoteUser uses for reverse-proxy header auth.
//
// Behaviour by case:
//   - service not configured (feature unused)      → no-op, normal auth applies
//   - user already set upstream                     → no-op, don't override
//   - no Authorization header, or the bearer value
//     isn't shaped like a LeafWiki key               → no-op (cookie/proxy auth still works)
//   - LeafWiki-shaped token, valid                   → user set in context
//   - LeafWiki-shaped token, invalid/revoked/expired → 401
func InjectAPIKeyUser(cfg APIKeyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Service == nil {
			c.Next()
			return
		}

		if _, exists := c.Get("user"); exists {
			c.Next()
			return
		}

		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" || !coreauth.LooksLikeAPIKeyToken(token) {
			c.Next()
			return
		}

		user, err := cfg.Service.Resolve(token)
		if err != nil {
			slog.Default().Warn("api key auth: rejected", "error", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired api key"})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
