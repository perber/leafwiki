package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── move_page ───────────────────────────────────────────────────────────────

func TestMovePageTool(t *testing.T) {
	t.Run("success reports the new path", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newMovePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "path": "docs/intro"})
		if !strings.Contains(text, "p-1") || !strings.Contains(text, "docs/intro") {
			t.Fatalf("expected a move confirmation with the new path, got %q", text)
		}
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		if ops.lastMove.Path != "docs/intro" {
			t.Fatalf("Path = %q, want %q", ops.lastMove.Path, "docs/intro")
		}
	})

	t.Run("surrounding slashes and whitespace are trimmed", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newMovePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "path": "  /docs/intro/  "})
		if ops.lastMove.Path != "docs/intro" {
			t.Fatalf("Path = %q, want %q", ops.lastMove.Path, "docs/intro")
		}
	})

	t.Run("an empty path moves to the root", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newMovePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1"})
		if ops.lastMove.Path != "" {
			t.Fatalf("Path = %q, want empty (root)", ops.lastMove.Path)
		}
	})

	t.Run("an empty path segment is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newMovePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "path": "docs//intro"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newMovePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"path": "docs/intro"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a path over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newMovePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "path": strings.Repeat("x", maxRoutePathArgLength+1)})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized path, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized path must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertWriteCodePassthrough(t, newMovePageTool, map[string]any{"id": "p-1", "path": "docs/intro"}, "page_circular_move",
			func(f *recordingOps) {
				f.moveErr = sharederrors.NewLocalizedError("page_circular_move", "Move would create a circular reference", "circular reference detected", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newMovePageTool, map[string]any{"id": "p-1", "path": "docs/intro"})
	})
}
