package search

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the search domain.
type Routes struct {
	search           *SearchUseCase
	getIndexingStatus *GetIndexingStatusUseCase
	authService      *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	Search            *SearchUseCase
	GetIndexingStatus *GetIndexingStatusUseCase
	AuthService       *coreauth.AuthService
}

// NewRoutes constructs the search RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		search:            cfg.Search,
		getIndexingStatus: cfg.GetIndexingStatus,
		authService:       cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	if opts.PublicAccess {
		pub := ctx.Base.Group("/api")
		pub.GET("/search/status", r.handleGetIndexingStatus)
		pub.GET("/search", r.handleSearch)
	}

	authGroup := ctx.Base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)

	if !opts.PublicAccess {
		authGroup.GET("/search/status", r.handleGetIndexingStatus)
		authGroup.GET("/search", r.handleSearch)
	}
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "20")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset value"})
		return
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit value"})
		return
	}

	out, err := r.search.Execute(c.Request.Context(), SearchInput{Query: query, Offset: offset, Limit: limit})
	if err != nil {
		respondWithSearchError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Result)
}

func (r *Routes) handleGetIndexingStatus(c *gin.Context) {
	out := r.getIndexingStatus.Execute(c.Request.Context())
	c.JSON(http.StatusOK, out.Status)
}
