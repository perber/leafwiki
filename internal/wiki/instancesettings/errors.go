package instancesettings

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/publicaccess"
)

const (
	// ErrCodeInvalidPayload is returned for a malformed request body.
	ErrCodeInvalidPayload = "instance_settings_invalid_payload"
	// ErrCodeInternal is the fallback for an unclassified failure.
	ErrCodeInternal = "instance_settings_internal_error"
)

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, errorResponse{
		Error: errorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

// respondWithError maps a domain error to a structured JSON response.
func respondWithError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithStatusError(c, statusForCode(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
		return
	}
	respondWithStatusError(c, http.StatusInternalServerError, ErrCodeInternal,
		"Instance settings request failed", "instance settings request failed")
}

func statusForCode(code string) int {
	switch code {
	case publicaccess.ErrCodeEnvManaged:
		return http.StatusConflict
	case ErrCodeInvalidPayload:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
