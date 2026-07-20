package favorites

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

func newTestStore(t *testing.T) *FavoritesStore {
	t.Helper()
	store, err := NewFavoritesStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFavoritesStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	return store
}

func TestFavoritesStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewFavoritesStore(tmp)
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
		store, err := NewFavoritesStore(tmp)
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
