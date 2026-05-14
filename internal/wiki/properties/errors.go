package properties

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodePropertiesInternal     = "properties_internal_error"
	ErrCodePropertiesMissingKey   = "properties_missing_key"
	ErrCodePropertiesMissingValue = "properties_missing_value"
	ErrCodePropertiesInvalidLimit = "properties_invalid_limit"
)

type propertiesErrorResponse struct {
	Error propertiesErrorDetail `json:"error"`
}

type propertiesErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithPropertiesError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		c.JSON(propertiesErrorStatus(loc.Code), propertiesErrorResponse{
			Error: propertiesErrorDetail{
				Code:     loc.Code,
				Message:  loc.Message,
				Template: loc.Template,
				Args:     loc.Args,
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, propertiesErrorResponse{
		Error: propertiesErrorDetail{
			Code:     ErrCodePropertiesInternal,
			Message:  "Internal server error",
			Template: "internal server error",
		},
	})
}

func respondWithPropertiesBadRequest(c *gin.Context, code, message, template string) {
	c.JSON(http.StatusBadRequest, propertiesErrorResponse{
		Error: propertiesErrorDetail{Code: code, Message: message, Template: template},
	})
}

func propertiesErrorStatus(code string) int {
	switch code {
	case ErrCodePropertiesMissingKey, ErrCodePropertiesMissingValue, ErrCodePropertiesInvalidLimit:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
