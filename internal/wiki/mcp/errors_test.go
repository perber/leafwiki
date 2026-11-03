package mcp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func TestToolErrorFrom(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantNil      bool
		wantCode     string
		wantMessage  string
		leakSentinel string // must not appear in the client-visible Message
	}{
		{
			name:    "nil input returns nil",
			err:     nil,
			wantNil: true,
		},
		{
			name:        "localized error keeps code and message",
			err:         sharederrors.NewLocalizedError("page_not_found", "Page not found", "page %s not found", nil, "abc"),
			wantCode:    "page_not_found",
			wantMessage: "Page not found",
		},
		{
			name:        "wrapped localized error still maps",
			err:         fmt.Errorf("loading page: %w", sharederrors.NewLocalizedError("page_not_found", "Page not found", "page %s not found", nil, "abc")),
			wantCode:    "page_not_found",
			wantMessage: "Page not found",
		},
		{
			name:         "validation errors become mcp_invalid_input with the field messages",
			err:          validationErrorsFixture(),
			wantCode:     ErrCodeMCPInvalidInput,
			wantMessage:  "slug: must not be empty; title: must not be empty",
			leakSentinel: "internal validation detail",
		},
		{
			name:         "unknown error becomes generic mcp_internal_error",
			err:          errors.New("secret detail xyz"),
			wantCode:     ErrCodeMCPInternalError,
			wantMessage:  "tool failed",
			leakSentinel: "secret detail xyz",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toolErrorFrom(tc.err)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("toolErrorFrom(nil) = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("toolErrorFrom(%v) = nil, want a ToolError", tc.err)
			}
			if got.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", got.Code, tc.wantCode)
			}
			if got.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", got.Message, tc.wantMessage)
			}
			if tc.leakSentinel != "" && strings.Contains(got.Message, tc.leakSentinel) {
				t.Errorf("Message leaks internal detail %q: %q", tc.leakSentinel, got.Message)
			}
			if got.Cause == nil {
				t.Error("Cause is nil; the internal error must be kept for logging")
			}
		})
	}
}

func validationErrorsFixture() *sharederrors.ValidationErrors {
	vErr := sharederrors.NewValidationErrors()
	vErr.Add("slug", "must not be empty")
	vErr.Add("title", "must not be empty")
	return vErr
}

// TestToolError_Error_KeepsInternalDetailForLogs pins the Error() output:
// it names the code and the internal cause, for logging only.
func TestToolError_Error_KeepsInternalDetailForLogs(t *testing.T) {
	toolErr := toolErrorFrom(errors.New("secret detail xyz"))
	got := toolErr.Error()
	if !strings.Contains(got, ErrCodeMCPInternalError) {
		t.Errorf("Error() = %q, want it to name the code", got)
	}
	if !strings.Contains(got, "secret detail xyz") {
		t.Errorf("Error() = %q, want it to carry the cause for logs", got)
	}
}
