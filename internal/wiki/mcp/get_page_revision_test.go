package mcp

import (
	"strings"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── get_page_revision ──────────────────────────────────────────────────────

func TestGetPageRevisionTool(t *testing.T) {
	ctor := newGetPageRevisionTool

	t.Run("renders the revision metadata, line-numbered content and assets", func(t *testing.T) {
		ops := &recordingOps{snapshot: &RevisionSnapshot{
			Revision: &Revision{
				ID:        "r-9",
				Type:      "content_update",
				AuthorID:  "u-1",
				CreatedAt: time.Date(2025, 1, 3, 4, 5, 6, 0, time.UTC),
				Title:     "Spec Page",
				Path:      "docs/spec",
				Summary:   "revised the intro",
			},
			Content: "# Title\nbody line",
			Assets:  []RevisionAsset{{Name: "a.png", SizeBytes: 1234, MIMEType: "image/png"}},
		}}
		tool := newGetPageRevisionTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-1", "revision_id": "r-9"})
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		for _, want := range []string{
			"# Revision r-9", "Type: content_update", "Author: u-1",
			"Created: 2025-01-03 04:05:06", "Title: Spec Page", "Path: docs/spec",
			"Summary: revised the intro",
			"0001 | # Title", "0002 | body line",
			"- a.png (1234 bytes, image/png)",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in result:\n%s", want, text)
			}
		}
		want := GetRevisionInput{ID: "p-1", RevisionID: "r-9"}
		if ops.lastGetRevision != want {
			t.Fatalf("delegated input = %+v, want %+v", ops.lastGetRevision, want)
		}
	})

	t.Run("a snapshot without assets omits the assets section", func(t *testing.T) {
		ops := &recordingOps{snapshot: &RevisionSnapshot{
			Revision: &Revision{ID: "r-1"},
			Content:  "plain",
		}}
		tool := newGetPageRevisionTool(discardLog, ops)
		text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-1", "revision_id": "r-1"})
		if strings.Contains(text, "## Assets") {
			t.Fatalf("expected no assets section, got:\n%s", text)
		}
	})

	t.Run("missing revision_id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageRevisionTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-1"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageRevisionTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"revision_id": "r-1"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a revision ID at the length cap delegates, over it is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageRevisionTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{
			"id": "p-1", "revision_id": strings.Repeat("x", maxPageIDArgLength),
		})
		if len(ops.calls) != 1 {
			t.Fatalf("a revision ID at the cap must delegate, got %v", ops.calls)
		}
		ops.calls = nil
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{
			"id": "p-1", "revision_id": strings.Repeat("x", maxPageIDArgLength+1),
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized revision ID, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized revision ID must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		// revision_not_found is the adapter's mapping for an unknown
		// revision (pinned in internal/wiki/mcp_adapter_test.go).
		assertCodePassthrough(t, ctor, map[string]any{"id": "p-1", "revision_id": "r-9"}, "revision_not_found",
			func(f *recordingOps) {
				f.snapshotErr = sharederrors.NewLocalizedError("revision_not_found", "Revision not found", "revision not found", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertRoleGate(t, ctor, map[string]any{"id": "p-1", "revision_id": "r-9"})
	})
}
