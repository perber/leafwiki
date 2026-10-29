package links

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/http/dto"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
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

	// The wiki-wide broken-link audit is an admin maintenance view (the UI
	// exposes it only to admins), not a per-page read — keep it behind admin
	// auth even when the instance is in public mode.
	authGroup := ctx.APIAuthGroup(r.authService)
	authGroup.GET("/links/broken", authmw.RequireAdmin(ctx.Opts.AuthDisabled), r.handleGetBrokenLinks)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

// handleGetLinkStatus handles GET /api/pages/{id}/links
//
//	@Summary		Get link status of a page
//	@Description	Lists the links pointing at the given page (backlinks) and, separately, the links from other pages to this page's path that are broken (broken incoming links). The links starting on the page are split the same way into outgoing and broken outgoing links. The counters hold the length of each of the four lists.
//	@Description	Access: a signed-in user (session cookie, API key or reverse-proxy identity); anonymous callers only when public access is on. With public access on, a missing or invalid session cookie is treated as anonymous instead of 401.
//	@Tags			links
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string				true	"Page ID"
//	@Success		200	{object}	dto.LinkStatus		"Link status of the page; empty lists for a page without links"
//	@Failure		401	{object}	dto.LegacyError		"No valid session while public access is off, an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		404	{object}	dto.ErrorResponse	"No page with this ID (code link_page_not_found)"
//	@Failure		429	{object}	dto.LegacyError		"API-key attempts throttled"
//	@Failure		500	{object}	object				"Link lookup failed: the body is an ErrorResponse (code link_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503	{object}	dto.LegacyError		"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/pages/{id}/links [get]
func (r *Routes) handleGetLinkStatus(c *gin.Context) {
	pageID := c.Param("id")
	out, err := r.getLinkStatus.Execute(c.Request.Context(), GetLinkStatusInput{PageID: pageID})
	if err != nil {
		respondWithLinkError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToLinkStatus(out.Status))
}

// handleGetBrokenLinks handles GET /api/links/broken
//
//	@Summary		List broken links
//	@Description	Lists every link in the wiki whose target page does not exist, with the page it appears on.
//	@Description	Access: admin only, also when public access is on. API keys act with the lesser of the owner's role and the key's role. With authentication disabled the route answers 403.
//	@Tags			links
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	dto.BrokenLinks	"Broken links; an empty list when there are none"
//	@Failure		401	{object}	dto.LegacyError	"No valid session, an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		403	{object}	dto.LegacyError	"The caller is not an admin, or authentication is disabled"
//	@Failure		429	{object}	dto.LegacyError	"API-key attempts throttled"
//	@Failure		500	{object}	object			"Link lookup failed: the body is an ErrorResponse (code link_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503	{object}	dto.LegacyError	"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/links/broken [get]
func (r *Routes) handleGetBrokenLinks(c *gin.Context) {
	out, err := r.getBrokenLinks.Execute(c.Request.Context())
	if err != nil {
		respondWithLinkError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToBrokenLinks(out.Links))
}
