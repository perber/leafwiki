package tags

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/http/dto"
)

const (
	ErrCodeTagsInternal     = "tags_internal_error"
	ErrCodeTagsMissingParam = "tags_missing_param"
	ErrCodeTagsInvalidLimit = "tags_invalid_limit"
)

func respondWithTagsError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		c.JSON(tagsErrorStatus(loc.Code), dto.ErrorResponse{
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
			Code:     ErrCodeTagsInternal,
			Message:  "Internal server error",
			Template: "internal server error",
		},
	})
}

func respondWithTagsBadRequest(c *gin.Context, code, message, template string) {
	c.JSON(http.StatusBadRequest, dto.ErrorResponse{
		Error: dto.ErrorDetail{Code: code, Message: message, Template: template},
	})
}

func tagsErrorStatus(code string) int {
	switch code {
	case ErrCodeTagsMissingParam, ErrCodeTagsInvalidLimit:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
