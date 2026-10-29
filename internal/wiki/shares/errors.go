package shares

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/http/dto"
	"github.com/perber/wiki/internal/sharing"
)

// The slice shares the share_link_ domain with internal/sharing, which owns
// share_link_not_found and share_link_store_unavailable.
const (
	ErrCodeShareLinkInternal        = "share_link_internal_error"
	ErrCodeShareLinkTooManyAttempts = "share_link_too_many_attempts"
)

func errShareLinkNotFound() error {
	return sharederrors.NewLocalizedError(sharing.ErrCodeShareLinkNotFound,
		"Share link not found", "share link not found", sharing.ErrShareLinkNotFound)
}

// notFoundBody is the one body every "no such link or page" answer gets, so a
// link holder cannot tell an unknown link from a page outside its scope.
var notFoundBody = dto.ErrorResponse{Error: dto.ErrorDetail{
	Code: sharing.ErrCodeShareLinkNotFound, Message: "Share link not found", Template: "share link not found",
}}

func isNotFound(err error) bool {
	loc, ok := sharederrors.AsLocalizedError(err)
	return ok && loc.Code == sharing.ErrCodeShareLinkNotFound
}

// respondWithShareError answers a failed shared read. The caller is anonymous:
// nothing but the fixed messages of known codes ever reaches the response.
func respondWithShareError(c *gin.Context, err error) {
	loc, ok := sharederrors.AsLocalizedError(err)
	switch {
	case ok && loc.Code == sharing.ErrCodeShareLinkNotFound:
		c.JSON(http.StatusNotFound, notFoundBody)
	case ok && loc.Code == sharing.ErrCodeShareLinkStoreUnavailable:
		c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: dto.ErrorDetail{
			Code: loc.Code, Message: loc.Message, Template: loc.Template,
		}})
	default:
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: dto.ErrorDetail{
			Code: ErrCodeShareLinkInternal, Message: "Internal server error", Template: "internal server error",
		}})
	}
}

func respondTooManyAttempts(c *gin.Context) {
	c.JSON(http.StatusTooManyRequests, dto.ErrorResponse{Error: dto.ErrorDetail{
		Code:     ErrCodeShareLinkTooManyAttempts,
		Message:  "Too many invalid share link attempts, please try again later",
		Template: "too many invalid share link attempts, please try again later",
	}})
}
