package mcp

import (
	"math"
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
)

// pagingSampleContent is the page body the paging tests read: nine lines, so
// a window can sit mid-page and the absolute line numbers are observable. The
// sample page carries no timestamps, tags or properties, so its header is
// exactly pagingHeader.
const (
	pagingSampleContent = "alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\neta\ntheta\niota"
	pagingHeader        = "# Paged\nID: p-1\nPath: paged\nKind: page\n\n"
)

func pagingSamplePage() *Page {
	return &Page{ID: "p-1", Title: "Paged", Path: "paged", Kind: "page", Content: pagingSampleContent}
}

// TestGetPageTool_Paging table-drives the read paging: the window [offset,
// end) with absolute 1-based line numbers over the whole document, the
// truncation note (and only when the window truncates), and the invalid-input
// rows. With no offset or lines the output must be byte-identical to the
// pre-paging rendering — the first row pins that.
func TestGetPageTool_Paging(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		wantText    string // the exact result text; empty on error rows
		wantErrPart string // the error message expected in the result; empty on success rows
		delegated   bool
	}{
		{
			name: "no offset or lines renders the whole page, byte-identical to the pre-paging output",
			args: map[string]any{"id": "p-1"},
			wantText: pagingHeader + "0001 | alpha\n0002 | beta\n0003 | gamma\n0004 | delta\n" +
				"0005 | epsilon\n0006 | zeta\n0007 | eta\n0008 | theta\n0009 | iota",
			delegated: true,
		},
		{
			name: "an offset window reads from mid-page with absolute line numbers",
			args: map[string]any{"id": "p-1", "offset": 4},
			wantText: pagingHeader + "0005 | epsilon\n0006 | zeta\n0007 | eta\n" +
				"0008 | theta\n0009 | iota",
			delegated: true,
		},
		{
			name: "offset and lines window the body and note the truncation",
			args: map[string]any{"id": "p-1", "offset": 4, "lines": 2},
			// The note carries the window's absolute line range (1-based) and
			// the page's total, separated by an en dash.
			wantText:  pagingHeader + "0005 | epsilon\n0006 | zeta\n\n(Showing lines 5–6 of 9)",
			delegated: true,
		},
		{
			name: "a window reaching the last line carries no note",
			args: map[string]any{"id": "p-1", "offset": 4, "lines": 5},
			wantText: pagingHeader + "0005 | epsilon\n0006 | zeta\n0007 | eta\n" +
				"0008 | theta\n0009 | iota",
			delegated: true,
		},
		{
			name: "lines beyond the end clamp to the last line",
			args: map[string]any{"id": "p-1", "offset": 4, "lines": 100},
			wantText: pagingHeader + "0005 | epsilon\n0006 | zeta\n0007 | eta\n" +
				"0008 | theta\n0009 | iota",
			delegated: true,
		},
		{
			// An unbounded lines argument must not wrap the add (a wrapped
			// end rendered an empty window and a nonsense note): it clamps
			// to the remaining lines and reads them all.
			name: "a lines past the page's end clamps without wrapping the add",
			args: map[string]any{"id": "p-1", "lines": math.MaxInt64},
			wantText: pagingHeader + "0001 | alpha\n0002 | beta\n0003 | gamma\n0004 | delta\n" +
				"0005 | epsilon\n0006 | zeta\n0007 | eta\n0008 | theta\n0009 | iota",
			delegated: true,
		},
		{
			name: "lines 0 reads all remaining lines",
			args: map[string]any{"id": "p-1", "offset": 2, "lines": 0},
			wantText: pagingHeader + "0003 | gamma\n0004 | delta\n0005 | epsilon\n0006 | zeta\n" +
				"0007 | eta\n0008 | theta\n0009 | iota",
			delegated: true,
		},
		{
			name: "negative lines read all remaining, like 0",
			args: map[string]any{"id": "p-1", "offset": 2, "lines": -1},
			wantText: pagingHeader + "0003 | gamma\n0004 | delta\n0005 | epsilon\n0006 | zeta\n" +
				"0007 | eta\n0008 | theta\n0009 | iota",
			delegated: true,
		},
		{
			name:        "an offset at the line past the end is invalid input",
			args:        map[string]any{"id": "p-1", "offset": 9},
			wantErrPart: "offset 9 beyond end of page (page has 9 lines)",
			delegated:   true,
		},
		{
			name:        "an offset past the end is invalid input with the page's line count",
			args:        map[string]any{"id": "p-1", "offset": 42},
			wantErrPart: "offset 42 beyond end of page (page has 9 lines)",
			delegated:   true,
		},
		{
			name:        "a negative offset is invalid input",
			args:        map[string]any{"id": "p-1", "offset": -1},
			wantErrPart: "offset must not be negative",
			delegated:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ops := &recordingOps{page: pagingSamplePage()}
			tool := newGetPageTool(discardLog, ops)
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
