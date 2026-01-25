package api

import (
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

		const maxUploadSize = 500 << 20 // 500 MB
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

		// Parse form
		if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
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
		defer file.Close()

		// optional: targetBasePath from form (defaults to empty string = root)
		targetBasePath := c.PostForm("targetBasePath")

		plan, err := svc.CreateImportPlanFromZipUpload(file, targetBasePath)
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

		res, err := svc.ExecuteCurrentPlan(user.ID)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, res)
	}
}

func ClearImportPlanHandler(svc *importer.ImporterService) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc.ClearCurrentPlan()
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
