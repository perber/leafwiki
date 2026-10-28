package properties

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/http/dto"
)

// Routes is the RouteRegistrar for the properties domain.
type Routes struct {
	getPropertyKeys    *GetPropertyKeysUseCase
	getPagesByProperty *GetPagesByPropertyUseCase
	authService        *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	GetPropertyKeys    *GetPropertyKeysUseCase
	GetPagesByProperty *GetPagesByPropertyUseCase
	AuthService        *coreauth.AuthService
}

// NewRoutes constructs the properties RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		getPropertyKeys:    cfg.GetPropertyKeys,
		getPagesByProperty: cfg.GetPagesByProperty,
		authService:        cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	// Registered once, gated per request so these reads can flip between
	// authenticated-only and public without a restart (see APIReadGroup).
	readGroup := ctx.APIReadGroup(r.authService)
	readGroup.GET("/properties", r.handleGetPropertyKeys)
	readGroup.GET("/properties/pages", r.handleGetPagesByProperty)
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// handleGetPropertyKeys handles GET /api/properties?q=&limit=
//
//	@Summary		List property keys
//	@Description	Lists the frontmatter property keys with the number of pages carrying each, ordered by count (descending), then by key. Nested properties use dot-joined keys such as `a.b`.
//	@Description	Access: a signed-in user (session cookie, API key or reverse-proxy identity); anonymous callers only when public access is on. With public access on, a missing or invalid session cookie is treated as anonymous instead of 401.
//	@Tags			properties
//	@Produce		json
//	@Security		BearerAuth
//	@Param			q		query		string					false	"Prefix filter on the key; trimmed and lowercased before matching"
//	@Param			limit	query		integer					false	"Maximum number of keys; values below 1 mean 50, values above 200 mean 200"	default(50)
//	@Success		200		{array}		dto.PropertyKeyCount	"Keys with page counts; empty array when nothing matches"
//	@Failure		400		{object}	dto.ErrorResponse		"limit is not an integer (code properties_invalid_limit)"
//	@Failure		401		{object}	dto.LegacyError			"No valid session while public access is off, an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		429		{object}	dto.LegacyError			"API-key attempts throttled"
//	@Failure		500		{object}	object					"Index lookup failed: the body is an ErrorResponse (code properties_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503		{object}	dto.LegacyError			"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/properties [get]
func (r *Routes) handleGetPropertyKeys(c *gin.Context) {
	filter := c.DefaultQuery("q", "")
	limitStr := c.DefaultQuery("limit", "50")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		respondWithPropertiesBadRequest(c, ErrCodePropertiesInvalidLimit, "Invalid limit value", "invalid limit value")
		return
	}

	out, err := r.getPropertyKeys.Execute(c.Request.Context(), GetPropertyKeysInput{Filter: filter, Limit: limit})
	if err != nil {
		respondWithPropertiesError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToPropertyKeyCounts(out.Keys))
}

// handleGetPagesByProperty handles GET /api/properties/pages?key=status&value=draft
//
//	@Summary		List pages by property
//	@Description	Lists the pages whose property `key` has exactly the value `value`. Key and value are matched as given (not trimmed or lowercased); each must contain a non-space character.
//	@Description	Access: a signed-in user (session cookie, API key or reverse-proxy identity); anonymous callers only when public access is on. With public access on, a missing or invalid session cookie is treated as anonymous instead of 401.
//	@Tags			properties
//	@Produce		json
//	@Security		BearerAuth
//	@Param			key		query		string				true	"Property key, for example `status` or a dot-joined nested key"
//	@Param			value	query		string				true	"Exact property value"
//	@Success		200		{array}		dto.PropertyPage	"Matching pages with all their properties; empty array when none match"
//	@Failure		400		{object}	dto.ErrorResponse	"key or value is missing or blank (code properties_missing_key or properties_missing_value)"
//	@Failure		401		{object}	dto.LegacyError		"No valid session while public access is off, an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		429		{object}	dto.LegacyError		"API-key attempts throttled"
//	@Failure		500		{object}	object				"Index lookup failed: the body is an ErrorResponse (code properties_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503		{object}	dto.LegacyError		"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/properties/pages [get]
func (r *Routes) handleGetPagesByProperty(c *gin.Context) {
	key := c.Query("key")
	value := c.Query("value")

	out, err := r.getPagesByProperty.Execute(c.Request.Context(), GetPagesByPropertyInput{Key: key, Value: value})
	if err != nil {
		respondWithPropertiesError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Pages)
}
