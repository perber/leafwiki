package tags

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the tags domain.
type Routes struct {
	getTags        *GetTagsUseCase
	getPagesByTags *GetPagesByTagsUseCase
	authService    *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	GetTags        *GetTagsUseCase
	GetPagesByTags *GetPagesByTagsUseCase
	AuthService    *coreauth.AuthService
}

// NewRoutes constructs the tags RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		getTags:        cfg.GetTags,
		getPagesByTags: cfg.GetPagesByTags,
		authService:    cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	if opts.PublicAccess {
		pub := ctx.Base.Group("/api")
		pub.GET("/tags", r.handleGetTags)
		pub.GET("/tags/pages", r.handleGetPagesByTags)
	}

	authGroup := ctx.Base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)

	if !opts.PublicAccess {
		authGroup.GET("/tags", r.handleGetTags)
		authGroup.GET("/tags/pages", r.handleGetPagesByTags)
	}
}

// ─── Handlers ───────────────────────────────────────────────────────────────

// handleGetTags handles GET /api/tags?q=&limit=
func (r *Routes) handleGetTags(c *gin.Context) {
	filter := c.DefaultQuery("q", "")
	limitStr := c.DefaultQuery("limit", "50")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		respondWithTagsBadRequest(c, ErrCodeTagsInvalidLimit, "Invalid limit value", "invalid limit value")
		return
	}

	out, err := r.getTags.Execute(c.Request.Context(), GetTagsInput{
		Filter:   filter,
		Selected: queryTags(c, "selected"),
		Limit:    limit,
	})
	if err != nil {
		respondWithTagsError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Tags)
}

// handleGetPagesByTags handles GET /api/tags/pages?tags=react,typescript
func (r *Routes) handleGetPagesByTags(c *gin.Context) {
	tagList, err := ValidatePagesByTagsInput(queryTags(c, "tags"))
	if err != nil {
		respondWithTagsError(c, err)
		return
	}

	out, err := r.getPagesByTags.Execute(c.Request.Context(), GetPagesByTagsInput{Tags: tagList})
	if err != nil {
		respondWithTagsError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Pages)
}

func splitTags(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func queryTags(c *gin.Context, key string) []string {
	values := c.QueryArray(key)
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, splitTags(value)...)
	}
	return result
}
