package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── patch_page ──────────────────────────────────────────────────────────────

// validPatchArgs is a minimal well-formed patch argument: one range replacing
// line 1.
const validPatchArgs = `[{"start":1,"end":1,"replace":"replaced"}]`

func TestPatchPageTool(t *testing.T) {
	t.Run("success renders the patched page", func(t *testing.T) {
		ops := &recordingOps{patched: &Page{
			ID:      "p-1",
			Title:   "Hello",
			Path:    "hello",
			Kind:    "page",
			Content: "replaced\nline two",
		}}
		tool := newPatchPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "patch": validPatchArgs})
		for _, want := range []string{"Hello", "ID: p-1", "0001 | replaced", "0002 | line two"} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in result:\n%s", want, text)
			}
		}
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
	})

	t.Run("the parsed ranges are passed to the delegate", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newPatchPageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id":    "p-1",
			"patch": `[{"start":2,"end":3,"replace":"a\nb"},{"start":5,"end":5,"replace":""}]`,
		})
		want := []PatchRange{
			{Start: 2, End: 3, Replace: "a\nb"},
			{Start: 5, End: 5, Replace: ""},
		}
		if len(ops.lastPatch.Ranges) != len(want) {
			t.Fatalf("Ranges = %+v, want %+v", ops.lastPatch.Ranges, want)
		}
		for i, r := range ops.lastPatch.Ranges {
			if r != want[i] {
				t.Fatalf("Ranges[%d] = %+v, want %+v", i, r, want[i])
			}
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newPatchPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"patch": validPatchArgs})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a missing patch is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newPatchPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1"})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a non-string patch (a real JSON array) is invalid input with the precise answer", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newPatchPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{
			"id":    "p-1",
			"patch": []any{map[string]any{"start": 1, "end": 1, "replace": "x"}},
		})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
		}
		if !strings.Contains(text, "patch must be a string containing a JSON array of range objects") {
			t.Fatalf("expected the non-string answer, not a misleading %q, got %q", "patch is required", text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("invalid input must not delegate, got %v", ops.calls)
		}
	})

	t.Run("malformed patch JSON is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newPatchPageTool(discardLog, ops)
		for _, bad := range []string{"not json", `"just a string"`, `{}`, `[{}]`, `[{"start":0,"end":1,"replace":"x"}]`, `[{"start":2,"end":1,"replace":"x"}]`} {
			t.Run(bad, func(t *testing.T) {
				text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), map[string]any{"id": "p-1", "patch": bad})
				if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
					t.Fatalf("expected %s for patch %q, got %q", ErrCodeMCPInvalidInput, bad, text)
				}
				if len(ops.calls) != 0 {
					t.Fatalf("invalid patch must not delegate, got %v", ops.calls)
				}
			})
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertWriteCodePassthrough(t, newPatchPageTool, map[string]any{"id": "p-1", "patch": validPatchArgs}, "page_not_found",
			func(f *recordingOps) {
				f.patchErr = sharederrors.NewLocalizedError("page_not_found", "Page not found", "page not found", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertEditorRoleGate(t, newPatchPageTool, map[string]any{"id": "p-1", "patch": validPatchArgs})
	})
}
