package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware/utils"
)

func TestWriteAuthCookieError_HTTPSRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writeAuthCookieError(
		c,
		utils.ErrHTTPSRequired,
		"https guidance",
		"internal failure",
		"log message",
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	if body := rec.Body.String(); body != "{\"error\":\"https guidance\"}" {
		t.Fatalf("unexpected body %s", body)
	}
}

func TestWriteAuthCookieError_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writeAuthCookieError(
		c,
		errors.New("token generation failed"),
		"https guidance",
		"internal failure",
		"log message",
	)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	if body := rec.Body.String(); body != "{\"error\":\"internal failure\"}" {
		t.Fatalf("unexpected body %s", body)
	}
}
