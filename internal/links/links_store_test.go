package links

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
