package search

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRespondWithSearchError_WritesEnvelope pins status and body for a
// localized error and for an unexpected one, whose details must not leak.
func TestRespondWithSearchError_WritesEnvelope(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "service unavailable",
			err:        ErrSearchUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"error":{"code":"search_unavailable","message":"Search is currently unavailable","template":"search is currently unavailable"}}`,
		},
		{
			name:       "client gone gets a status and no body",
			err:        fmt.Errorf("search: %w", context.Canceled),
			wantStatus: statusClientClosedRequest,
			wantBody:   "",
		},
		{
			name:       "internal error is sanitized",
			err:        errors.New("sqlite disk I/O error"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":{"code":"search_internal_error","message":"Failed to perform search","template":"failed to perform search"}}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)

			respondWithSearchError(c, tc.err)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Body.String(); got != tc.wantBody {
				t.Fatalf("body = %s, want %s", got, tc.wantBody)
			}
		})
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
