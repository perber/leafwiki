package mcp

import (
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ─── compare_page_revisions ─────────────────────────────────────────────────

func TestComparePageRevisionsTool(t *testing.T) {
	ctor := newComparePageRevisionsTool

	args := map[string]any{"id": "p-1", "base_revision_id": "r-1", "target_revision_id": "r-2"}

	t.Run("renders the comparison shape with both contents", func(t *testing.T) {
		ops := &recordingOps{comparison: &RevisionComparison{
			Base:           &RevisionSnapshot{Revision: &Revision{ID: "r-1"}, Content: "old line\nkeep"},
			Target:         &RevisionSnapshot{Revision: &Revision{ID: "r-2"}, Content: "new line\nkeep"},
			ContentChanged: true,
			AssetChanges: []RevisionAssetChange{
				{Name: "a.png", Status: "added"},
				{Name: "b.txt", Status: "removed"},
			},
		}}
		tool := newComparePageRevisionsTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), args)
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
		for _, want := range []string{
			"# Comparison: r-1 → r-2",
			"Content changed: yes",
			"Asset changes: a.png added; b.txt removed",
			"## Base (r-1)",
			"0001 | old line",
			"## Target (r-2)",
			"0001 | new line",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in result:\n%s", want, text)
			}
		}
		want := CompareRevisionsInput{ID: "p-1", BaseRevisionID: "r-1", TargetRevisionID: "r-2"}
		if ops.lastCompare != want {
			t.Fatalf("delegated input = %+v, want %+v", ops.lastCompare, want)
		}
	})

	t.Run("an unchanged comparison renders the flag and no asset changes", func(t *testing.T) {
		ops := &recordingOps{comparison: &RevisionComparison{
			Base:           &RevisionSnapshot{Revision: &Revision{ID: "r-1"}, Content: "same"},
			Target:         &RevisionSnapshot{Revision: &Revision{ID: "r-2"}, Content: "same"},
			ContentChanged: false,
		}}
		tool := newComparePageRevisionsTool(discardLog, ops)
		text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), args)
		if !strings.Contains(text, "Content changed: no") || !strings.Contains(text, "Asset changes: none") {
			t.Fatalf("expected the unchanged comparison, got:\n%s", text)
		}
	})

	t.Run("each missing argument is invalid input", func(t *testing.T) {
		tests := []struct {
			name string
			args map[string]any
		}{
			{name: "missing id", args: map[string]any{"base_revision_id": "r-1", "target_revision_id": "r-2"}},
			{name: "missing base revision", args: map[string]any{"id": "p-1", "target_revision_id": "r-2"}},
			{name: "missing target revision", args: map[string]any{"id": "p-1", "base_revision_id": "r-1"}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				ops := &recordingOps{}
				tool := newComparePageRevisionsTool(discardLog, ops)
				text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), tc.args)
				if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
					t.Fatalf("expected %s, got %q", ErrCodeMCPInvalidInput, text)
				}
				if len(ops.calls) != 0 {
					t.Fatalf("invalid input must not delegate, got %v", ops.calls)
				}
			})
		}
	})

	t.Run("an over-cap revision ID is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newComparePageRevisionsTool(discardLog, ops)
		oversized := map[string]any{"id": "p-1", "base_revision_id": "r-1", "target_revision_id": strings.Repeat("x", maxPageIDArgLength+1)}
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), oversized)
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized target revision ID, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized revision ID must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		// revision_not_found is the adapter's mapping for an unknown
		// revision in either slot (pinned in internal/wiki/mcp_adapter_test.go).
		assertCodePassthrough(t, ctor, args, "revision_not_found",
			func(f *recordingOps) {
				f.comparisonErr = sharederrors.NewLocalizedError("revision_not_found", "Revision not found", "revision not found", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertRoleGate(t, ctor, args)
	})
}
