package utils

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

var ErrHTTPSRequired = errors.New("https is required for secure cookies")

func RequireSecure(c *gin.Context, allowInsecure bool) (bool, error) {
	if c.Request.TLS != nil {
		return true, nil
	}
	xfp := strings.ToLower(c.GetHeader("X-Forwarded-Proto"))
	if strings.Contains(xfp, "https") {
		return true, nil
	}

	if strings.EqualFold(c.GetHeader("X-Forwarded-Ssl"), "on") {
		return true, nil
	}

	if strings.EqualFold(c.GetHeader("Front-End-Https"), "on") {
		return true, nil
	}
	if allowInsecure {
		return false, nil
	}
	return false, ErrHTTPSRequired
}
