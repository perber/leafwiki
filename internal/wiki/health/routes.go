package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/search"
)

type Routes struct {
	health *HealthUseCase
}

type RoutesConfig struct {
	Index      *search.SQLiteIndex
	Status     *search.IndexingStatus
	StorageDir string
}

func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		health: NewHealthUseCase(cfg.Index, cfg.Status, cfg.StorageDir),
	}
}

func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	ctx.Base.GET("/api/health", r.handleHealth)
}

// handleHealth reports the state of the SQLite index, the data directory and search indexing.
//
//	@Summary		Check instance health
//	@Description	Public liveness and readiness probe; no authentication is required and session cookies are never validated. Global middleware can still reject a request before the handler runs: 401 for an invalid, revoked or expired API key (only when API-key management is enabled) or for an identity asserted by a trusted reverse proxy that LeafWiki rejects (only when reverse-proxy authentication is enabled), and 429 when API-key attempts are throttled.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	HealthResponse	"All checks passed (ongoing search indexing still counts as healthy)"
//	@Failure		401	{object}	dto.LegacyError	"Invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity: unknown user or auto-provisioning email conflict (when reverse-proxy authentication is enabled)"
//	@Failure		429	{object}	dto.LegacyError	"API-key attempts throttled"
//	@Failure		503	{object}	object			"Degraded: the body is a HealthResponse with failed checks. If API-key resolution cannot reach its store, the body is a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Router			/health [get]
func (r *Routes) handleHealth(c *gin.Context) {
	healthy, checks := r.health.Execute()

	status := "ok"
	code := http.StatusOK
	if !healthy {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, HealthResponse{Status: status, Checks: checks})
}
