package search

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/http/dto"
)

// Routes is the RouteRegistrar for the search domain.
type Routes struct {
	search            *SearchUseCase
	getIndexingStatus *GetIndexingStatusUseCase
	authService       *coreauth.AuthService
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
	// Registered once, gated per request so these reads can flip between
	// authenticated-only and public without a restart (see APIReadGroup).
	readGroup := ctx.APIReadGroup(r.authService)
	readGroup.GET("/search/status", r.handleGetIndexingStatus)
	readGroup.GET("/search", r.handleSearch)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

// handleSearch handles GET /api/search?q=&tags=&offset=&limit=
//
//	@Summary		Search pages
//	@Description	Full-text search over page titles and content, or a tag-only listing when `tags` is given without `q`. With both, the text hits are filtered to pages carrying every given tag. Each hit carries a rank and an excerpt; the result also lists the tags of the matching pages as facets. `limit` and `offset` are handed to the text index as they are (no upper bound, `-1` means all rows); a tag-only search turns a `limit` below 1 into 20 and clamps a negative `offset`.
//	@Description	Access: a signed-in user (session cookie, API key or reverse-proxy identity); anonymous callers only when public access is on. With public access on, a missing or invalid session cookie is treated as anonymous instead of 401.
//	@Tags			search
//	@Produce		json
//	@Security		BearerAuth
//	@Param			q		query		string				false	"Search text; required unless `tags` is given"
//	@Param			tags	query		[]string			false	"Tags a page must all carry; repeat the parameter or separate tags with commas"	collectionFormat(multi)
//	@Param			offset	query		integer				false	"Number of hits to skip"	default(0)
//	@Param			limit	query		integer				false	"Maximum number of hits"	default(20)
//	@Success		200		{object}	dto.SearchResult	"Hits and tag facets; empty lists when nothing matches"
//	@Failure		400		{object}	dto.ErrorResponse	"Neither `q` nor a tag given (code search_missing_query), or `offset` or `limit` is not an integer (codes search_invalid_offset, search_invalid_limit)"
//	@Failure		401		{object}	dto.LegacyError		"No valid session while public access is off, an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		429		{object}	dto.LegacyError		"API-key attempts throttled"
//	@Failure		500		{object}	object				"Index lookup failed: the body is an ErrorResponse (code search_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503		{object}	dto.LegacyError		"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/search [get]
func (r *Routes) handleSearch(c *gin.Context) {
	query := c.Query("q")
	tags := queryTags(c, "tags")
	if query == "" && len(tags) == 0 {
		respondWithSearchStatusError(c, http.StatusBadRequest, ErrCodeSearchMissingQuery, "Query parameter 'q' is required", "query parameter q is required")
		return
	}

	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "20")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		respondWithSearchStatusError(c, http.StatusBadRequest, ErrCodeSearchInvalidOffset, "Invalid offset value", "invalid offset value")
		return
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		respondWithSearchStatusError(c, http.StatusBadRequest, ErrCodeSearchInvalidLimit, "Invalid limit value", "invalid limit value")
		return
	}

	out, err := r.search.Execute(c.Request.Context(), SearchInput{Query: query, Tags: tags, Offset: offset, Limit: limit})
	if err != nil {
		respondWithSearchError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToSearchResult(out.Result))
}

// handleGetIndexingStatus handles GET /api/search/status
//
//	@Summary		Get search indexing status
//	@Description	Reports whether the search index is being (re)built, how many pages were indexed and how many failed in the current or last run, and when the last run finished. `finished_at` is `0001-01-01T00:00:00Z` until the first run has finished.
//	@Description	Access: a signed-in user (session cookie, API key or reverse-proxy identity); anonymous callers only when public access is on. With public access on, a missing or invalid session cookie is treated as anonymous instead of 401.
//	@Tags			search
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	dto.IndexingStatus	"Current indexing status"
//	@Failure		401	{object}	dto.LegacyError		"No valid session while public access is off, an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		429	{object}	dto.LegacyError		"API-key attempts throttled"
//	@Failure		500	{object}	dto.LegacyError		"Authentication middleware failure; the handler itself cannot fail"
//	@Failure		503	{object}	dto.LegacyError		"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/search/status [get]
func (r *Routes) handleGetIndexingStatus(c *gin.Context) {
	out := r.getIndexingStatus.Execute(c.Request.Context())
	c.JSON(http.StatusOK, dto.ToIndexingStatus(out.Status))
}

func splitTags(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		tag := strings.TrimSpace(p)
		if tag != "" {
			result = append(result, tag)
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
