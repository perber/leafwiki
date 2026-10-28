package tags

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/http/dto"
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
	// Registered once, gated per request so these reads can flip between
	// authenticated-only and public without a restart (see APIReadGroup).
	readGroup := ctx.APIReadGroup(r.authService)
	readGroup.GET("/tags", r.handleGetTags)
	readGroup.GET("/tags/pages", r.handleGetPagesByTags)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

// handleGetTags handles GET /api/tags?q=&limit=&selected=
//
//	@Summary		List tags
//	@Description	Lists tags with the number of pages carrying each, ordered by count (descending), then by tag. With `selected`, only pages carrying every selected tag are counted and the selected tags themselves are left out, so the result is the tags that can narrow the selection further.
//	@Description	Access: a signed-in user (session cookie, API key or reverse-proxy identity); anonymous callers only when public access is on. With public access on, a missing or invalid session cookie is treated as anonymous instead of 401.
//	@Tags			tags
//	@Produce		json
//	@Security		BearerAuth
//	@Param			q			query		string			false	"Prefix filter on the tag name; trimmed and lowercased before matching"
//	@Param			limit		query		integer			false	"Maximum number of tags; values below 1 mean 50, values above 200 mean 200"	default(50)
//	@Param			selected	query		[]string		false	"Already selected tags; repeat the parameter or separate tags with commas"	collectionFormat(multi)
//	@Success		200			{array}		dto.TagCount	"Tags with page counts; empty array when nothing matches"
//	@Failure		400			{object}	dto.ErrorResponse	"limit is not an integer (code tags_invalid_limit)"
//	@Failure		401			{object}	dto.LegacyError		"No valid session while public access is off, an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		429			{object}	dto.LegacyError		"API-key attempts throttled"
//	@Failure		500			{object}	object				"Index lookup failed: the body is an ErrorResponse (code tags_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503			{object}	dto.LegacyError		"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/tags [get]
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
	c.JSON(http.StatusOK, dto.ToTagCounts(out.Tags))
}

// handleGetPagesByTags handles GET /api/tags/pages?tags=react,typescript
//
//	@Summary		List pages by tags
//	@Description	Lists the pages that carry every given tag. Tags are trimmed and lowercased before matching.
//	@Description	Access: a signed-in user (session cookie, API key or reverse-proxy identity); anonymous callers only when public access is on. With public access on, a missing or invalid session cookie is treated as anonymous instead of 401.
//	@Tags			tags
//	@Produce		json
//	@Security		BearerAuth
//	@Param			tags	query		[]string			true	"Tags a page must all carry; repeat the parameter or separate tags with commas"	collectionFormat(multi)
//	@Success		200		{array}		dto.TaggedPage		"Matching pages; empty array when none match"
//	@Failure		400		{object}	dto.ErrorResponse	"No non-empty tag given (code tags_missing_param)"
//	@Failure		401		{object}	dto.LegacyError		"No valid session while public access is off, an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		429		{object}	dto.LegacyError		"API-key attempts throttled"
//	@Failure		500		{object}	object				"Index lookup failed: the body is an ErrorResponse (code tags_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503		{object}	dto.LegacyError		"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/tags/pages [get]
func (r *Routes) handleGetPagesByTags(c *gin.Context) {
	tagList := queryTags(c, "tags")
	if len(tagList) == 0 {
		respondWithTagsBadRequest(c, ErrCodeTagsMissingParam, "Query parameter 'tags' is required", "query parameter tags is required")
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
