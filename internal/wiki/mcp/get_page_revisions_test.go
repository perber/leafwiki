package mcp

import (
	"strings"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── get_page_revisions ─────────────────────────────────────────────────────

func TestGetPageRevisionsTool(t *testing.T) {
	ctor := newGetPageRevisionsTool

	t.Run("renders the history newest first with the next cursor", func(t *testing.T) {
		ops := &recordingOps{
			revisions: []Revision{
				{ID: "r-2", Type: "content_update", AuthorID: "u-1",
					CreatedAt: time.Date(2025, 1, 3, 4, 5, 6, 0, time.UTC), Title: "T", Path: "t"},
				{ID: "r-1", Type: "restore", AuthorID: "u-2",
					CreatedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC), Title: "T", Path: "t", Summary: "rolled back"},
			},
			nextCursor: "next-cursor",
		}
		tool := newGetPageRevisionsTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-1"})
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		for _, want := range []string{
			"Revisions for page p-1 (newest first):",
			"- r-2 (content_update) by u-1 at 2025-01-03 04:05:06",
			"- r-1 (restore) by u-2 at 2025-01-02 03:04:05 — rolled back",
			"Next page cursor: next-cursor",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in result:\n%s", want, text)
			}
		}
		// The store order (newest first) must be kept, not re-sorted.
		if strings.Index(text, "r-2") > strings.Index(text, "r-1") {
			t.Fatalf("expected the newest revision first:\n%s", text)
		}
		want := ListRevisionsInput{ID: "p-1", Limit: revisionListDefaultLimit}
		if ops.lastListRevisions != want {
			t.Fatalf("delegated input = %+v, want %+v", ops.lastListRevisions, want)
		}
	})

	t.Run("an empty history is reported", func(t *testing.T) {
		ops := &recordingOps{revisions: nil}
		tool := newGetPageRevisionsTool(discardLog, ops)
		text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-1"})
		if !strings.Contains(text, "No revisions found for page p-1.") {
			t.Fatalf("expected the empty message, got %q", text)
		}
	})

	t.Run("a cursor is passed through", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageRevisionsTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-1", "cursor": "  cursor-1  "})
		if ops.lastListRevisions.Cursor != "cursor-1" {
			t.Fatalf("cursor = %q, want the trimmed %q", ops.lastListRevisions.Cursor, "cursor-1")
		}
	})

	t.Run("a limit at and over the cap is clamped to the maximum", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			limit int
			want  int
		}{
			{name: "at the cap", limit: revisionListMaxLimit, want: revisionListMaxLimit},
			{name: "over the cap", limit: revisionListMaxLimit + 1, want: revisionListMaxLimit},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ops := &recordingOps{}
				tool := newGetPageRevisionsTool(discardLog, ops)
				callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-1", "limit": tc.limit})
				if ops.lastListRevisions.Limit != tc.want {
					t.Fatalf("limit = %d, want %d", ops.lastListRevisions.Limit, tc.want)
				}
			})
		}
	})

	t.Run("a non-positive limit falls back to the default", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageRevisionsTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-1", "limit": -3})
		if ops.lastListRevisions.Limit != revisionListDefaultLimit {
			t.Fatalf("limit = %d, want the default %d", ops.lastListRevisions.Limit, revisionListDefaultLimit)
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		assertInvalidInput(t, ctor, "id")
	})

	t.Run("an id over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageRevisionsTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": strings.Repeat("x", maxPageIDArgLength+1)})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized id, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized id must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a whitespace-only id is invalid input", func(t *testing.T) {
		// The trim path: id "   " trims to "", the required-ID arm.
		ops := &recordingOps{}
		tool := newGetPageRevisionsTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "   "})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for a whitespace-only id, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("a whitespace-only id must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		// revision_internal_error is the adapter's mapping for a list error
		// that is neither a LocalizedError nor os.ErrNotExist (pinned in
		// internal/wiki/mcp_adapter_test.go).
		assertCodePassthrough(t, ctor, map[string]any{"id": "p-1"}, "revision_internal_error",
			func(f *recordingOps) {
				f.revisionsErr = sharederrors.NewLocalizedError("revision_internal_error", "Revision request failed", "revision request failed", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertRoleGate(t, ctor, map[string]any{"id": "p-1"})
	})
}
