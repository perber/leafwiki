package links

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

func TestLinksStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewLinksStore(tmp)
	if err != nil {
		t.Fatalf("NewLinksStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(tmp, "links.db")); err != nil {
		t.Fatalf("expected links.db in storage dir, got err: %v", err)
	}
}

func TestLinksStore_UsesWALJournalMode(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewLinksStore(tmp)
	if err != nil {
		t.Fatalf("NewLinksStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	var mode string
	if err := store.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("failed to read journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want %q", mode, "wal")
	}
}

// Pins the RWMutex change (mu sync.Mutex -> sync.RWMutex, read methods
// moved to RLock): concurrent read calls must be able to run in parallel
// without data races or errors. Run with `go test -race` to actually catch
// a regression (e.g. a read method left on Lock by mistake would still
// pass this test functionally, but a write method accidentally moved to
// RLock would show up as a race here against Close/AddLinks-style
// mutation, which -race is what would catch).
func TestLinksStore_ConcurrentReadsDoNotRace(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewLinksStore(tmp)
	if err != nil {
		t.Fatalf("NewLinksStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	const pageCount = 20
	pageIDs := make([]string, pageCount)
	for i := 0; i < pageCount; i++ {
		pageID := fmt.Sprintf("page-%d", i)
		pageIDs[i] = pageID
		if err := store.AddLinks(pageID, "Title "+pageID, []TargetLink{{
			TargetPageID:   "target-" + pageID,
			TargetPagePath: "target/" + pageID,
		}}); err != nil {
			t.Fatalf("AddLinks(%s) failed: %v", pageID, err)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, pageCount*3)
	for _, pageID := range pageIDs {
		pageID := pageID
		wg.Add(3)
		go func() {
			defer wg.Done()
			if _, err := store.GetBacklinksForPage(pageID); err != nil {
				errCh <- err
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := store.GetOutgoingLinksForPage(pageID); err != nil {
				errCh <- err
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := store.GetOutgoingLinksForPages(pageIDs); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent read failed: %v", err)
	}
}

func TestLinksDatabasePath_WindowsPath(t *testing.T) {
	got := strings.ReplaceAll(linksDatabasePath(`C:\wiki\data`, "links.db"), `\`, `/`)
	want := `C:/wiki/data/links.db`
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestLinksStore_GetOutgoingLinksForPages_BatchesLargeInputs(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewLinksStore(tmp)
	if err != nil {
		t.Fatalf("NewLinksStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	pageIDs := make([]string, 0, maxOutgoingLinksQueryArgs+5)
	for i := 0; i < maxOutgoingLinksQueryArgs+5; i++ {
		pageID := fmt.Sprintf("page-%d", i)
		pageIDs = append(pageIDs, pageID)
		if err := store.AddLinks(pageID, "Title "+pageID, []TargetLink{{
			TargetPageID:   "target-" + pageID,
			TargetPagePath: "target/" + pageID,
		}}); err != nil {
			t.Fatalf("AddLinks(%s) failed: %v", pageID, err)
		}
	}

	outgoingByPageID, err := store.GetOutgoingLinksForPages(pageIDs)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPages failed: %v", err)
	}
	if len(outgoingByPageID) != len(pageIDs) {
		t.Fatalf("expected %d page entries, got %d", len(pageIDs), len(outgoingByPageID))
	}

	for _, pageID := range pageIDs {
		outgoings := outgoingByPageID[pageID]
		if len(outgoings) != 1 {
			t.Fatalf("expected 1 outgoing for %s, got %d", pageID, len(outgoings))
		}
		if outgoings[0].FromPageID != pageID {
			t.Fatalf("expected outgoing from %s, got %s", pageID, outgoings[0].FromPageID)
		}
		if outgoings[0].ToPath != "target/"+pageID {
			t.Fatalf("expected target path %q, got %q", "target/"+pageID, outgoings[0].ToPath)
		}
	}
}
