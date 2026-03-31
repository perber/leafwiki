package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware/utils"
)

func writeAuthCookieError(c *gin.Context, err error, httpsMessage string, internalMessage string, logMessage string) {
	if errors.Is(err, utils.ErrHTTPSRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": httpsMessage})
		return
	}

	slog.Default().Error(logMessage, "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": internalMessage})
}

func WriteConfigAuthCookieError(c *gin.Context, err error) {
	writeAuthCookieError(
		c,
		err,
		"HTTPS is required for auth cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.",
		"Failed to issue CSRF cookie",
		"failed to issue config CSRF cookie",
	)
}
