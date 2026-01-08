package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func UpdateBrandingHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SiteName string `json:"siteName"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		if err := wikiInstance.UpdateBranding(req.SiteName); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Return updated branding config
		branding, err := wikiInstance.GetBranding()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load branding config"})
			return
		}

		c.JSON(http.StatusOK, branding)
	}
}
