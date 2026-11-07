package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── convert_node ────────────────────────────────────────────────────────────

func TestConvertNodeTool(t *testing.T) {
	t.Run("success reports the conversion", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newConvertNodeTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id": "p-1", "kind": "section",
		})
		if !strings.Contains(text, "p-1") || !strings.Contains(text, "section") {
			t.Fatalf("expected a conversion confirmation with the kind, got %q", text)
		}
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		if ops.lastConvert.ID != "p-1" || ops.lastConvert.Kind != "section" {
			t.Fatalf("delegate input = %+v, want ID p-1 and kind section", ops.lastConvert)
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newConvertNodeTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"kind": "section"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("missing kind is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newConvertNodeTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("an unknown kind is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newConvertNodeTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id": "p-1", "kind": "note",
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an unknown kind, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an unknown kind must not delegate, got %v", ops.calls)
		}
	})

	t.Run("the refusal code the real adapter produces survives", func(t *testing.T) {
		// The error the real adapter answers for a section with children,
		// pinned in TestWikiMCPAdapter_ConvertNode_Disallowed. The mapper's
		// page_convert_not_allowed arm covers only an on-disk drift state the
		// adapter can't produce, so the tool-level row uses the code the
		// adapter actually produces; the mapper arm stays pinned in
		// TestMapPageError.
		assertWriteCodePassthrough(t, newConvertNodeTool, map[string]any{"id": "p-1", "kind": "page"}, "page_has_children",
			func(f *recordingOps) {
				f.convertErr = sharederrors.NewLocalizedError("page_has_children", "Page has children, use recursive delete", "page has children", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newConvertNodeTool, map[string]any{"id": "p-1", "kind": "section"})
	})
}
