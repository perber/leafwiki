package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func GetBrandingHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		branding, err := wikiInstance.GetBranding()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load branding config"})
			return
		}

		c.JSON(http.StatusOK, branding)
	}
}
