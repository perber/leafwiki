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
//	@Description	Public liveness and readiness probe; no authentication is required. A 401 or 429 is possible only when invalid authentication is supplied or API-key attempts are throttled by global middleware.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	HealthResponse	"All checks passed (ongoing search indexing still counts as healthy)"
//	@Failure		401	{object}	dto.LegacyError	"Invalid API key or session supplied"
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
