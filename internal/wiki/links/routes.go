package links

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the links domain.
type Routes struct {
	getLinkStatus *GetLinkStatusUseCase
	authService   *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	GetLinkStatus *GetLinkStatusUseCase
	AuthService   *coreauth.AuthService
}

// NewRoutes constructs the links RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		getLinkStatus: cfg.GetLinkStatus,
		authService:   cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	// Registered once, gated per request: RequireAuthOrPublicRead lets these
	// reads flip between authenticated-only and public without a restart.
	readGroup := ctx.Base.Group("/api")
	readGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuthOrPublicRead(r.authService, ctx.AuthCookies, opts.AuthDisabled, opts.PublicAccess),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)
	readGroup.GET("/pages/:id/links", r.handleGetLinkStatus)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleGetLinkStatus(c *gin.Context) {
	pageID := c.Param("id")
	out, err := r.getLinkStatus.Execute(c.Request.Context(), GetLinkStatusInput{PageID: pageID})
	if err != nil {
		respondWithLinkError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Status)
}
