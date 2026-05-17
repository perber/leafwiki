package tags

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeTagsInternal     = "tags_internal_error"
	ErrCodeTagsMissingParam = "tags_missing_param"
	ErrCodeTagsInvalidLimit = "tags_invalid_limit"
)

type tagsErrorResponse struct {
	Error tagsErrorDetail `json:"error"`
}

type tagsErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithTagsError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		c.JSON(tagsErrorStatus(loc.Code), tagsErrorResponse{
			Error: tagsErrorDetail{
				Code:     loc.Code,
				Message:  loc.Message,
				Template: loc.Template,
				Args:     loc.Args,
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, tagsErrorResponse{
		Error: tagsErrorDetail{
			Code:     ErrCodeTagsInternal,
			Message:  "Internal server error",
			Template: "internal server error",
		},
	})
}

func respondWithTagsBadRequest(c *gin.Context, code, message, template string) {
	c.JSON(http.StatusBadRequest, tagsErrorResponse{
		Error: tagsErrorDetail{Code: code, Message: message, Template: template},
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
