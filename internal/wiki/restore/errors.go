package wikirestore

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeRestoreNotEnabled      = "restore_not_enabled"
	ErrCodeRestoreNotIntervenable = "restore_not_needs_intervention"
	ErrCodeRestoreInternalError   = "restore_internal_error"
)

// RestoreErrorResponse is the structured JSON error body returned by restore endpoints.
type RestoreErrorResponse struct {
	Error RestoreErrorDetail `json:"error"`
}

// RestoreErrorDetail carries the localization-ready error data.
type RestoreErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithRestoreStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, RestoreErrorResponse{
		Error: RestoreErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

func respondWithRestoreError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithRestoreStatusError(c, restoreErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
		return
	}
	respondWithRestoreStatusError(c, http.StatusInternalServerError, ErrCodeRestoreInternalError, "Restore request failed", "restore request failed")
}

func restoreErrorStatus(code string) int {
	switch code {
	case "restore_already_running":
		return http.StatusConflict
	case ErrCodeRestoreNotIntervenable:
		return http.StatusConflict
	// The next two are surfaced as-is from snapshot.Manager.SnapshotZipPath
	// during validation — same codes/statuses as the snapshot package itself.
	case "snapshot_not_found":
		return http.StatusNotFound
	case "snapshot_invalid_id":
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
