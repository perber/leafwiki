package properties

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/http/dto"
)

const (
	ErrCodePropertiesInternal     = "properties_internal_error"
	ErrCodePropertiesMissingKey   = "properties_missing_key"
	ErrCodePropertiesMissingValue = "properties_missing_value"
	ErrCodePropertiesInvalidLimit = "properties_invalid_limit"
)

func respondWithPropertiesError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		c.JSON(propertiesErrorStatus(loc.Code), dto.ErrorResponse{
			Error: dto.ErrorDetail{
				Code:     loc.Code,
				Message:  loc.Message,
				Template: loc.Template,
				Args:     loc.Args,
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
		Error: dto.ErrorDetail{
			Code:     ErrCodePropertiesInternal,
			Message:  "Internal server error",
			Template: "internal server error",
		},
	})
}

func respondWithPropertiesBadRequest(c *gin.Context, code, message, template string) {
	c.JSON(http.StatusBadRequest, dto.ErrorResponse{
		Error: dto.ErrorDetail{Code: code, Message: message, Template: template},
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
