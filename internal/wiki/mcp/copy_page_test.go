package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── copy_page ───────────────────────────────────────────────────────────────

func TestCopyPageTool(t *testing.T) {
	t.Run("success renders the copied page", func(t *testing.T) {
		ops := &recordingOps{copied: &Page{
			ID: "p-9", Title: "Copied Page", Path: "s-1/copied", Kind: "page", Content: "copy body",
		}}
		tool := newCopyPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id": "p-1", "title": "Copied Page", "slug": "copied", "parentID": "s-1",
		})
		for _, want := range []string{"Copied Page", "ID: p-9", "Path: s-1/copied"} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in the copied page result:\n%s", want, text)
			}
		}
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		if ops.lastCopy.SourceID != "p-1" || ops.lastCopy.Title != "Copied Page" || ops.lastCopy.Slug != "copied" {
			t.Fatalf("delegate input = %+v, want source p-1 with title and slug", ops.lastCopy)
		}
		if ops.lastCopy.ParentID == nil || *ops.lastCopy.ParentID != "s-1" {
			t.Fatalf("ParentID = %v, want a pointer to %q", ops.lastCopy.ParentID, "s-1")
		}
	})

	t.Run("an omitted parentID copies to the root", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newCopyPageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id": "p-1", "title": "Copied", "slug": "copied",
		})
		if ops.lastCopy.ParentID != nil {
			t.Fatalf("ParentID = %v, want nil (root)", ops.lastCopy.ParentID)
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newCopyPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"title": "Copied", "slug": "copied",
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
		tool := newCopyPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id": "p-1", "slug": "copied",
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a missing slug is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newCopyPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id": "p-1", "title": "Copied",
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a slug over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newCopyPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id": "p-1", "title": "Copied", "slug": strings.Repeat("x", maxPageSlugArgLength+1),
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized slug, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized slug must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertWriteCodePassthrough(t, newCopyPageTool, map[string]any{
			"id": "p-1", "title": "Copied", "slug": "copied",
		}, "page_slug_conflict",
			func(f *recordingOps) {
				f.copyErr = sharederrors.NewLocalizedError("page_slug_conflict", "Page already exists", "page already exists", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newCopyPageTool, map[string]any{
			"id": "p-1", "title": "Copied", "slug": "copied",
		})
	})
}
