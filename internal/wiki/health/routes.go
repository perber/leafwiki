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

func (r *Routes) handleHealth(c *gin.Context) {
	healthy, checks := r.health.Execute()

	status := "ok"
	code := http.StatusOK
	if !healthy {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status": status,
		"checks": checks,
	})
}
