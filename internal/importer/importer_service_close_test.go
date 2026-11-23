package importer

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/tree"
)

// Regression: closing the wiki while a background import ran let the import
// keep writing into stores that were already closed (a nil *sql.DB panic in
// the links store took the whole process down). Close must stop the import
// before its next item and wait for the running item, and leave it resumable.
func TestImporterService_Close_WaitsForRunningImportAndLeavesItResumable(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A\nbody")
	mustWrite(t, ws, "b.md", "# B\nbody")

	enterFirst := make(chan struct{}, 1)
	allowFirst := make(chan struct{})
	var ensuredB atomic.Bool
	w := &fakeWiki{
		treeHash: "h1",
		lookups:  map[string]*tree.PathLookup{},
		ensureFn: func(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error) {
			switch targetPath {
			case "a":
				enterFirst <- struct{}{}
				<-allowFirst
			case "b":
				ensuredB.Store(true)
			}
			return &tree.Page{PageNode: &tree.PageNode{ID: "p-" + targetPath, Title: title, Slug: targetPath, Kind: *kind}}, nil
		},
	}
	is := newServiceWithFakeWiki(t, w)

	if _, err := is.CreateImportPlanFromFolder(ws, ""); err != nil {
		t.Fatalf("CreateImportPlanFromFolder: %v", err)
	}
	if _, _, err := is.StartCurrentPlanExecution("user1"); err != nil {
		t.Fatalf("StartCurrentPlanExecution: %v", err)
	}
	<-enterFirst

	closed := make(chan struct{})
	go func() {
		is.Close()
		close(closed)
	}()

	// Release the item only once Close has flagged the interrupt, so a slow
	// scheduler cannot let the import run on to the next item.
	deadline := time.Now().Add(5 * time.Second)
	for !is.closing.Load() {
		if time.Now().After(deadline) {
			t.Fatal("Close did not flag the running import")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-closed:
		t.Fatal("Close returned while an import item was still running")
	case <-time.After(100 * time.Millisecond):
	}

	close(allowFirst)
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return after the running item finished")
	}

	if ensuredB.Load() {
		t.Fatal("import continued with the next item after Close")
	}

	state, err := is.GetCurrentPlan()
	if err != nil {
		t.Fatalf("GetCurrentPlan: %v", err)
	}
	if state.ExecutionStatus != ExecutionStatusRunning {
		t.Fatalf("expected the interrupted import to stay %q for resume, got %q", ExecutionStatusRunning, state.ExecutionStatus)
	}
	if state.ProcessedItems != 1 {
		t.Fatalf("expected progress 1 after interruption, got %d", state.ProcessedItems)
	}
}

func TestImporterService_Close_RefusesNewExecutions(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A\nbody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)
	if _, err := is.CreateImportPlanFromFolder(ws, ""); err != nil {
		t.Fatalf("CreateImportPlanFromFolder: %v", err)
	}

	is.Close()

	if _, _, err := is.StartCurrentPlanExecution("user1"); !errors.Is(err, ErrImporterClosed) {
		t.Fatalf("StartCurrentPlanExecution after Close: expected ErrImporterClosed, got %v", err)
	}
	if _, err := is.ExecuteCurrentPlan("user1"); !errors.Is(err, ErrImporterClosed) {
		t.Fatalf("ExecuteCurrentPlan after Close: expected ErrImporterClosed, got %v", err)
	}
	state, err := is.GetCurrentPlan()
	if err != nil {
		t.Fatalf("GetCurrentPlan: %v", err)
	}
	if state.ExecutionStatus != ExecutionStatusPlanned {
		t.Fatalf("expected the plan to stay %q, got %q", ExecutionStatusPlanned, state.ExecutionStatus)
	}
}

// A second Close (and one without any import) must neither block nor panic.
func TestImporterService_Close_IsIdempotentWithoutImport(t *testing.T) {
	is := newServiceWithFakeWiki(t, &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}})
	is.Close()
	is.Close()
}
