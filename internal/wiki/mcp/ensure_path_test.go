package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── ensure_path ─────────────────────────────────────────────────────────────

func TestEnsurePathTool(t *testing.T) {
	t.Run("success renders the ensured final node", func(t *testing.T) {
		ops := &recordingOps{ensured: &Page{
			ID: "p-9", Title: "Final Page", Path: "a/b/final", Kind: "page", Content: "body line",
		}}
		tool := newEnsurePathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"path":  "/a/b/final",
			"title": "Final Page",
		})
		for _, want := range []string{"Final Page", "ID: p-9", "Path: a/b/final", "Kind: page"} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in the ensured node result:\n%s", want, text)
			}
		}
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		if ops.lastEnsure.Path != "a/b/final" {
			t.Fatalf("Path = %q, want %q", ops.lastEnsure.Path, "a/b/final")
		}
		if ops.lastEnsure.Title != "Final Page" {
			t.Fatalf("Title = %q, want %q", ops.lastEnsure.Title, "Final Page")
		}
		if ops.lastEnsure.Kind != "" {
			t.Fatalf("Kind = %q, want empty (the page default)", ops.lastEnsure.Kind)
		}
	})

	t.Run("a section kind is passed through", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"path": "a/b/final", "title": "Final", "kind": "section",
		})
		if ops.lastEnsure.Kind != "section" {
			t.Fatalf("Kind = %q, want %q", ops.lastEnsure.Kind, "section")
		}
	})

	t.Run("surrounding slashes and whitespace are trimmed", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"path": "  /a/b/final/  ", "title": "Final",
		})
		if ops.lastEnsure.Path != "a/b/final" {
			t.Fatalf("Path = %q, want %q", ops.lastEnsure.Path, "a/b/final")
		}
	})

	t.Run("a missing path is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"title": "Final"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a whitespace-only path is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"path": "///", "title": "Final",
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for a separator-only path, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a path with an empty segment is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"path": "a//b", "title": "Final",
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a missing title is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"path": "a/b/final"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a path over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"path": strings.Repeat("x", maxRoutePathArgLength+1), "title": "Final",
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized path, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized path must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a title over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"path": "a/b/final", "title": strings.Repeat("x", maxPageTitleArgLength+1),
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized title, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized title must not delegate, got %v", ops.calls)
		}
	})

	t.Run("an unknown kind is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newEnsurePathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"path": "a/b/final", "title": "Final", "kind": "note",
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an unknown kind, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an unknown kind must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertWriteCodePassthrough(t, newEnsurePathTool, map[string]any{"path": "a/b/final", "title": "Final"}, "page_slug_conflict",
			func(f *recordingOps) {
				f.ensureErr = sharederrors.NewLocalizedError("page_slug_conflict", "Page already exists", "page already exists", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newEnsurePathTool, map[string]any{"path": "a/b/final", "title": "Final"})
	})
}
