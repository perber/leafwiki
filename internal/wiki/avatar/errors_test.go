package avatar

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	coreavatar "github.com/perber/wiki/internal/avatar"
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

func TestRespondWithAvatarError_InvalidDimensions_Returns400(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithAvatarError(c, sharederrors.NewLocalizedError(
		ErrCodeAvatarInvalidDimensions,
		"Avatar image dimensions too large",
		"avatar image is %sx%s pixels (allowed: up to %s pixels in total)",
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

// TestRespondWithAvatarError_InvalidDimensions_WireBody runs a header-only JPEG
// declaring 8000 x 5001 pixels through the real service and pins the error
// body (GHSA-p295-c79q-qjg7): a 400 with its own code, the template the
// locales translate, and the args filling its three placeholders in order.
func TestRespondWithAvatarError_InvalidDimensions_WireBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, err := coreavatar.NewAvatarService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAvatarService() error: %v", err)
	}
	header := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC0, 0x00, 0x0B, 8, 0x13, 0x89, 0x1F, 0x40, 1, 1, 0x11, 0, // SOF0: 5001 high, 8000 wide
		0xFF, 0xDA, 0x00, 0x08, 1, 1, 0, 0, 0x3F, 0, // SOS
	}
	uploadErr := svc.UploadAvatar("user-1", newMultipartFile(header), "avatar.jpg")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	respondWithAvatarError(c, uploadErr)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	want := `{"error":{"code":"avatar_invalid_dimensions","message":"Avatar image dimensions too large","template":"avatar image is %sx%s pixels (allowed: up to %s pixels in total)","args":["8000","5001","16777216"]}}`
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
