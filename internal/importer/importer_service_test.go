package importer

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/tree"
)

// --- Helpers ----------------------------------------------------------------

func mustWrite(t *testing.T, base, rel, content string) string {
	t.Helper()
	abs := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return abs
}

func newServiceWithFakeWiki(t *testing.T, w *fakeWiki) *ImporterService {
	t.Helper()
	planner := NewPlanner(w, tree.NewSlugService())
	store := NewPlanStore()
	return &ImporterService{
		planner:   planner,
		planStore: store,
		extractor: NewZipExtractor(), // unused in these tests
		logger:    slog.Default().With("component", "ImporterServiceTest"),
	}
}

// --- Tests ------------------------------------------------------------------

func TestImporterService_createImportPlanFromFolder_StoresPlan(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, tmp, "a.md", "# A\nbody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(tmp)
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if plan == nil || len(plan.Items) != 1 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	// plan should have correct options

	if _, err := is.GetCurrentPlan(); err != nil {
		t.Fatalf("GetCurrentPlan err: %v", err)
	}
}

func TestImporterService_createImportPlanFromFolder_CleansUpOldWorkspace(t *testing.T) {
	// old workspace with a marker file
	oldWS := t.TempDir()
	marker := mustWrite(t, oldWS, "marker.txt", "x")

	// new workspace with md
	newWS := t.TempDir()
	mustWrite(t, newWS, "b.md", "# B")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	// seed old plan in store
	is.planStore.Set(&StoredPlan{
		Plan:          &PlanResult{ID: "old", TreeHash: "h1"},
		PlanOptions:   PlanOptions{SourceBasePath: oldWS},
		WorkspaceRoot: oldWS,
		CreatedAt:     time.Now(),
	})

	_, err := is.createImportPlanFromFolder(newWS)
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}

	// old workspace should be removed
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("expected old workspace removed; statErr=%v", statErr)
	}

	// store should now point to new workspace

	if _, err := is.GetCurrentPlan(); err != nil {
		t.Fatalf("GetCurrentPlan err: %v", err)
	}
}

func TestImporterService_GetCurrentPlan_NoPlan(t *testing.T) {
	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	_, err := is.GetCurrentPlan()
	if !errors.Is(err, ErrNoPlan) {
		t.Fatalf("expected ErrNoPlan, got %v", err)
	}
}

func TestImporterService_ClearCurrentPlan(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, tmp, "a.md", "# A")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	_, err := is.createImportPlanFromFolder(tmp)
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}

	is.ClearCurrentPlan()
	_, err = is.GetCurrentPlan()
	if !errors.Is(err, ErrNoPlan) {
		t.Fatalf("expected ErrNoPlan after clear, got %v", err)
	}
}

func TestImporterService_ExecuteCurrentPlan_NoPlan(t *testing.T) {
	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	_, err := is.ExecuteCurrentPlan("user1")
	if !errors.Is(err, ErrNoPlan) {
		t.Fatalf("expected ErrNoPlan, got %v", err)
	}
}

func TestImporterService_ExecuteCurrentPlan_HappyPath_UsesExecutorAndStripsFrontmatter(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "---\ntitle: X\n---\n\n# Heading\nBody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(ws)
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if plan.TreeHash != "h1" {
		t.Fatalf("plan.TreeHash=%q want h1", plan.TreeHash)
	}

	res, err := is.ExecuteCurrentPlan("user1")
	if err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	if res.ImportedCount != 1 {
		t.Fatalf("ImportedCount=%d want 1", res.ImportedCount)
	}
	if res.SkippedCount != 0 {
		t.Fatalf("SkippedCount=%d want 0", res.SkippedCount)
	}
	if w.ensureCalls != 1 || w.updateCalls != 1 {
		t.Fatalf("wiki calls ensure=%d update=%d", w.ensureCalls, w.updateCalls)
	}

	if w.lastUpdatedContent == nil {
		t.Fatalf("expected UpdatePage content")
	}
	if strings.Contains(*w.lastUpdatedContent, "title: X") || strings.Contains(*w.lastUpdatedContent, "---") {
		t.Fatalf("frontmatter not stripped; got: %q", *w.lastUpdatedContent)
	}
	if !strings.Contains(*w.lastUpdatedContent, "# Heading") {
		t.Fatalf("expected body to include heading; got: %q", *w.lastUpdatedContent)
	}
}

func TestImporterService_ExecuteCurrentPlan_ExecutorStalePlanPropagatesError(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(ws)
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	// make plan stale
	plan.TreeHash = "OLD"

	_, err = is.ExecuteCurrentPlan("user1")
	if err == nil {
		t.Fatalf("expected stale plan error")
	}
	if !strings.Contains(err.Error(), "plan is stale") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindMarkdownEntries_FindsMdRecursively_AndNormalizesSlashes(t *testing.T) {
	base := t.TempDir()
	mustWrite(t, base, "a.md", "x")
	mustWrite(t, base, "b.txt", "x")
	mustWrite(t, base, "sub/c.MD", "x")
	mustWrite(t, base, "sub/deeper/d.md", "x")

	got, err := FindMarkdownEntries(base)
	if err != nil {
		t.Fatalf("FindMarkdownEntries err: %v", err)
	}

	// collect paths in a set for stable assertion (WalkDir order is OS-dependent)
	set := map[string]bool{}
	for _, e := range got {
		set[e.SourcePath] = true
		// should be slash-normalized
		if strings.Contains(e.SourcePath, `\`) {
			t.Fatalf("SourcePath should be slash-normalized: %q", e.SourcePath)
		}
	}

	// only .md files (case-insensitive)
	if !set["a.md"] || !set["sub/c.MD"] && !set["sub/c.md"] {
		// depending on Rel and actual filename case, filepath.ToSlash keeps case from disk;
		// but we wrote "c.MD". The SourcePath will be "sub/c.MD".
	}
	if !set["a.md"] {
		t.Fatalf("missing a.md, got %#v", set)
	}
	if !set["sub/c.MD"] {
		t.Fatalf("missing sub/c.MD, got %#v", set)
	}
	if !set["sub/deeper/d.md"] {
		t.Fatalf("missing sub/deeper/d.md, got %#v", set)
	}
	if set["b.txt"] {
		t.Fatalf("should not include b.txt")
	}
}
