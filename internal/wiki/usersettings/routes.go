package usersettings

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the user-settings domain.
type Routes struct {
	getUserSettings    *GetUserSettingsUseCase
	updateUserSettings *UpdateUserSettingsUseCase
	authService        *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	GetUserSettings    *GetUserSettingsUseCase
	UpdateUserSettings *UpdateUserSettingsUseCase
	AuthService        *coreauth.AuthService
}

// NewRoutes constructs the user-settings RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		getUserSettings:    cfg.GetUserSettings,
		updateUserSettings: cfg.UpdateUserSettings,
		authService:        cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar. Any authenticated user manages
// only their own settings — there is no admin override.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts
	authGroup := ctx.Base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)
	authGroup.GET("/user-settings", r.handleGetUserSettings)
	authGroup.PUT("/user-settings", r.handleUpdateUserSettings)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleGetUserSettings(c *gin.Context) {
	user := authmw.MustGetUser(c)
	settings, err := r.getUserSettings.Execute(c.Request.Context(), user.ID)
	if err != nil {
		respondWithUserSettingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (r *Routes) handleUpdateUserSettings(c *gin.Context) {
	var req struct {
		Language   *string `json:"language"`
		AutoSave   *bool   `json:"autoSave"`
		DateFormat *string `json:"dateFormat"`
		TimeFormat *string `json:"timeFormat"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithUserSettingsStatusError(c, http.StatusBadRequest, ErrCodeUserSettingsInvalidPayload, "Invalid payload", "invalid payload")
		return
	}

	user := authmw.MustGetUser(c)
	settings, err := r.updateUserSettings.Execute(c.Request.Context(), UpdateUserSettingsInput{
		UserID:     user.ID,
		Language:   req.Language,
		AutoSave:   req.AutoSave,
		DateFormat: req.DateFormat,
		TimeFormat: req.TimeFormat,
	})
	if err != nil {
		respondWithUserSettingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}
