package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/shared"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

func UploadAssetHandler(w *wiki.Wiki, maxUploadSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		const maxMultipartMemory = 32 << 20 // 32 MiB in memory before spilling multipart data to disk

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

		// Parse form
		if err := c.Request.ParseMultipartForm(maxMultipartMemory); err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}

		pageID := c.Param("id")
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				slog.Default().Error("could not close file", "error", err)
			}
		}()

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		url, err := w.UploadAsset(user.ID, pageID, file, header.Filename, maxUploadSize)
		if err != nil {
			if errors.Is(err, shared.ErrFileTooLarge) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
				return
			}
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusCreated, gin.H{"file": url})
	}
}
