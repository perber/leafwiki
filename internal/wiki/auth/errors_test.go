package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func TestRespondWithAuthError_TOTPInvalidCode(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, sharederrors.NewLocalizedError(
		ErrCodeAuthTOTPInvalidCode,
		"Invalid authentication code",
		"invalid TOTP or recovery code",
		nil,
	))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got, want := rec.Body.String(), `{"error":{"code":"auth_totp_invalid_code","message":"Invalid authentication code","template":"invalid TOTP or recovery code"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRespondWithAuthError_TOTPChallengeInvalid(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, sharederrors.NewLocalizedError(
		ErrCodeAuthTOTPChallengeInvalid,
		"Invalid or expired login challenge",
		"invalid or expired TOTP login challenge",
		nil,
	))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestRespondWithAuthError_TOTPNotConfigured(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, sharederrors.NewLocalizedError(
		ErrCodeAuthTOTPNotConfigured,
		"Two-factor authentication is not available on this server",
		"TOTP login attempted but no TOTP encryption key is configured",
		nil,
	))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRespondWithAuthError_TOTPAlreadyEnabled(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, sharederrors.NewLocalizedError(
		ErrCodeAuthTOTPAlreadyEnabled,
		"Two-factor authentication is already enabled",
		"TOTP is already enabled for this account",
		nil,
	))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRespondWithAuthError_TOTPSetupNotStarted(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, sharederrors.NewLocalizedError(
		ErrCodeAuthTOTPSetupNotStarted,
		"Two-factor authentication setup was not started",
		"no pending TOTP setup for this account",
		nil,
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRespondWithAuthError_TOTPNotEnabled(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, sharederrors.NewLocalizedError(
		ErrCodeAuthTOTPNotEnabled,
		"Two-factor authentication is not enabled",
		"TOTP is not enabled for this account",
		nil,
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
