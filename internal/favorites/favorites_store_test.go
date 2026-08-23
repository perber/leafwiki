package favorites

import (
	"os"
	"path/filepath"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/test_utils"
)

func newTestStore(t *testing.T) *FavoritesStore {
	t.Helper()
	store, err := NewFavoritesStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewFavoritesStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	return store
}

func TestFavoritesStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewFavoritesStore(tmp, nil)
	if err != nil {
		t.Fatalf("NewFavoritesStore: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(tmp, "favorites.db")); err != nil {
		t.Fatalf("expected favorites.db to exist: %v", err)
	}
}

func TestFavoritesStore_IdempotentSchema(t *testing.T) {
	tmp := t.TempDir()
	for i := 0; i < 3; i++ {
		store, err := NewFavoritesStore(tmp, nil)
		if err != nil {
			t.Fatalf("NewFavoritesStore (run %d): %v", i, err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close (run %d): %v", i, err)
		}
	}
}

func TestFavoritesStore_Add_ThenListPageIDsForUser_ReturnsIt(t *testing.T) {
	store := newTestStore(t)

	if err := store.Add("user-1", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := store.ListPageIDsForUser("user-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser: %v", err)
	}
	if len(got) != 1 || got[0] != "page-1" {
		t.Fatalf("expected [page-1], got %v", got)
	}
}

func TestFavoritesStore_Add_IsIdempotent(t *testing.T) {
	store := newTestStore(t)

	if err := store.Add("user-1", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add("user-1", "page-1"); err != nil {
		t.Fatalf("Add (again): %v", err)
	}

	got, err := store.ListPageIDsForUser("user-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly one favorite, got %v", got)
	}
}

func TestFavoritesStore_Remove_IsIdempotentAndRemovesOnlyThatUsersFavorite(t *testing.T) {
	store := newTestStore(t)

	if err := store.Add("user-1", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add("user-2", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := store.Remove("user-1", "page-1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	// Removing again (already removed) must not error.
	if err := store.Remove("user-1", "page-1"); err != nil {
		t.Fatalf("Remove (again): %v", err)
	}

	got1, err := store.ListPageIDsForUser("user-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser user-1: %v", err)
	}
	if len(got1) != 0 {
		t.Fatalf("expected user-1 to have no favorites, got %v", got1)
	}

	got2, err := store.ListPageIDsForUser("user-2")
	if err != nil {
		t.Fatalf("ListPageIDsForUser user-2: %v", err)
	}
	if len(got2) != 1 || got2[0] != "page-1" {
		t.Fatalf("expected user-2 to still have page-1, got %v", got2)
	}
}

func TestFavoritesStore_ListPageIDsForUser_ScopedPerUser(t *testing.T) {
	store := newTestStore(t)

	if err := store.Add("user-1", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add("user-2", "page-2"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := store.ListPageIDsForUser("user-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser: %v", err)
	}
	if len(got) != 1 || got[0] != "page-1" {
		t.Fatalf("expected [page-1] for user-1, got %v", got)
	}
}

func TestFavoritesStore_DeleteAllForPage_RemovesAcrossUsers(t *testing.T) {
	store := newTestStore(t)

	if err := store.Add("user-1", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add("user-2", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add("user-1", "page-2"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := store.DeleteAllForPage("page-1"); err != nil {
		t.Fatalf("DeleteAllForPage: %v", err)
	}

	got1, err := store.ListPageIDsForUser("user-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser user-1: %v", err)
	}
	if len(got1) != 1 || got1[0] != "page-2" {
		t.Fatalf("expected user-1 to only have page-2 left, got %v", got1)
	}

	got2, err := store.ListPageIDsForUser("user-2")
	if err != nil {
		t.Fatalf("ListPageIDsForUser user-2: %v", err)
	}
	if len(got2) != 0 {
		t.Fatalf("expected user-2 to have no favorites left, got %v", got2)
	}
}

func TestFavoritesStore_DeleteAllForUser_RemovesOnlyThatUsersFavorites(t *testing.T) {
	store := newTestStore(t)

	if err := store.Add("user-1", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add("user-1", "page-2"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Add("user-2", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := store.DeleteAllForUser("user-1"); err != nil {
		t.Fatalf("DeleteAllForUser: %v", err)
	}

	got1, err := store.ListPageIDsForUser("user-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser user-1: %v", err)
	}
	if len(got1) != 0 {
		t.Fatalf("expected user-1 to have no favorites left, got %v", got1)
	}

	got2, err := store.ListPageIDsForUser("user-2")
	if err != nil {
		t.Fatalf("ListPageIDsForUser user-2: %v", err)
	}
	if len(got2) != 1 || got2[0] != "page-1" {
		t.Fatalf("expected user-2 to still have page-1, got %v", got2)
	}
}

func TestFavoritesStore_PauseForSwap_ClosesDBAndBlocksQueries(t *testing.T) {
	store := newTestStore(t)
	if err := store.Add("user-1", "page-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}
	if store.db != nil {
		t.Fatal("expected db to be nil immediately after PauseForSwap")
	}

	_, err := store.ListPageIDsForUser("user-1")
	if err == nil {
		t.Fatal("expected a query against a suspended store to fail, not silently reconnect")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != ErrCodeFavoritesStoreUnavailable {
		t.Fatalf("expected %s, got %v", ErrCodeFavoritesStoreUnavailable, err)
	}
	if store.db != nil {
		t.Fatal("expected the failed query to NOT have reopened db — that's exactly the race this fix prevents")
	}

	// PauseForSwap must be safe to call again while already suspended.
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("second PauseForSwap: %v", err)
	}
}

func TestFavoritesStore_Replace_ReopensAgainstNewFileAndClosesOld(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFavoritesStore(dir, nil)
	if err != nil {
		t.Fatalf("NewFavoritesStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })

	if err := store.Add("user-1", "pre-restore-page"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}

	// Simulate a restore having swapped in a different favorites.db at the
	// same path while this store's handle was released.
	if err := os.Remove(filepath.Join(dir, "favorites.db")); err != nil {
		t.Fatalf("remove original favorites.db: %v", err)
	}
	replacement, err := NewFavoritesStore(dir, nil)
	if err != nil {
		t.Fatalf("NewFavoritesStore (replacement): %v", err)
	}
	if err := replacement.Add("user-1", "restored-page"); err != nil {
		t.Fatalf("Add (replacement): %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("Close (replacement): %v", err)
	}

	if err := store.Replace(dir); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, err := store.ListPageIDsForUser("user-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser after Replace: %v", err)
	}
	if len(got) != 1 || got[0] != "restored-page" {
		t.Fatalf("expected Replace to reflect the file actually on disk (restored-page), got %v", got)
	}
}
