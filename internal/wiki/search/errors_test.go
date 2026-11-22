package search

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRespondWithSearchError_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithSearchError(c, ErrSearchUnavailable)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"search_unavailable","message":"Search is currently unavailable","template":"search is currently unavailable"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRespondWithSearchError_InternalErrorIsSanitized(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithSearchError(c, errors.New("sqlite disk I/O error"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if got, want := rec.Body.String(), `{"error":{"code":"search_internal_error","message":"Failed to perform search","template":"failed to perform search"}}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestSearchErrorStatus_MapsCodesToStatus(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{name: "unavailable", code: ErrCodeSearchUnavailable, want: http.StatusServiceUnavailable},
		{name: "missing query", code: ErrCodeSearchMissingQuery, want: http.StatusBadRequest},
		{name: "invalid offset", code: ErrCodeSearchInvalidOffset, want: http.StatusBadRequest},
		{name: "invalid limit", code: ErrCodeSearchInvalidLimit, want: http.StatusBadRequest},
		{name: "query too long", code: ErrCodeSearchQueryTooLong, want: http.StatusBadRequest},
		{name: "internal error", code: ErrCodeSearchInternal, want: http.StatusInternalServerError},
		{name: "unknown code", code: "something_else", want: http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := searchErrorStatus(tc.code); got != tc.want {
				t.Errorf("searchErrorStatus(%q) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}
