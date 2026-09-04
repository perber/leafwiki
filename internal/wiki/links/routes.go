package links

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
)

// Routes is the RouteRegistrar for the links domain.
type Routes struct {
	getLinkStatus  *GetLinkStatusUseCase
	getBrokenLinks *GetBrokenLinksUseCase
	authService    *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	GetLinkStatus  *GetLinkStatusUseCase
	GetBrokenLinks *GetBrokenLinksUseCase
	AuthService    *coreauth.AuthService
}

// NewRoutes constructs the links RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		getLinkStatus:  cfg.GetLinkStatus,
		getBrokenLinks: cfg.GetBrokenLinks,
		authService:    cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	// Registered once, gated per request so this read can flip between
	// authenticated-only and public without a restart (see APIReadGroup).
	readGroup := ctx.APIReadGroup(r.authService)
	readGroup.GET("/pages/:id/links", r.handleGetLinkStatus)
	readGroup.GET("/links/broken", r.handleGetBrokenLinks)
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

func (r *Routes) handleGetBrokenLinks(c *gin.Context) {
	out, err := r.getBrokenLinks.Execute(c.Request.Context())
	if err != nil {
		respondWithLinkError(c, err)
		return
	}

	c.JSON(http.StatusOK, out)
}
