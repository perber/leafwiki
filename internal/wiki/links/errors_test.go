package links

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func TestRespondWithLinkError_PageNotFound(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithLinkError(c, sharederrors.NewLocalizedError(
		ErrCodeLinkPageNotFound,
		"Page not found",
		"page not found",
		nil,
	))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"link_page_not_found","message":"Page not found","template":"page not found"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRespondWithLinkError_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithLinkError(c, ErrLinkServiceUnavailable)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"link_service_unavailable","message":"Link service is unavailable","template":"link service is unavailable"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRespondWithLinkError_InternalErrorIsSanitized(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithLinkError(c, errors.New("sql: database is closed"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"link_internal_error","message":"Failed to load link status","template":"failed to load link status"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
