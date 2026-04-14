package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeAuthDisabled            = "auth_disabled"
	ErrCodeAuthInvalidCredentials  = "auth_invalid_credentials"
	ErrCodeAuthTokenExpired        = "auth_token_expired"
	ErrCodeAuthUserNotFound        = "auth_user_not_found"
	ErrCodeAuthUserAlreadyExists   = "auth_user_already_exists"
	ErrCodeAuthInvalidRole         = "auth_invalid_role"
	ErrCodeAuthForbidden           = "auth_forbidden"
	ErrCodeAuthAdminCannotDelete   = "auth_admin_cannot_delete"
	ErrCodeAuthInternalError       = "auth_internal_error"
)

// AuthErrorResponse is the structured JSON error body returned by auth endpoints.
type AuthErrorResponse struct {
	Error AuthErrorDetail `json:"error"`
}

// AuthErrorDetail carries the localization-ready error data.
type AuthErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithAuthStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, AuthErrorResponse{
		Error: AuthErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

// respondWithAuthError is the central error handler for auth endpoints.
func respondWithAuthError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithAuthStatusError(c, authErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
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

	switch {
	case errors.Is(err, coreauth.ErrUserInvalidCredentials):
		respondWithAuthStatusError(c, http.StatusUnauthorized, ErrCodeAuthInvalidCredentials, "Invalid credentials", "invalid credentials")
	case errors.Is(err, coreauth.ErrUserNotFound):
		respondWithAuthStatusError(c, http.StatusNotFound, ErrCodeAuthUserNotFound, "User not found", "user not found")
	case errors.Is(err, coreauth.ErrUserAlreadyExists):
		respondWithAuthStatusError(c, http.StatusConflict, ErrCodeAuthUserAlreadyExists, "User already exists", "user already exists")
	case errors.Is(err, coreauth.ErrUserInvalidRole):
		respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidRole, "Invalid role", "invalid role")
	case errors.Is(err, coreauth.ErrUserAdminCannotBeDeleted):
		respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthAdminCannotDelete, "Admin user cannot be deleted", "admin user cannot be deleted")
	case errors.Is(err, ErrAuthDisabled):
		respondWithAuthStatusError(c, http.StatusForbidden, ErrCodeAuthDisabled, "Authentication is disabled", "authentication is disabled")
	default:
		respondWithAuthStatusError(c, http.StatusInternalServerError, ErrCodeAuthInternalError, err.Error(), "internal error")
	}
}

func authErrorStatus(code string) int {
	switch code {
	case ErrCodeAuthUserNotFound:
		return http.StatusNotFound
	case ErrCodeAuthInvalidCredentials, ErrCodeAuthTokenExpired:
		return http.StatusUnauthorized
	case ErrCodeAuthUserAlreadyExists:
		return http.StatusConflict
	case ErrCodeAuthInvalidRole, ErrCodeAuthAdminCannotDelete:
		return http.StatusBadRequest
	case ErrCodeAuthDisabled, ErrCodeAuthForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
