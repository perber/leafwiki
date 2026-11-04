package shares

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Failed share-link lookups per client in a window, like API keys. Tokens are
// 256-bit, so this caps cost and noise rather than guessing odds.
const (
	failedTokenLimit  = 20
	failedTokenWindow = 5 * time.Minute
)

type sharedPageGetter interface {
	Execute(ctx context.Context, in GetSharedPageInput) (*GetSharedPageOutput, error)
}

// Routes is the RouteRegistrar for reading pages through share links.
type Routes struct {
	getPage sharedPageGetter
	limiter *security.KeyedLimiter
	log     *slog.Logger
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	GetPage *GetSharedPageUseCase
	Log     *slog.Logger
}

// NewRoutes constructs the shares RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	return &Routes{getPage: cfg.GetPage, limiter: newFailedTokenLimiter(), log: log.With("component", "shares")}
}

func newFailedTokenLimiter() *security.KeyedLimiter {
	return security.NewKeyedLimiter(failedTokenLimit, failedTokenWindow, true)
}

// RegisterRoutes implements RouteRegistrar. Share links are capability URLs:
// no identity, hardened responses, the token never in the access log
// (ADR-0018).
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	g := ctx.CapabilityGroup("/shared")
	g.GET("/:token/page", r.handleGetSharedRootPage)
	g.GET("/:token/pages/:id", r.handleGetSharedPage)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

// handleGetSharedRootPage handles GET /api/shared/:token/page
//
//	@Summary		Read the shared page
//	@Description	Returns the page a share link points to. The token in the path is the only authorization: no session, reverse-proxy identity or API key is used, and none is needed. Unknown, expired and revoked links get the same 404 as a page outside the link. Requires --enable-share-links.
//	@Description	Responses carry Referrer-Policy: no-referrer, X-Robots-Tag: noindex, nofollow and Cache-Control: no-store.
//	@Tags			shared
//	@Produce		json
//	@Param			token	path		string					true	"Share link token"
//	@Success		200		{object}	scopedread.PageView		"The shared page"
//	@Failure		404		{object}	dto.ErrorResponse		"Unknown, malformed, expired or revoked link, or the page no longer exists (code share_link_not_found)"
//	@Failure		429		{object}	dto.ErrorResponse		"Too many failed link lookups from this client (code share_link_too_many_attempts)"
//	@Failure		500		{object}	dto.ErrorResponse		"Reading the page failed (code share_link_internal_error)"
//	@Failure		503		{object}	dto.ErrorResponse		"Share link store unavailable during a live restore (code share_link_store_unavailable)"
//	@Router			/shared/{token}/page [get]
func (r *Routes) handleGetSharedRootPage(c *gin.Context) {
	r.serveSharedPage(c, "")
}

// handleGetSharedPage handles GET /api/shared/:token/pages/:id
//
//	@Summary		Read a page inside a share link
//	@Description	Returns a page inside the share link's scope: the shared page itself, or, for a link that includes subpages, any page below it as the tree is now. The token in the path is the only authorization. A page outside the scope gets the same 404 as an unknown link. Requires --enable-share-links.
//	@Description	Responses carry Referrer-Policy: no-referrer, X-Robots-Tag: noindex, nofollow and Cache-Control: no-store.
//	@Tags			shared
//	@Produce		json
//	@Param			token	path		string					true	"Share link token"
//	@Param			id		path		string					true	"Page ID"
//	@Success		200		{object}	scopedread.PageView		"The page"
//	@Failure		404		{object}	dto.ErrorResponse		"Unknown, malformed, expired or revoked link, or a page outside the link, unknown or deleted (code share_link_not_found)"
//	@Failure		429		{object}	dto.ErrorResponse		"Too many failed link lookups from this client (code share_link_too_many_attempts)"
//	@Failure		500		{object}	dto.ErrorResponse		"Reading the page failed (code share_link_internal_error)"
//	@Failure		503		{object}	dto.ErrorResponse		"Share link store unavailable during a live restore (code share_link_store_unavailable)"
//	@Router			/shared/{token}/pages/{id} [get]
func (r *Routes) handleGetSharedPage(c *gin.Context) {
	r.serveSharedPage(c, c.Param("id"))
}

// serveSharedPage resolves the link first, so a valid link is never
// throttled; only failed lookups count towards the per-client limit, and
// once it is reached they get 429 instead of 404. All failure causes look the
// same, so the limit reveals nothing about which links exist.
func (r *Routes) serveSharedPage(c *gin.Context, pageID string) {
	key := security.ClientKey(c)
	out, err := r.getPage.Execute(c.Request.Context(), GetSharedPageInput{Token: c.Param("token"), PageID: pageID})
	switch {
	case err == nil:
		r.limiter.NotifyResult(key, true)
		c.JSON(http.StatusOK, out.Page)
	case isNotFound(err) && !r.limiter.Allow(key):
		respondTooManyAttempts(c)
	case isNotFound(err):
		respondWithShareError(c, err)
	default:
		r.log.Error("shared page read failed", "error", err)
		respondWithShareError(c, err)
	}
}
