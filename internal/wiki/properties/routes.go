package properties

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
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
	c.JSON(http.StatusOK, out.Keys)
}

// handleGetPagesByProperty handles GET /api/properties/pages?key=status&value=draft
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
