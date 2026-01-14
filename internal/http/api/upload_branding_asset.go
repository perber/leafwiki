package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func UploadBrandingLogoHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		constraints, err := w.GetBrandingConstraints()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load branding config"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, constraints.MaxLogoSize)

		if err := c.Request.ParseMultipartForm(constraints.MaxLogoSize); err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
			return
		}
		defer file.Close()

		path, err := w.UploadBrandingLogo(file, header.Filename)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Return updated branding config
		branding, err := w.GetBranding()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load branding config"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"path": path, "branding": branding})
	}
}

func UploadBrandingFaviconHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		constraints, err := w.GetBrandingConstraints()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load branding config"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, constraints.MaxFaviconSize)

		if err := c.Request.ParseMultipartForm(constraints.MaxFaviconSize); err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
			return
		}
		defer file.Close()

		path, err := w.UploadBrandingFavicon(file, header.Filename)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Return updated branding config
		branding, err := w.GetBranding()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load branding config"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"path": path, "branding": branding})
	}
}

func DeleteBrandingFaviconHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := w.DeleteBrandingFavicon()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete favicon"})
			return
		}

		// Return updated branding config
		branding, err := w.GetBranding()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load branding config"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"branding": branding})
	}
}

func DeleteBrandingLogoHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := w.DeleteBrandingLogo()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete logo"})
			return
		}

		// Return updated branding config
		branding, err := w.GetBranding()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load branding config"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"branding": branding})
	}
}
