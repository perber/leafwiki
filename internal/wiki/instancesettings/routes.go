// Package instancesettings exposes admin-only instance settings that can be
// changed at runtime instead of only via flags/env + a restart: the
// public-access ("public mode") toggle and the page display settings.
// PUT /admin/settings/display changes any display setting; the older
// per-setting routes (toc-display, content-width) stay for API clients. It
// is the natural home for future runtime-reconfigurable RouterOptions.
package instancesettings

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/pagedisplay"
	"github.com/perber/wiki/internal/publicaccess"
)

// PageDisplaySettings is what the display-settings routes need from
// internal/pagedisplay: read the current settings and change them.
// *pagedisplay.Service implements it.
type PageDisplaySettings interface {
	Settings() pagedisplay.Settings
	Update(patch pagedisplay.Patch) (pagedisplay.Settings, error)
	SetAlwaysShow(alwaysShow bool) error
	SetContentWidth(width string) error
}

// Routes is the RouteRegistrar for runtime instance settings.
type Routes struct {
	publicAccess *publicaccess.Service
	pageDisplay  PageDisplaySettings
	authService  *coreauth.AuthService
	log          *slog.Logger
}

// NewRoutes constructs the instance-settings RouteRegistrar.
func NewRoutes(publicAccess *publicaccess.Service, pageDisplay PageDisplaySettings, authService *coreauth.AuthService, log *slog.Logger) *Routes {
	if log == nil {
		log = slog.Default()
	}
	return &Routes{publicAccess: publicAccess, pageDisplay: pageDisplay, authService: authService, log: log}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	adminGroup := ctx.APIAuthGroup(r.authService).Group("/admin/settings")
	// Flipping instance-wide anonymous access is a browser-session-only
	// action, like API-key management — an API key must not be able to do it
	// (CSRF no longer blocks key-authenticated writes, so RequireCookieSession
	// is what enforces this).
	adminGroup.PUT("/public-access",
		authmw.RequireAdmin(ctx.Opts.AuthDisabled),
		authmw.RequireCookieSession(),
		r.handleSetPublicAccess,
	)
	adminGroup.PUT("/toc-display",
		authmw.RequireAdmin(ctx.Opts.AuthDisabled),
		r.handleSetTocDisplay,
	)
	adminGroup.PUT("/content-width",
		authmw.RequireAdmin(ctx.Opts.AuthDisabled),
		r.handleSetContentWidth,
	)
	// One route for every page display setting: the body is a
	// pagedisplay.Patch, so a new setting doesn't touch this package.
	adminGroup.PUT("/display",
		authmw.RequireAdmin(ctx.Opts.AuthDisabled),
		r.handleUpdateDisplaySettings,
	)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleSetPublicAccess(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		respondWithStatusError(c, http.StatusBadRequest, ErrCodeInvalidPayload, "Invalid payload", "invalid payload")
		return
	}

	if err := r.publicAccess.SetEnabled(*req.Enabled); err != nil {
		// Env-managed instances land here with publicaccess.ErrCodeEnvManaged
		// → 409, so the frontend knows to render its status-only view.
		respondWithError(c, err)
		return
	}

	r.log.Info("public access mode changed",
		"enabled", r.publicAccess.Enabled(),
		"actor_id", actorID(c),
	)
	c.JSON(http.StatusOK, gin.H{"enabled": r.publicAccess.Enabled()})
}

func (r *Routes) handleSetTocDisplay(c *gin.Context) {
	var req struct {
		AlwaysShow *bool `json:"alwaysShow"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AlwaysShow == nil {
		respondWithStatusError(c, http.StatusBadRequest, ErrCodeInvalidPayload, "Invalid payload", "invalid payload")
		return
	}

	if err := r.pageDisplay.SetAlwaysShow(*req.AlwaysShow); err != nil {
		respondWithError(c, err)
		return
	}

	r.log.Info("toc always-show setting changed",
		"always_show", r.pageDisplay.Settings().AlwaysShowToc,
		"actor_id", actorID(c),
	)
	c.JSON(http.StatusOK, gin.H{"alwaysShow": r.pageDisplay.Settings().AlwaysShowToc})
}

func (r *Routes) handleSetContentWidth(c *gin.Context) {
	var req struct {
		ContentWidth *string `json:"contentWidth"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ContentWidth == nil {
		respondWithStatusError(c, http.StatusBadRequest, ErrCodeInvalidPayload, "Invalid payload", "invalid payload")
		return
	}

	if err := r.pageDisplay.SetContentWidth(*req.ContentWidth); err != nil {
		respondWithError(c, err)
		return
	}

	// camelCase fields per ADR-0008; the older handlers above still use
	// snake_case.
	r.log.Info("page content width changed",
		"contentWidth", r.pageDisplay.Settings().ContentWidth,
		"actorID", actorID(c),
	)
	c.JSON(http.StatusOK, gin.H{"contentWidth": r.pageDisplay.Settings().ContentWidth})
}

// maxDisplaySettingsBody caps the PUT /admin/settings/display body. A full
// patch is well under 100 bytes; the cap keeps decodeStrictJSON's trailing
// data check from buffering an arbitrarily large second value.
const maxDisplaySettingsBody = 4 << 10

func (r *Routes) handleUpdateDisplaySettings(c *gin.Context) {
	// The body is a pagedisplay.Patch: its JSON names are those of
	// pagedisplay.Settings (as in /api/config), every field optional, a
	// missing or null field left unchanged.
	var patch pagedisplay.Patch
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxDisplaySettingsBody)
	if err := decodeStrictJSON(body, &patch); err != nil || patch.IsEmpty() {
		respondWithStatusError(c, http.StatusBadRequest, ErrCodeInvalidPayload, "Invalid payload", "invalid payload")
		return
	}

	settings, err := r.pageDisplay.Update(patch)
	if err != nil {
		respondWithError(c, err)
		return
	}

	r.log.Info("page display settings changed",
		"changedFields", patch.ChangedFields(),
		"actorID", actorID(c),
	)
	c.JSON(http.StatusOK, settings)
}

// decodeStrictJSON decodes exactly one JSON value into dst and rejects
// unknown fields and trailing data. gin's ShouldBindJSON can do neither per
// route: its unknown-field switch (binding.EnableDecoderDisallowUnknownFields)
// is process-global, and it ignores anything after the first value. A typo
// or an old field name (alwaysShow) must fail instead of silently changing
// nothing. It is not fully strict: encoding/json still matches field names
// case-insensitively ("AlwaysShowToc" is accepted) and a duplicate key takes
// the last value, the same as on the per-setting routes.
func decodeStrictJSON(body io.Reader, dst any) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the JSON value")
	}
	return nil
}

func actorID(c *gin.Context) string {
	if u := authmw.TryGetUser(c); u != nil {
		return u.ID
	}
	return ""
}
