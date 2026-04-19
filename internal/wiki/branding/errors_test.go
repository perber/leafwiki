package branding

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func TestRespondWithBrandingError_ValidationErrors(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	ve := sharederrors.NewValidationErrors()
	ve.Add("siteName", "site name is required")

	respondWithBrandingError(c, ve)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if got, want := rec.Body.String(), `{"error":"validation_error","fields":[{"field":"siteName","message":"site name is required"}]}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRespondWithBrandingError_LocalizedError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	err := sharederrors.NewLocalizedError(
		ErrCodeBrandingLogoInvalidType,
		"Invalid logo file type",
		"invalid logo file type %s (allowed: %s)",
		nil,
		".exe",
		".png, .svg",
	)

	respondWithBrandingError(c, err)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"branding_logo_invalid_type","message":"Invalid logo file type","template":"invalid logo file type %s (allowed: %s)","args":[".exe",".png, .svg"]}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRespondWithBrandingError_InternalErrorIsSanitized(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithBrandingError(c, errors.New("write config: permission denied"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"branding_internal_error","message":"Branding request failed","template":"branding request failed"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
