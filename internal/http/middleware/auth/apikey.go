package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// APIKeyConfig holds the configuration for Bearer API-key authentication.
type APIKeyConfig struct {
	Service *coreauth.APIKeyService
	// RateLimiter throttles repeated failed Bearer-auth attempts per client
	// IP (a valid key resets its own count — see NotifyResult). Optional;
	// nil disables rate limiting for this path.
	RateLimiter *security.KeyedLimiter
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
//   - LeafWiki-shaped token, rate limited            → 429
//   - LeafWiki-shaped token, valid                   → user set in context
//   - LeafWiki-shaped token, invalid/revoked/expired → 401
//
// Note on writes: every feature's authGroup also runs CSRFMiddleware, which
// requires a CSRF cookie a pure Bearer client never has — so today, every
// mutating (POST/PUT/PATCH/DELETE) request made with an API key is rejected
// regardless of the key's role. This is expected for the read-only phase this
// feature currently supports; it is not itself an access-control mechanism.
// A future phase adding API-key write support must address CSRF for Bearer
// requests explicitly rather than assuming it already works.
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

		var limiterKey string
		if cfg.RateLimiter != nil {
			limiterKey = security.ClientKey(c)
			if !cfg.RateLimiter.Allow(limiterKey) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many api key attempts, please try again later"})
				return
			}
		}

		user, err := cfg.Service.Resolve(token)
		if cfg.RateLimiter != nil {
			cfg.RateLimiter.NotifyResult(limiterKey, err == nil)
		}
		if err != nil {
			slog.Default().Warn("api key auth: rejected", "error", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired api key"})
			return
		}

		c.Set("user", user)
		c.Set("apiKeyAuth", true)
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
