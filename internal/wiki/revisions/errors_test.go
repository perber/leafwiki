package revisions

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
)

// TestMapRevisionError pins the table the REST route and every non-HTTP
// consumer (the MCP adapter) share: a store os.ErrNotExist gets
// revision_not_found with the route's fixed message, and an unknown error
// falls into the internal-error arm. The status column is what
// revisionErrorStatus answers for each mapped code — the status
// respondWithRevisionError renders for the same error.
func TestMapRevisionError(t *testing.T) {
	unknown := errors.New("disk on fire")
	tests := []struct {
		name        string
		err         error
		wantCode    string
		wantMessage string
		wantStatus  int
	}{
		{name: "not found", err: os.ErrNotExist, wantCode: ErrCodeRevisionNotFound, wantMessage: "Revision resource not found", wantStatus: http.StatusNotFound},
		{name: "wrapped not found keeps the code", err: fmt.Errorf("reading store: %w", os.ErrNotExist), wantCode: ErrCodeRevisionNotFound, wantMessage: "Revision resource not found", wantStatus: http.StatusNotFound},
		{name: "unknown error is the internal-error arm", err: unknown, wantCode: ErrCodeRevisionInternalError, wantMessage: "Revision request failed", wantStatus: http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc := MapRevisionError(tc.err)
			if loc.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", loc.Code, tc.wantCode)
			}
			if loc.Message != tc.wantMessage {
				t.Fatalf("message = %q, want %q", loc.Message, tc.wantMessage)
			}
			if status := revisionErrorStatus(loc.Code); status != tc.wantStatus {
				t.Fatalf("status for %q = %d, want %d", loc.Code, status, tc.wantStatus)
			}
		})
	}
}
