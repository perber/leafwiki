package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/importer"
	"github.com/perber/wiki/internal/wiki"
)

func CreateImportPlanHandler(svc *importer.ImporterService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		const maxUploadSize = 500 << 20 // 500 MiB (~524 MB)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

		// Parse form
		if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "upload exceeds maximum size limit of 500 MiB"})
			return
		}

		// multipart: file
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
				slog.Default().Error("could not close uploaded file", "error", err)
			}
		}()

		// optional: targetBasePath from form (defaults to empty string = root)
		targetBasePath := c.PostForm("targetBasePath")

		if _, err := svc.CreateImportPlanFromZipUpload(file, targetBasePath); err != nil {
			respondWithError(c, err)
			return
		}

		plan, err := svc.GetCurrentPlan()
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, plan)
	}
}

func GetImportPlanHandler(svc *importer.ImporterService) gin.HandlerFunc {
	return func(c *gin.Context) {
		plan, err := svc.GetCurrentPlan()
		if err != nil {
			respondWithError(c, err)
			return
		}
		c.JSON(http.StatusOK, plan)
	}
}

func ExecuteImportHandler(svc *importer.ImporterService, w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		res, started, err := svc.StartCurrentPlanExecution(user.ID)
		if err != nil {
			respondWithError(c, err)
			return
		}

		statusCode := http.StatusOK
		if started || res.ExecutionStatus == importer.ExecutionStatusRunning {
			statusCode = http.StatusAccepted
		}

		c.JSON(statusCode, res)
	}
}

func ClearImportPlanHandler(svc *importer.ImporterService) gin.HandlerFunc {
	return func(c *gin.Context) {
		state, _, err := svc.CancelCurrentPlan()
		if err == nil && state != nil && state.ExecutionStatus == importer.ExecutionStatusRunning && state.CancelRequested {
			c.JSON(http.StatusAccepted, state)
			return
		}
		if err != nil && err != importer.ErrNoPlan {
			respondWithError(c, err)
			return
		}

		if err := svc.ClearCurrentPlan(); err != nil {
			respondWithError(c, err)
			return
		}
		c.JSON(http.StatusOK, nil)
	}
}
