package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── restore_page_revision ──────────────────────────────────────────────────

func TestRestorePageRevisionTool(t *testing.T) {
	t.Run("delegates with the caller as actor and renders the restored page", func(t *testing.T) {
		ops := &recordingOps{restored: &Page{
			ID:      "p-1",
			Title:   "Restored Page",
			Path:    "restored",
			Kind:    "page",
			Content: "reverted body",
		}}
		tool := newRestorePageRevisionTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "revision_id": "r-1"})
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		for _, want := range []string{"# Restored Page", "0001 | reverted body"} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in result:\n%s", want, text)
			}
		}
		want := RestoreRevisionInput{Actor: Actor{ID: "u-1"}, ID: "p-1", RevisionID: "r-1"}
		if ops.lastRestore != want {
			t.Fatalf("delegated input = %+v, want %+v", ops.lastRestore, want)
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newRestorePageRevisionTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"revision_id": "r-1"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("missing revision_id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newRestorePageRevisionTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("an over-cap revision ID is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newRestorePageRevisionTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
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
		// revision_restore_revision_not_found is what the restore use case
		// itself answers for an unknown revision (pinned in
		// internal/wiki/mcp_adapter_test.go).
		assertWriteCodePassthrough(t, newRestorePageRevisionTool, map[string]any{"id": "p-1", "revision_id": "r-1"}, "revision_restore_revision_not_found",
			func(f *recordingOps) {
				f.restoreErr = sharederrors.NewLocalizedError("revision_restore_revision_not_found", "Restore revision not found", "restore revision not found", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newRestorePageRevisionTool, map[string]any{"id": "p-1", "revision_id": "r-1"})
	})
}
