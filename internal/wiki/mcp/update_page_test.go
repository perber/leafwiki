package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── update_page ─────────────────────────────────────────────────────────────

func TestUpdatePageTool(t *testing.T) {
	t.Run("content alone delegates with a non-nil content pointer", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "content": "new body"})
		if ops.lastUpdate.Content == nil {
			t.Fatal("Content must be set when the content argument is present")
		}
		if *ops.lastUpdate.Content != "new body" {
			t.Fatalf("Content = %q, want %q", *ops.lastUpdate.Content, "new body")
		}
		if ops.lastUpdate.Title != "" {
			t.Fatalf("Title = %q, want empty (keep current)", ops.lastUpdate.Title)
		}
		if ops.lastUpdate.Slug != "" {
			t.Fatalf("Slug = %q, want empty (keep current)", ops.lastUpdate.Slug)
		}
	})

	t.Run("a slug alone delegates as a slug-only change", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "slug": "new-slug"})
		if ops.lastUpdate.Slug != "new-slug" {
			t.Fatalf("Slug = %q, want %q", ops.lastUpdate.Slug, "new-slug")
		}
		if ops.lastUpdate.Title != "" {
			t.Fatalf("Title = %q, want empty (keep current)", ops.lastUpdate.Title)
		}
		if ops.lastUpdate.Content != nil {
			t.Fatalf("Content = %v, want nil (keep current content)", ops.lastUpdate.Content)
		}
	})

	t.Run("a slug is trimmed like create_page's and copy_page's", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "slug": "  spaced  "})
		if ops.lastUpdate.Slug != "spaced" {
			t.Fatalf("Slug = %q, want the trimmed %q", ops.lastUpdate.Slug, "spaced")
		}
	})

	t.Run("an empty string clears the content", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "content": ""})
		if ops.lastUpdate.Content == nil || *ops.lastUpdate.Content != "" {
			t.Fatalf("Content = %v, want pointer to the empty string", ops.lastUpdate.Content)
		}
	})

	t.Run("title alone delegates with no content change", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "title": "New Title"})
		if ops.lastUpdate.Title != "New Title" {
			t.Fatalf("Title = %q, want %q", ops.lastUpdate.Title, "New Title")
		}
		if ops.lastUpdate.Content != nil {
			t.Fatalf("Content = %v, want nil (keep current content)", ops.lastUpdate.Content)
		}
	})

	t.Run("surrounding whitespace is trimmed from the title, like create_page trims its arguments", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "title": "  Trimmed  "})
		if ops.lastUpdate.Title != "Trimmed" {
			t.Fatalf("Title = %q, want the trimmed %q", ops.lastUpdate.Title, "Trimmed")
		}
	})

	t.Run("a whitespace-only title with no content is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "title": "   "})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("no change at all is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"content": "x"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("an id over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newUpdatePageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": strings.Repeat("x", maxPageIDArgLength+1), "title": "T"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized id, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized id must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertWriteCodePassthrough(t, newUpdatePageTool, map[string]any{"id": "p-1", "content": "x"}, "page_version_conflict",
			func(f *recordingOps) {
				f.updateErr = sharederrors.NewLocalizedError("page_version_conflict", "Page was changed by another request", "page was changed by another request", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newUpdatePageTool, map[string]any{"id": "p-1", "content": "x"})
	})
}
