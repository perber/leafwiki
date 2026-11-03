package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── delete_page ─────────────────────────────────────────────────────────────

func TestDeletePageTool(t *testing.T) {
	t.Run("a non-recursive delete is reported plainly", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newDeletePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1"})
		if !strings.Contains(text, "p-1") || !strings.Contains(text, "deleted") {
			t.Fatalf("expected a deletion confirmation for p-1, got %q", text)
		}
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		if ops.lastDelete.Recursive {
			t.Fatal("recursive defaulted to true, want false")
		}
	})

	t.Run("a recursive delete reports the subtree", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newDeletePageTool(discardLog, ops)
		text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "s-1", "recursive": true})
		if !strings.Contains(text, "subtree") {
			t.Fatalf("expected the subtree to be mentioned, got %q", text)
		}
		if !ops.lastDelete.Recursive {
			t.Fatal("recursive was not passed to the delegate")
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newDeletePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), nil)
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("an id over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newDeletePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": strings.Repeat("x", maxPageIDArgLength+1)})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized id, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized id must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertWriteCodePassthrough(t, newDeletePageTool, map[string]any{"id": "s-1"}, "page_has_children",
			func(f *recordingOps) {
				f.deleteErr = sharederrors.NewLocalizedError("page_has_children", "Page has children, use recursive delete", "page has children", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newDeletePageTool, map[string]any{"id": "p-1"})
	})
}
