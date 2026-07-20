package apikeys

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for API key management.
type Routes struct {
	createAPIKey *CreateAPIKeyUseCase
	listAPIKeys  *ListAPIKeysUseCase
	revokeAPIKey *RevokeAPIKeyUseCase
	authService  *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	CreateAPIKey *CreateAPIKeyUseCase
	ListAPIKeys  *ListAPIKeysUseCase
	RevokeAPIKey *RevokeAPIKeyUseCase
	AuthService  *coreauth.AuthService
}

// NewRoutes constructs the api-keys RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		createAPIKey: cfg.CreateAPIKey,
		listAPIKeys:  cfg.ListAPIKeys,
		revokeAPIKey: cfg.RevokeAPIKey,
		authService:  cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar. All three routes are admin-only
// and require the normal cookie-authenticated session — RequireCookieSession
// enforces that API keys manage themselves through the UI, not through
// another API key (an admin-scoped key must not be able to enumerate or
// manage every key in the system).
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts
	base := ctx.Base

	authGroup := base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)

	authGroup.POST("/api-keys", authmw.RequireCookieSession(), authmw.RequireAdmin(opts.AuthDisabled), r.handleCreateAPIKey)
	authGroup.GET("/api-keys", authmw.RequireCookieSession(), authmw.RequireAdmin(opts.AuthDisabled), r.handleListAPIKeys)
	authGroup.DELETE("/api-keys/:id", authmw.RequireCookieSession(), authmw.RequireAdmin(opts.AuthDisabled), r.handleRevokeAPIKey)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleCreateAPIKey(c *gin.Context) {
	admin := authmw.MustGetUser(c)
	if admin == nil {
		return
	}

	var req struct {
		Name      string `json:"name" binding:"required"`
		UserID    string `json:"userId" binding:"required"`
		Role      string `json:"role"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithAPIKeyStatusError(c, http.StatusBadRequest, ErrCodeAPIKeyInvalidRequest, "Invalid request", "invalid request")
		return
	}

	var expiresAt *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			respondWithAPIKeyStatusError(c, http.StatusBadRequest, ErrCodeAPIKeyInvalidExpiry, "Invalid expiry date", "invalid expiry date")
			return
		}
		expiresAt = &parsed
	}

	out, err := r.createAPIKey.Execute(c.Request.Context(), CreateAPIKeyInput{
		Name: req.Name, UserID: req.UserID, Role: req.Role, ExpiresAt: expiresAt, CreatedBy: admin.ID,
	})
	if err != nil {
		respondWithAPIKeyError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"key":    apiKeyResponse(out.Key),
		"secret": out.Secret,
	})
}

func (r *Routes) handleListAPIKeys(c *gin.Context) {
	out, err := r.listAPIKeys.Execute(c.Request.Context())
	if err != nil {
		respondWithAPIKeyError(c, err)
		return
	}
	keys := make([]gin.H, len(out.Keys))
	for i, k := range out.Keys {
		keys[i] = apiKeyResponse(k)
	}
	c.JSON(http.StatusOK, keys)
}

func (r *Routes) handleRevokeAPIKey(c *gin.Context) {
	id := c.Param("id")
	if err := r.revokeAPIKey.Execute(c.Request.Context(), RevokeAPIKeyInput{ID: id}); err != nil {
		respondWithAPIKeyError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// apiKeyResponse maps a domain APIKey to its public JSON shape. KeyHash is
// deliberately never included — the store's raw struct must never reach a
// response body directly.
func apiKeyResponse(key *coreauth.APIKey) gin.H {
	resp := gin.H{
		"id":        key.ID,
		"name":      key.Name,
		"userId":    key.UserID,
		"prefix":    key.Prefix,
		"role":      key.Role,
		"createdBy": key.CreatedBy,
		"createdAt": key.CreatedAt.UTC().Format(time.RFC3339),
	}
	if key.ExpiresAt != nil {
		resp["expiresAt"] = key.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if key.LastUsedAt != nil {
		resp["lastUsedAt"] = key.LastUsedAt.UTC().Format(time.RFC3339)
	}
	if key.RevokedAt != nil {
		resp["revokedAt"] = key.RevokedAt.UTC().Format(time.RFC3339)
	}
	return resp
}
