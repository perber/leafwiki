package links

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/http/dto"
)

const (
	ErrCodeLinkPageNotFound  = "link_page_not_found"
	ErrCodeLinkUnavailable   = "link_service_unavailable"
	ErrCodeLinkInternalError = "link_internal_error"
)

func respondWithLinkStatusError(c *gin.Context, status int, code, message, template string) {
	c.JSON(status, dto.ErrorResponse{
		Error: dto.ErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
		},
	})
}

// respondWithLinkError maps errors to JSON responses for link endpoints.
func respondWithLinkError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithLinkStatusError(c, linkErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template)
		return
	}

	respondWithLinkStatusError(c, http.StatusInternalServerError, ErrCodeLinkInternalError, "Failed to load link status", "failed to load link status")
}

func linkErrorStatus(code string) int {
	switch code {
	case ErrCodeLinkPageNotFound:
		return http.StatusNotFound
	case ErrCodeLinkUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
