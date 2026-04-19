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
	importerDir := filepath.Join(t.TempDir(), ".importer")
	store := NewPlanStore(filepath.Join(importerDir, "current-plan.json"))
	return &ImporterService{
		planner:          planner,
		planStore:        store,
		extractor:        NewZipExtractor(), // unused in these tests
		logger:           slog.Default().With("component", "ImporterServiceTest"),
		workspaceBaseDir: filepath.Join(importerDir, "workspaces"),
	}
}

func waitForExecutionStatus(t *testing.T, is *ImporterService, want ExecutionStatus) *CurrentPlanState {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, err := is.GetCurrentPlan()
		if err == nil && state.ExecutionStatus == want {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}

	state, err := is.GetCurrentPlan()
	t.Fatalf("timed out waiting for execution status %q, state=%#v err=%v", want, state, err)
	return nil
}

// --- Tests ------------------------------------------------------------------

func TestImporterService_createImportPlanFromFolder_StoresPlan(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, tmp, "a.md", "# A\nbody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.CreateImportPlanFromFolder(tmp, "")
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
	if err := is.planStore.Set(&StoredPlan{
		Plan:          &PlanResult{ID: "old", TreeHash: "h1"},
		PlanOptions:   PlanOptions{SourceBasePath: oldWS},
		WorkspaceRoot: oldWS,
		CreatedAt:     time.Now(),
	}); err != nil {
		t.Fatalf("Set err: %v", err)
	}

	_, err := is.CreateImportPlanFromFolder(newWS, "")
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

	_, err := is.CreateImportPlanFromFolder(tmp, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}

	if err := is.ClearCurrentPlan(); err != nil {
		t.Fatalf("ClearCurrentPlan err: %v", err)
	}
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

func TestImporterService_StartCurrentPlanExecution_RunsInBackgroundAndStoresResult(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A\nbody")

	allowEnsure := make(chan struct{})
	w := &fakeWiki{
		treeHash: "h1",
		lookups:  map[string]*tree.PathLookup{},
		ensureFn: func(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error) {
			<-allowEnsure
			return &tree.Page{PageNode: &tree.PageNode{ID: "p1", Title: title, Slug: "slug", Kind: *kind}}, nil
		},
	}
	is := newServiceWithFakeWiki(t, w)

	if _, err := is.CreateImportPlanFromFolder(ws, ""); err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}

	state, started, err := is.StartCurrentPlanExecution("user1")
	if err != nil {
		t.Fatalf("StartCurrentPlanExecution err: %v", err)
	}
	if !started {
		t.Fatalf("expected execution to start")
	}
	if state.ExecutionStatus != ExecutionStatusRunning {
		t.Fatalf("expected running state, got %q", state.ExecutionStatus)
	}
	if state.TotalItems != 1 || state.ProcessedItems != 0 {
		t.Fatalf("expected initial progress 0/1, got %d/%d", state.ProcessedItems, state.TotalItems)
	}
	if state.StartedAt == nil {
		t.Fatalf("expected started_at to be set")
	}

	runningState, err := is.GetCurrentPlan()
	if err != nil {
		t.Fatalf("GetCurrentPlan err: %v", err)
	}
	if runningState.ExecutionStatus != ExecutionStatusRunning {
		t.Fatalf("expected stored running state, got %q", runningState.ExecutionStatus)
	}

	close(allowEnsure)

	completedState := waitForExecutionStatus(t, is, ExecutionStatusCompleted)
	if completedState.ExecutionResult == nil {
		t.Fatalf("expected execution result to be stored")
	}
	if completedState.ExecutionResult.ImportedCount != 1 {
		t.Fatalf("expected imported count 1, got %#v", completedState.ExecutionResult)
	}
	if completedState.ProcessedItems != 1 || completedState.TotalItems != 1 {
		t.Fatalf("expected final progress 1/1, got %d/%d", completedState.ProcessedItems, completedState.TotalItems)
	}
	if completedState.CurrentItemSourcePath != nil {
		t.Fatalf("expected current item to be cleared after completion, got %q", *completedState.CurrentItemSourcePath)
	}
	if completedState.FinishedAt == nil {
		t.Fatalf("expected finished_at to be set")
	}
}

func TestImporterService_ClearCurrentPlan_WhileRunning_ReturnsError(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A\nbody")

	allowEnsure := make(chan struct{})
	w := &fakeWiki{
		treeHash: "h1",
		lookups:  map[string]*tree.PathLookup{},
		ensureFn: func(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error) {
			<-allowEnsure
			return &tree.Page{PageNode: &tree.PageNode{ID: "p1", Title: title, Slug: "slug", Kind: *kind}}, nil
		},
	}
	is := newServiceWithFakeWiki(t, w)

	if _, err := is.CreateImportPlanFromFolder(ws, ""); err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if _, _, err := is.StartCurrentPlanExecution("user1"); err != nil {
		t.Fatalf("StartCurrentPlanExecution err: %v", err)
	}

	err := is.ClearCurrentPlan()
	if !errors.Is(err, ErrImportExecutionRunning) {
		t.Fatalf("expected ErrImportExecutionRunning, got %v", err)
	}

	close(allowEnsure)
	waitForExecutionStatus(t, is, ExecutionStatusCompleted)
}

func TestImporterService_CancelCurrentPlan_StopsBeforeNextItem(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A\nbody")
	mustWrite(t, ws, "b.md", "# B\nbody")

	enterFirstEnsure := make(chan struct{}, 1)
	allowFirstEnsure := make(chan struct{})
	w := &fakeWiki{
		treeHash: "h1",
		lookups:  map[string]*tree.PathLookup{},
		ensureFn: func(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error) {
			if targetPath == "a" {
				enterFirstEnsure <- struct{}{}
				<-allowFirstEnsure
			}
			return &tree.Page{PageNode: &tree.PageNode{ID: "p1", Title: title, Slug: "slug", Kind: *kind}}, nil
		},
	}
	is := newServiceWithFakeWiki(t, w)

	if _, err := is.CreateImportPlanFromFolder(ws, ""); err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if _, _, err := is.StartCurrentPlanExecution("user1"); err != nil {
		t.Fatalf("StartCurrentPlanExecution err: %v", err)
	}

	<-enterFirstEnsure

	state, requested, err := is.CancelCurrentPlan()
	if err != nil {
		t.Fatalf("CancelCurrentPlan err: %v", err)
	}
	if !requested || !state.CancelRequested {
		t.Fatalf("expected cancel request to be recorded, got requested=%v state=%#v", requested, state)
	}

	close(allowFirstEnsure)

	canceledState := waitForExecutionStatus(t, is, ExecutionStatusCanceled)
	if canceledState.ExecutionResult == nil {
		t.Fatalf("expected partial result on cancellation")
	}
	if canceledState.ExecutionResult.ImportedCount != 1 {
		t.Fatalf("expected one imported item before cancel, got %#v", canceledState.ExecutionResult)
	}
	if canceledState.ProcessedItems != 1 || canceledState.TotalItems != 2 {
		t.Fatalf("expected progress 1/2 after cancel, got %d/%d", canceledState.ProcessedItems, canceledState.TotalItems)
	}
}

func TestImporterService_ResumesRunningImportFromPersistedState(t *testing.T) {
	workspaceRoot := t.TempDir()
	mustWrite(t, workspaceRoot, "a.md", "# A\nbody")
	mustWrite(t, workspaceRoot, "b.md", "# B\nbody")

	stateRoot := t.TempDir()
	stateFile := filepath.Join(stateRoot, "current-plan.json")
	w := &fakeWiki{treeHash: "partial-tree", lookups: map[string]*tree.PathLookup{}}
	planner := NewPlanner(w, tree.NewSlugService())
	store := NewPlanStore(stateFile)

	service := &ImporterService{
		planner:          planner,
		planStore:        store,
		extractor:        NewZipExtractor(),
		logger:           slog.Default().With("component", "ImporterServiceTest"),
		workspaceBaseDir: filepath.Join(stateRoot, "workspaces"),
	}

	plan, err := service.CreateImportPlanFromFolder(workspaceRoot, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	plan.TreeHash = "original-tree"

	sp, started, err := store.TryStartExecution("user1")
	if err != nil || !started {
		t.Fatalf("TryStartExecution err=%v started=%v", err, started)
	}
	startedAt := time.Now()
	if err := store.UpdateExecutionProgress(plan.ID, ExecutionProgress{
		ProcessedItems: 1,
		TotalItems:     2,
		StartedAt:      &startedAt,
	}, &ExecutionResult{
		ImportedCount:  1,
		TreeHashBefore: "original-tree",
		TreeHash:       "partial-tree",
		Items: []ExecutionItemResult{
			{SourcePath: "a.md", TargetPath: "a", Action: ExecutionActionCreated},
		},
	}); err != nil {
		t.Fatalf("UpdateExecutionProgress err: %v", err)
	}

	resumed := NewImporterService(planner, NewPlanStore(stateFile), filepath.Join(stateRoot, "workspaces"), 0)
	_ = sp

	completedState := waitForExecutionStatus(t, resumed, ExecutionStatusCompleted)
	if completedState.ExecutionResult == nil {
		t.Fatalf("expected completed result after resume")
	}
	if completedState.ExecutionResult.ImportedCount != 2 {
		t.Fatalf("expected resumed import to keep prior count and finish with 2 imports, got %#v", completedState.ExecutionResult)
	}
	if completedState.ProcessedItems != 2 || completedState.TotalItems != 2 {
		t.Fatalf("expected final progress 2/2 after resume, got %d/%d", completedState.ProcessedItems, completedState.TotalItems)
	}
}

func TestImporterService_ResumeRunningImport_FailsWhenTreeHashChanged(t *testing.T) {
	workspaceRoot := t.TempDir()
	mustWrite(t, workspaceRoot, "a.md", "# A\nbody")
	mustWrite(t, workspaceRoot, "b.md", "# B\nbody")

	stateRoot := t.TempDir()
	stateFile := filepath.Join(stateRoot, "current-plan.json")
	w := &fakeWiki{treeHash: "changed-tree", lookups: map[string]*tree.PathLookup{}}
	planner := NewPlanner(w, tree.NewSlugService())
	store := NewPlanStore(stateFile)

	service := &ImporterService{
		planner:          planner,
		planStore:        store,
		extractor:        NewZipExtractor(),
		logger:           slog.Default().With("component", "ImporterServiceTest"),
		workspaceBaseDir: filepath.Join(stateRoot, "workspaces"),
	}

	plan, err := service.CreateImportPlanFromFolder(workspaceRoot, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	plan.TreeHash = "original-tree"
	if err := store.Set(&StoredPlan{
		Plan:            plan,
		PlanOptions:     PlanOptions{SourceBasePath: workspaceRoot},
		WorkspaceRoot:   workspaceRoot,
		CreatedAt:       time.Now(),
		ExecutionStatus: ExecutionStatusRunning,
		ExecutionUserID: "user1",
		ExecutionResult: &ExecutionResult{
			ImportedCount:  1,
			TreeHashBefore: "original-tree",
			TreeHash:       "partially-imported-tree",
			Items: []ExecutionItemResult{
				{SourcePath: "a.md", TargetPath: "a", Action: ExecutionActionCreated},
			},
		},
		ExecutionProgress: ExecutionProgress{
			ProcessedItems: 1,
			TotalItems:     2,
		},
	}); err != nil {
		t.Fatalf("Set err: %v", err)
	}

	resumed := NewImporterService(planner, NewPlanStore(stateFile), filepath.Join(stateRoot, "workspaces"), 0)
	failedState := waitForExecutionStatus(t, resumed, ExecutionStatusFailed)
	if failedState.ExecutionError == nil || !strings.Contains(*failedState.ExecutionError, "plan is stale") {
		t.Fatalf("expected stale-plan failure after resume, got %#v", failedState)
	}
}

func TestImporterService_ExecuteCurrentPlan_HappyPath_PreservesNonInternalFrontmatter(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "---\naliases:\n  - x\ncustom_key: keep-me\nleafwiki_id: source-id\nleafwiki_title: Source Title\ntitle: X\n---\n\n# Heading\nBody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.CreateImportPlanFromFolder(ws, "")
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

func TestImporterService_ExecuteCurrentPlan_ExecutorStalePlanPropagatesError(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.CreateImportPlanFromFolder(ws, "")
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

	plan, err := is.CreateImportPlanFromFolder(tmp, "docs/imports")
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
