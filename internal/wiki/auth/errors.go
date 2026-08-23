package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeAuthDisabled                 = "auth_disabled"
	ErrCodeAuthInvalidCredentials       = "auth_invalid_credentials"
	ErrCodeAuthTokenExpired             = "auth_token_expired"
	ErrCodeAuthUserNotFound             = "auth_user_not_found"
	ErrCodeAuthUserAlreadyExists        = "auth_user_already_exists"
	ErrCodeAuthInvalidRole              = "auth_invalid_role"
	ErrCodeAuthForbidden                = "auth_forbidden"
	ErrCodeAuthAdminCannotDelete        = "auth_admin_cannot_delete"
	ErrCodeAuthLastAdminCannotBeDemoted = "auth_last_admin_cannot_be_demoted"
	ErrCodeAuthInternalError            = "auth_internal_error"
	ErrCodeAuthInvalidPayload           = "auth_invalid_payload"
	ErrCodeAuthCookieFailed             = "auth_cookie_failed"
	ErrCodeAuthCsrfFailed               = "auth_csrf_failed"
	ErrCodeAuthInvalidRefreshToken      = "auth_invalid_refresh_token"
	ErrCodeAuthInvalidRequest           = "auth_invalid_request"
	ErrCodeAuthAccountLocked            = "auth_account_locked"
	ErrCodeAuthTOTPInvalidCode          = "auth_totp_invalid_code"
	ErrCodeAuthTOTPChallengeInvalid     = "auth_totp_challenge_invalid"
	ErrCodeAuthTOTPNotConfigured        = "auth_totp_not_configured"
	ErrCodeAuthTOTPAlreadyEnabled       = "auth_totp_already_enabled"
	ErrCodeAuthTOTPSetupNotStarted      = "auth_totp_setup_not_started"
	ErrCodeAuthTOTPNotEnabled           = "auth_totp_not_enabled"
	ErrCodeAuthTOTPVerificationFailed   = "auth_totp_verification_failed"
	ErrCodeAuthUserStoreUnavailable     = "auth_user_store_unavailable"
	ErrCodeAuthEmailDisabled            = "auth_email_disabled"
	ErrCodeAuthTokenInvalid             = "auth_token_invalid"
	ErrCodeAuthInviteAlreadyAccepted    = "auth_invite_already_accepted"
	ErrCodeAuthEditorLimitReached       = "auth_editor_limit_reached"
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

// isUserStoreUnavailable reports whether err is the user store's own
// "suspended for live restore" LocalizedError (see errUserStoreUnavailable
// in internal/core/auth), so callers can bucket it separately from a genuine
// failure instead of miscounting it as one.
func isUserStoreUnavailable(err error) bool {
	loc, ok := sharederrors.AsLocalizedError(err)
	return ok && loc.Code == ErrCodeAuthUserStoreUnavailable
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
	case errors.Is(err, coreauth.ErrInvalidToken):
		respondWithAuthStatusError(c, http.StatusUnprocessableEntity, ErrCodeAuthInvalidRefreshToken, "Missing or invalid refresh token", "missing or invalid refresh token")
	case errors.Is(err, coreauth.ErrUserAccountLocked):
		respondWithAuthStatusError(c, http.StatusUnauthorized, ErrCodeAuthAccountLocked, "Account temporarily locked due to too many failed login attempts", "account locked")
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
	case errors.Is(err, coreauth.ErrLastAdminCannotBeDemoted):
		respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthLastAdminCannotBeDemoted, "Cannot remove admin role from the last admin user", "cannot remove admin role from the last admin user")
	case errors.Is(err, coreauth.ErrEditorLimitReached):
		respondWithAuthStatusError(c, http.StatusForbidden, ErrCodeAuthEditorLimitReached, "Editor limit reached for this plan", "editor limit reached for this plan")
	case errors.Is(err, ErrAuthDisabled):
		respondWithAuthStatusError(c, http.StatusForbidden, ErrCodeAuthDisabled, "Authentication is disabled", "authentication is disabled")
	case errors.Is(err, coreauth.ErrEmailDisabled):
		respondWithAuthStatusError(c, http.StatusForbidden, ErrCodeAuthEmailDisabled, "Email is not configured on this server", "email disabled")
	case errors.Is(err, coreauth.ErrEmailTokenInvalid):
		respondWithAuthStatusError(c, http.StatusUnprocessableEntity, ErrCodeAuthTokenInvalid, "This link is invalid or has expired", "invalid or expired token")
	case errors.Is(err, coreauth.ErrInviteAlreadyAccepted):
		respondWithAuthStatusError(c, http.StatusConflict, ErrCodeAuthInviteAlreadyAccepted, "This invite has already been accepted", "invite already accepted")
	default:
		slog.Default().Error("unhandled auth error", "error", err)
		respondWithAuthStatusError(c, http.StatusInternalServerError, ErrCodeAuthInternalError, "Authentication request failed", "authentication request failed")
	}
}

func authErrorStatus(code string) int {
	switch code {
	case ErrCodeAuthUserNotFound:
		return http.StatusNotFound
	case ErrCodeAuthInvalidCredentials, ErrCodeAuthTokenExpired:
		return http.StatusUnauthorized
	case ErrCodeAuthInvalidRefreshToken:
		return http.StatusUnprocessableEntity
	case ErrCodeAuthUserAlreadyExists:
		return http.StatusConflict
	case ErrCodeAuthInvalidRole, ErrCodeAuthAdminCannotDelete, ErrCodeAuthLastAdminCannotBeDemoted,
		ErrCodeAuthInvalidPayload, ErrCodeAuthCookieFailed, ErrCodeAuthCsrfFailed,
		ErrCodeAuthInvalidRequest:
		return http.StatusBadRequest
	case ErrCodeAuthAccountLocked:
		return http.StatusUnauthorized
	case ErrCodeAuthDisabled, ErrCodeAuthForbidden:
		return http.StatusForbidden
	case ErrCodeAuthTOTPInvalidCode:
		return http.StatusUnauthorized
	case ErrCodeAuthTOTPChallengeInvalid:
		return http.StatusUnprocessableEntity
	case ErrCodeAuthTOTPNotConfigured, ErrCodeAuthTOTPVerificationFailed, ErrCodeAuthUserStoreUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeAuthTOTPAlreadyEnabled, ErrCodeAuthInviteAlreadyAccepted:
		return http.StatusConflict
	case ErrCodeAuthTOTPSetupNotStarted, ErrCodeAuthTOTPNotEnabled:
		return http.StatusBadRequest
	case ErrCodeAuthEmailDisabled:
		return http.StatusForbidden
	case ErrCodeAuthTokenInvalid:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
