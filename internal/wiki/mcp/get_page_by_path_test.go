package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
)

// TestGetPageByPathTool_Paging pins the shared windowed rendering through the
// route-path tool: a mid-page window carries the absolute line numbers and the
// truncation note, and an offset past the end of the page is the tool's own
// mcp_invalid_input. The full paging matrix lives in TestGetPageTool_Paging —
// both tools render through the same windowed renderer.
func TestGetPageByPathTool_Paging(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		wantText    string // the exact result text; empty on error rows
		wantErrPart string // the error message expected in the result; empty on success rows
		delegated   bool
	}{
		{
			name:      "offset and lines window the body with absolute line numbers",
			args:      map[string]any{"path": "paged", "offset": 4, "lines": 2},
			wantText:  pagingHeader + "0005 | epsilon\n0006 | zeta\n\n(Showing lines 5–6 of 9)",
			delegated: true,
		},
		{
			name:        "an offset past the end of the page is invalid input",
			args:        map[string]any{"path": "paged", "offset": 9},
			wantErrPart: "offset 9 beyond end of page (page has 9 lines)",
			delegated:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ops := &recordingOps{page: pagingSamplePage()}
			tool := newGetPageByPathTool(discardLog, ops)
			text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), tc.args)
			wantCalls := 0
			if tc.delegated {
				wantCalls = 1
			}
			if len(ops.calls) != wantCalls {
				t.Fatalf("delegated %d call(s), want %d: %v", len(ops.calls), wantCalls, ops.calls)
			}
			if tc.wantErrPart != "" {
				if !res.IsError {
					t.Fatalf("expected an errored result, got success: %s", text)
				}
				if !strings.Contains(text, ErrCodeMCPInvalidInput) {
					t.Fatalf("expected %s in the result, got %q", ErrCodeMCPInvalidInput, text)
				}
				if !strings.Contains(text, tc.wantErrPart) {
					t.Fatalf("expected %q in the result, got %q", tc.wantErrPart, text)
				}
				return
			}
			if res.IsError {
				t.Fatalf("expected a success result, got an error: %s", text)
			}
			if text != tc.wantText {
				t.Fatalf("result =\n%s\nwant:\n%s", text, tc.wantText)
			}
		})
	}
}
