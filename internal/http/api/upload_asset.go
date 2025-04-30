package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func UploadAssetHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {

		const maxUploadSize = 500 << 20 // 50 MB
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

		// Parse form
		if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}

		pageID := c.Param("id")
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
			return
		}
		defer file.Close()

		url, err := w.UploadAsset(pageID, file, header.Filename)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusCreated, gin.H{"file": url})
	}
}
