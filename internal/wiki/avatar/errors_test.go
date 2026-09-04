package avatar

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func TestRespondWithAvatarError_InvalidType_Returns400(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAvatarError(c, sharederrors.NewLocalizedError(
		ErrCodeAvatarInvalidType,
		"Invalid avatar file type",
		"invalid avatar file type",
		nil,
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRespondWithAvatarError_DecodeFailed_Returns400(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAvatarError(c, sharederrors.NewLocalizedError(
		ErrCodeAvatarDecodeFailed,
		"Failed to decode avatar image",
		"failed to decode avatar image",
		nil,
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRespondWithAvatarError_UploadFailed_Returns500(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAvatarError(c, sharederrors.NewLocalizedError(
		ErrCodeAvatarUploadFailed,
		"Failed to save avatar file",
		"failed to save avatar file",
		nil,
	))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRespondWithAvatarError_DeleteFailed_Returns500(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAvatarError(c, sharederrors.NewLocalizedError(
		ErrCodeAvatarDeleteFailed,
		"Failed to delete avatar",
		"failed to delete avatar",
		nil,
	))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRespondWithAvatarError_UnknownError_Returns500(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAvatarError(c, errors.New("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got, want := rec.Body.String(), `{"error":{"code":"avatar_internal_error","message":"Avatar request failed","template":"avatar request failed"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
