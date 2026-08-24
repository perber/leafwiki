package usersettings

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	coreusersettings "github.com/perber/wiki/internal/usersettings"
)

func TestRespondWithUserSettingsError_InvalidPayload_Returns400(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithUserSettingsError(c, sharederrors.NewLocalizedError(
		ErrCodeUserSettingsInvalidPayload,
		"Invalid payload",
		"invalid payload",
		nil,
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"usersettings_invalid_payload","message":"Invalid payload","template":"invalid payload"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRespondWithUserSettingsError_ValidationErrors_Returns400WithFields(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	ve := sharederrors.NewValidationErrors()
	ve.Add("language", "Language must be one of: en")
	respondWithUserSettingsError(c, ve)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if got, want := rec.Body.String(), `{"error":"validation_error","fields":[{"field":"language","message":"Language must be one of: en"}]}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

// TestRespondWithUserSettingsError_StoreUnavailable_Returns503 is the
// regression test for a real bug found in review: a store-unavailable error
// (surfaced while a live restore has usersettings.db suspended) used to fall
// through to the generic 500 default, unlike apikeys' equivalent case for
// its own store, which already maps to 503 — a client can't tell "retry
// shortly" apart from "something is broken" without this.
func TestRespondWithUserSettingsError_StoreUnavailable_Returns503(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithUserSettingsError(c, sharederrors.NewLocalizedError(
		coreusersettings.ErrCodeUserSettingsStoreUnavailable,
		"The server is restoring from a backup — please try again in a moment",
		"user settings store is suspended for an in-progress restore",
		nil,
	))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRespondWithUserSettingsError_UnknownError_Returns500(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithUserSettingsError(c, errors.New("sql: database is closed"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"usersettings_internal_error","message":"User settings request failed","template":"user settings request failed"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
