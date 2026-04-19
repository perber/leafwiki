package importer

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	coreimporter "github.com/perber/wiki/internal/importer"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

const importMaxUploadSize = 500 << 20 // 500 MiB

// Routes is the RouteRegistrar for the importer domain.
type Routes struct {
	createPlan  *CreateImportPlanUseCase
	getPlan     *GetImportPlanUseCase
	execute     *ExecuteImportUseCase
	clearPlan   *ClearImportPlanUseCase
	authService *coreauth.AuthService
	svc         *coreimporter.ImporterService
	log         *slog.Logger
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	CreatePlan  *CreateImportPlanUseCase
	GetPlan     *GetImportPlanUseCase
	Execute     *ExecuteImportUseCase
	ClearPlan   *ClearImportPlanUseCase
	AuthService *coreauth.AuthService
	Svc         *coreimporter.ImporterService
	Log         *slog.Logger
}

// NewRoutes constructs the importer RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		createPlan:  cfg.CreatePlan,
		getPlan:     cfg.GetPlan,
		execute:     cfg.Execute,
		clearPlan:   cfg.ClearPlan,
		authService: cfg.AuthService,
		svc:         cfg.Svc,
		log:         cfg.Log,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	// Let the router's configured limit override the wiki-init default.
	if r.svc != nil && opts.MaxAssetUploadSizeBytes > 0 {
		r.svc.SetAssetMaxUploadSizeBytes(opts.MaxAssetUploadSizeBytes)
	}

	authGroup := ctx.Base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)

	authGroup.POST("/import/plan", authmw.RequireEditorOrAdmin(), r.handleCreatePlan)
	authGroup.GET("/import/plan", authmw.RequireEditorOrAdmin(), r.handleGetPlan)
	authGroup.POST("/import/execute", authmw.RequireEditorOrAdmin(), r.handleExecute)
	authGroup.DELETE("/import/plan", authmw.RequireEditorOrAdmin(), r.handleClearPlan)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleCreatePlan(c *gin.Context) {
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, importMaxUploadSize)
	if err := c.Request.ParseMultipartForm(importMaxUploadSize); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "upload exceeds maximum size limit of 500 MiB"})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	file, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open uploaded file"})
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			r.log.Error("could not close uploaded import file", "error", err)
		}
	}()

	targetBasePath := c.PostForm("targetBasePath")
	out, err := r.createPlan.Execute(c.Request.Context(), CreateImportPlanInput{
		File: file, TargetBasePath: targetBasePath,
	})
	if err != nil {
		respondWithImporterError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Plan)
}

func (r *Routes) handleGetPlan(c *gin.Context) {
	out, err := r.getPlan.Execute(c.Request.Context())
	if err != nil {
		respondWithImporterError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Plan)
}

func (r *Routes) handleExecute(c *gin.Context) {
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}

	out, err := r.execute.Execute(c.Request.Context(), ExecuteImportInput{UserID: user.ID})
	if err != nil {
		respondWithImporterError(c, err)
		return
	}

	statusCode := http.StatusOK
	if out.Started || out.State.ExecutionStatus == coreimporter.ExecutionStatusRunning {
		statusCode = http.StatusAccepted
	}
	c.JSON(statusCode, out.State)
}

func (r *Routes) handleClearPlan(c *gin.Context) {
	state, err := r.clearPlan.Execute(c.Request.Context())
	if err != nil {
		respondWithImporterError(c, err)
		return
	}
	if state != nil {
		// Cancel was requested but execution is still running.
		c.JSON(http.StatusAccepted, state)
		return
	}
	c.JSON(http.StatusOK, nil)
}
