package wikiresync

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeResyncAlreadyRunning = "resync_already_running"
	ErrCodeResyncInternalError  = "resync_internal_error"
)

var ErrResyncAlreadyRunning = sharederrors.NewLocalizedError(
	ErrCodeResyncAlreadyRunning,
	"A sync is already in progress",
	"a sync is already in progress",
	nil,
)

// ResyncErrorResponse is the structured JSON error body returned by resync endpoints.
type ResyncErrorResponse struct {
	Error ResyncErrorDetail `json:"error"`
}

// ResyncErrorDetail carries the localization-ready error data.
type ResyncErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithResyncStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, ResyncErrorResponse{
		Error: ResyncErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

func respondWithResyncError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithResyncStatusError(c, resyncErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
		return
	}
	respondWithResyncStatusError(c, http.StatusInternalServerError, ErrCodeResyncInternalError, "Resync request failed", "resync request failed")
}

func resyncErrorStatus(code string) int {
	switch code {
	case ErrCodeResyncAlreadyRunning:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
