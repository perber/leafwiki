package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
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

// TestToolResult_LogsInternalErrorOnce pins the ADR-0008 swallow point: a
// failed tool answer is HTTP 200 and mcp-go never logs it, so the point
// where the internal detail stops propagating is toolResult. 500-equivalent
// codes — mcp_internal_error and a slice's own <domain>_internal_error
// (the page write path's page_internal_error, the 500-equivalent
// LocalizedError its error mapper returns for a failure below its sentinel
// table) — are logged exactly once there with the tool name, the code and
// the error itself; expected client errors (page_not_found, …) stay out of
// the logs, matching the REST slices.
func TestToolResult_LogsInternalErrorOnce(t *testing.T) {
	tests := []struct {
		name       string
		toolErr    *ToolError
		wantLogged bool
		wantCause  string // the internal detail the log line must carry, when logged
	}{
		{
			name:       "mcp_internal_error is logged with tool, code and error fields",
			toolErr:    &ToolError{Code: ErrCodeMCPInternalError, Message: "tool failed", Cause: errors.New("secret detail xyz")},
			wantLogged: true,
			wantCause:  "secret detail xyz",
		},
		{
			// The page write path reports wrapped storage failures under the
			// page slice's 500-equivalent code; a failed MCP tool answer is
			// HTTP 200, so this code must leave a log line the way the REST
			// 500 it mirrors would.
			name:       "a slice's 500-equivalent code (page_internal_error) is logged",
			toolErr:    &ToolError{Code: "page_internal_error", Message: "could not write page content", Cause: errors.New("disk detail abc")},
			wantLogged: true,
			wantCause:  "disk detail abc",
		},
		{
			name:    "a domain code is not logged",
			toolErr: &ToolError{Code: "page_not_found", Message: "Page not found"},
		},
		{
			name:    "mcp_forbidden is not logged",
			toolErr: &ToolError{Code: ErrCodeMCPForbidden, Message: "caller lacks the required role for this tool"},
		},
		{
			name:    "mcp_invalid_input is not logged",
			toolErr: &ToolError{Code: ErrCodeMCPInvalidInput, Message: "id is required"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			res := toolResult(log, "get_page", tc.toolErr)

			if !res.IsError {
				t.Fatalf("toolResult must set IsError")
			}
			wantText := tc.toolErr.Code + ": " + tc.toolErr.Message
			for _, c := range res.Content {
				if tc2, ok := c.(mcpgo.TextContent); ok && tc2.Text != wantText {
					t.Fatalf("result text = %q, want %q", tc2.Text, wantText)
				}
			}
			logged := buf.String()
			if gotLogged := strings.Contains(logged, "mcp tool failed"); gotLogged != tc.wantLogged {
				t.Fatalf("logged = %v, want %v (log output: %s)", gotLogged, tc.wantLogged, logged)
			}
			if tc.wantLogged {
				for _, want := range []string{"tool=get_page", "code=" + tc.toolErr.Code, "error="} {
					if !strings.Contains(logged, want) {
						t.Errorf("log output missing %q: %s", want, logged)
					}
				}
				if tc.wantCause != "" && !strings.Contains(logged, tc.wantCause) {
					t.Errorf("log output must carry the internal cause %q: %s", tc.wantCause, logged)
				}
			}
		})
	}
}
