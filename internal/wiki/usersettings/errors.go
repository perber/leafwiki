package usersettings

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	coreusersettings "github.com/perber/wiki/internal/usersettings"
)

const (
	ErrCodeUserSettingsInvalidPayload = "usersettings_invalid_payload"
	ErrCodeUserSettingsLoadFailed     = "usersettings_load_failed"
	ErrCodeUserSettingsUpdateFailed   = "usersettings_update_failed"
	ErrCodeUserSettingsDeleteFailed   = "usersettings_delete_failed"
	ErrCodeUserSettingsInternalError  = "usersettings_internal_error"
)

// UserSettingsErrorResponse is the structured JSON error body returned by
// user-settings endpoints.
type UserSettingsErrorResponse struct {
	Error UserSettingsErrorDetail `json:"error"`
}

// UserSettingsErrorDetail carries the localization-ready error data.
type UserSettingsErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithUserSettingsStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, UserSettingsErrorResponse{
		Error: UserSettingsErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

// respondWithUserSettingsError maps errors to JSON responses for user-settings endpoints.
func respondWithUserSettingsError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithUserSettingsStatusError(c, userSettingsErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
		return
	}

	var vErr *sharederrors.ValidationErrors
	if errors.As(err, &vErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation_error",
			"fields": vErr.Errors,
		})
		return
	}

	respondWithUserSettingsStatusError(c, http.StatusInternalServerError, ErrCodeUserSettingsInternalError, "User settings request failed", "user settings request failed")
}

func userSettingsErrorStatus(code string) int {
	switch code {
	case ErrCodeUserSettingsInvalidPayload:
		return http.StatusBadRequest
	case ErrCodeUserSettingsLoadFailed, ErrCodeUserSettingsUpdateFailed, ErrCodeUserSettingsDeleteFailed, ErrCodeUserSettingsInternalError:
		return http.StatusInternalServerError
	case coreusersettings.ErrCodeUserSettingsStoreUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
