package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── create_page ────────────────────────────────────────────────────────────

func TestCreatePageTool(t *testing.T) {
	t.Run("success renders the created page", func(t *testing.T) {
		ops := &recordingOps{created: &Page{
			ID:      "p-new",
			Title:   "Created",
			Path:    "created",
			Kind:    "page",
			Content: "fresh body",
		}}
		tool := newCreatePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"title":   "Created",
			"content": "fresh body",
		})
		for _, want := range []string{"Created", "ID: p-new", "Path: created", "0001 | fresh body"} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in result:\n%s", want, text)
			}
		}
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
	})

	t.Run("a non-empty parentID is passed as a pointer, an empty one is not", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newCreatePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"title": "T", "parentID": "s-1"})
		if ops.lastCreate.ParentID == nil || *ops.lastCreate.ParentID != "s-1" {
			t.Fatalf("ParentID = %v, want pointer to %q", ops.lastCreate.ParentID, "s-1")
		}
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"title": "T"})
		if ops.lastCreate.ParentID != nil {
			t.Fatalf("ParentID = %v, want nil when parentID is empty", ops.lastCreate.ParentID)
		}
	})

	t.Run("missing title is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newCreatePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"content": "x"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("an unknown kind is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newCreatePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"title": "T", "kind": "note"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for kind %q, got %q", ErrCodeMCPInvalidInput, "note", text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a title at the length cap delegates, over the cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newCreatePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"title": strings.Repeat("x", maxPageTitleArgLength)})
		if len(ops.calls) != 1 {
			t.Fatalf("a title at the cap must delegate, got %v", ops.calls)
		}
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"title": strings.Repeat("x", maxPageTitleArgLength+1)})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized title, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 1 {
			t.Fatalf("an oversized title must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		// page_not_found is the code the real adapter produces for a missing
		// parent (the REST create route answers the same code).
		assertWriteCodePassthrough(t, newCreatePageTool, map[string]any{"title": "T"}, "page_not_found",
			func(f *recordingOps) {
				f.createErr = sharederrors.NewLocalizedError("page_not_found", "Page not found", "page not found", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newCreatePageTool, map[string]any{"title": "T"})
	})
}
