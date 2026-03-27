package importer

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
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

	plan, err := is.createImportPlanFromFolder(tmp, "")
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

	_, err := is.createImportPlanFromFolder(newWS, "")
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

	_, err := is.createImportPlanFromFolder(tmp, "")
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

func TestImporterService_ExecuteCurrentPlan_HappyPath_PreservesNonInternalFrontmatter(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "---\naliases:\n  - x\ncustom_key: keep-me\nleafwiki_id: source-id\nleafwiki_title: Source Title\ntitle: X\n---\n\n# Heading\nBody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(ws, "")
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
	fm, body, has, err := markdown.ParseFrontmatter(*w.lastUpdatedContent)
	if err != nil {
		t.Fatalf("ParseFrontmatter err: %v", err)
	}
	if !has {
		t.Fatalf("expected preserved frontmatter, got: %q", *w.lastUpdatedContent)
	}
	if body != "\n# Heading\nBody" {
		t.Fatalf("unexpected body: %q", body)
	}
	if got := fm.ExtraFields["custom_key"]; got != "keep-me" {
		t.Fatalf("expected custom_key to be preserved, got %#v", got)
	}
	if got := fm.ExtraFields["title"]; got != "X" {
		t.Fatalf("expected title extra field to be preserved, got %#v", got)
	}
	aliases, ok := fm.ExtraFields["aliases"].([]interface{})
	if !ok || len(aliases) != 1 || aliases[0] != "x" {
		t.Fatalf("expected aliases to be preserved, got %#v", fm.ExtraFields["aliases"])
	}
	if strings.Contains(*w.lastUpdatedContent, "leafwiki_id: source-id") {
		t.Fatalf("expected source leafwiki_id to be dropped, got: %q", *w.lastUpdatedContent)
	}
	if strings.Contains(*w.lastUpdatedContent, "leafwiki_title: Source Title") {
		t.Fatalf("expected source leafwiki_title to be dropped, got: %q", *w.lastUpdatedContent)
	}
}

func TestImporterService_ExecuteCurrentPlan_WritesPreservedFrontmatterToDisk(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "Imported.md", "---\naliases:\n  - alpha\ncustom_key: keep-me\nleafwiki_id: source-id\ntitle: Imported Title\n---\n\n# Imported Title\nBody")

	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWiki err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	planner := NewPlanner(w, tree.NewSlugService())
	is := NewImporterService(planner, NewPlanStore())

	plan, err := is.createImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("expected one plan item, got %#v", plan.Items)
	}

	res, err := is.ExecuteCurrentPlan("system")
	if err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}
	if res.ImportedCount != 1 || res.SkippedCount != 0 {
		t.Fatalf("unexpected result: imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}

	rawBytes, err := os.ReadFile(filepath.Join(w.GetStorageDir(), "root", "imported.md"))
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}
	raw := string(rawBytes)

	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter err: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter in written file, got: %q", raw)
	}
	if body != "\n# Imported Title\nBody" {
		t.Fatalf("unexpected body: %q", body)
	}
	if got := fm.ExtraFields["custom_key"]; got != "keep-me" {
		t.Fatalf("expected custom_key to be preserved, got %#v", got)
	}
	if got := fm.ExtraFields["title"]; got != "Imported Title" {
		t.Fatalf("expected title to be preserved, got %#v", got)
	}
	aliases, ok := fm.ExtraFields["aliases"].([]interface{})
	if !ok || len(aliases) != 1 || aliases[0] != "alpha" {
		t.Fatalf("expected aliases to be preserved, got %#v", fm.ExtraFields["aliases"])
	}
	if strings.Contains(raw, "leafwiki_id: source-id") {
		t.Fatalf("expected source leafwiki_id to be dropped, got: %q", raw)
	}
	if fm.LeafWikiID == "" {
		t.Fatalf("expected written file to contain generated leafwiki_id")
	}
	if fm.LeafWikiTitle != "Imported Title" {
		t.Fatalf("expected written file to contain effective leafwiki_title, got %q", fm.LeafWikiTitle)
	}
}

func TestImporterService_ExecuteCurrentPlan_ExecutorStalePlanPropagatesError(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(ws, "")
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

func TestImporterService_createImportPlanFromFolder_UsesTargetBasePath(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, tmp, "a.md", "# A\nbody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(tmp, "docs/imports")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if plan == nil || len(plan.Items) != 1 {
		t.Fatalf("unexpected plan: %#v", plan)
	}

	// Verify the plan item has the correct target path with the base path
	item := plan.Items[0]
	if item.TargetPath != "docs/imports/a" {
		t.Fatalf("expected TargetPath 'docs/imports/a', got %q", item.TargetPath)
	}

	// Verify the stored plan options has the correct target base path
	sp, err := is.planStore.Get()
	if err != nil {
		t.Fatalf("Get plan err: %v", err)
	}
	if sp.PlanOptions.TargetBasePath != "docs/imports" {
		t.Fatalf("expected TargetBasePath 'docs/imports', got %q", sp.PlanOptions.TargetBasePath)
	}
}

func TestFindMarkdownEntries_FindsMixedCaseMdExtensions(t *testing.T) {
	base := t.TempDir()
	mustWrite(t, base, "a.MD", "x")
	mustWrite(t, base, "b.mD", "x")
	mustWrite(t, base, "c.Md", "x")
	mustWrite(t, base, "d.txt", "x")

	got, err := FindMarkdownEntries(base)
	if err != nil {
		t.Fatalf("FindMarkdownEntries err: %v", err)
	}

	set := map[string]bool{}
	for _, e := range got {
		set[e.SourcePath] = true
	}

	if !set["a.MD"] || !set["b.mD"] || !set["c.Md"] {
		t.Fatalf("expected mixed-case markdown files to be included, got %#v", set)
	}
	if set["d.txt"] {
		t.Fatalf("should not include non-markdown files")
	}
}
