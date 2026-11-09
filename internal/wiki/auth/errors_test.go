package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
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

func TestRespondWithAuthError_TOTPVerificationFailed(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, sharederrors.NewLocalizedError(
		ErrCodeAuthTOTPVerificationFailed,
		"Two-factor authentication is temporarily unavailable",
		"failed to verify TOTP code",
		nil,
	))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRespondWithAuthError_UserStoreUnavailable(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, sharederrors.NewLocalizedError(
		ErrCodeAuthUserStoreUnavailable,
		"The server is restoring from a backup — please try again in a moment",
		"user store is suspended for an in-progress restore",
		nil,
	))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRespondWithAuthError_EditorLimitReached(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAuthError(c, coreauth.ErrEditorLimitReached)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got, want := rec.Body.String(), `{"error":{"code":"auth_editor_limit_reached","message":"Editor limit reached for this plan","template":"editor limit reached for this plan"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

// A locked account, also when the error arrives wrapped, answers with the very
// same status and body as a wrong password, so the response cannot tell the
// two apart.
func TestRespondWithAuthError_LockedAccountMatchesInvalidCredentials(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	respond := func(err error) (int, string) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		respondWithAuthError(c, err)
		return rec.Code, rec.Body.String()
	}

	wantStatus, wantBody := respond(coreauth.ErrUserInvalidCredentials)
	if wantStatus != http.StatusUnauthorized {
		t.Fatalf("invalid credentials status = %d, want %d", wantStatus, http.StatusUnauthorized)
	}

	tests := []struct {
		name string
		err  error
	}{
		{name: "locked", err: coreauth.ErrLoginThrottled},
		{name: "locked, wrapped", err: fmt.Errorf("login: %w", coreauth.ErrLoginThrottled)},
		{name: "invalid credentials, wrapped", err: fmt.Errorf("login: %w", coreauth.ErrUserInvalidCredentials)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, body := respond(tt.err)
			if status != wantStatus || body != wantBody {
				t.Fatalf("got %d %s, want %d %s", status, body, wantStatus, wantBody)
			}
		})
	}
}
