package sharing

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/test_utils"
)

func newTestStore(t *testing.T) *ShareLinkStore {
	t.Helper()
	store, err := NewShareLinkStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewShareLinkStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	return store
}

// storedLinkIDs returns the IDs of every stored link, newest first. Listing
// links is not part of the store's API yet; it arrives with its first caller.
func storedLinkIDs(t *testing.T, store *ShareLinkStore) []string {
	t.Helper()
	var ids []string
	err := store.withDB(func(db *sql.DB) error {
		rows, err := db.Query(`SELECT id FROM share_links ORDER BY created_at DESC, id`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("storedLinkIDs: %v", err)
	}
	return ids
}

func testLink(id, pageID, tokenHash string) *ShareLink {
	return &ShareLink{
		ID:        id,
		PageID:    pageID,
		CreatedBy: "editor-1",
		CreatedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		tokenHash: tokenHash,
	}
}

func TestShareLinkStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewShareLinkStore(tmp, nil)
	if err != nil {
		t.Fatalf("NewShareLinkStore: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(tmp, "share_links.db")); err != nil {
		t.Fatalf("expected share_links.db to exist: %v", err)
	}
}

func TestShareLinkStore_IdempotentSchema(t *testing.T) {
	tmp := t.TempDir()
	for i := 0; i < 3; i++ {
		store, err := NewShareLinkStore(tmp, nil)
		if err != nil {
			t.Fatalf("NewShareLinkStore (run %d): %v", i, err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close (run %d): %v", i, err)
		}
	}
}

func TestShareLinkStore_Insert_ThenFindByTokenHash_RoundTrips(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	expires := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	want := testLink("share-1", "page-1", "hash-1")
	want.IncludeDescendants = true
	want.ExpiresAt = &expires

	if err := store.Insert(ctx, want); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	got, err := store.FindByTokenHash(ctx, "hash-1")
	if err != nil {
		t.Fatalf("FindByTokenHash: %v", err)
	}
	if got.ID != want.ID || got.PageID != want.PageID || !got.IncludeDescendants ||
		got.CreatedBy != want.CreatedBy || !got.CreatedAt.Equal(want.CreatedAt) ||
		got.ExpiresAt == nil || !got.ExpiresAt.Equal(expires) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestShareLinkStore_FindByTokenHash_UnknownIsNotFound(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.FindByTokenHash(context.Background(), "nope"); !errors.Is(err, errLinkNotStored) {
		t.Fatalf("err = %v, want errLinkNotStored", err)
	}
}

func TestShareLinkStore_Insert_DuplicateTokenHashFails(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.Insert(ctx, testLink("share-1", "page-1", "same-hash")); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := store.Insert(ctx, testLink("share-2", "page-2", "same-hash")); err == nil {
		t.Fatal("expected a second link with the same token hash to be rejected")
	}
}

func TestShareLinkStore_Delete_ReportsWhetherARowWasRemoved(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.Insert(ctx, testLink("share-1", "page-1", "hash-1")); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	removed, err := store.Delete(ctx, "share-1")
	if err != nil || !removed {
		t.Fatalf("Delete existing = %v, %v; want true, nil", removed, err)
	}
	removed, err = store.Delete(ctx, "share-1")
	if err != nil || removed {
		t.Fatalf("Delete again = %v, %v; want false, nil", removed, err)
	}
	if _, err := store.FindByTokenHash(ctx, "hash-1"); !errors.Is(err, errLinkNotStored) {
		t.Fatalf("deleted link still found: %v", err)
	}
}

func TestShareLinkStore_PauseForSwap_ClosesDBAndBlocksQueries(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}
	if store.db != nil {
		t.Fatal("expected db to be nil immediately after PauseForSwap")
	}

	_, err := store.FindByTokenHash(ctx, "hash-1")
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != ErrCodeShareLinkStoreUnavailable {
		t.Fatalf("expected %s, got %v", ErrCodeShareLinkStoreUnavailable, err)
	}
	if store.db != nil {
		t.Fatal("a query against a suspended store must not reopen it")
	}
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("second PauseForSwap: %v", err)
	}
}

func TestShareLinkStore_Replace_ReopensAgainstNewFile(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := NewShareLinkStore(dir, nil)
	if err != nil {
		t.Fatalf("NewShareLinkStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	if err := store.Insert(ctx, testLink("pre-restore", "page-1", "h-pre")); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}

	// Simulate a restore swapping in a different share_links.db.
	if err := os.Remove(filepath.Join(dir, "share_links.db")); err != nil {
		t.Fatalf("remove original share_links.db: %v", err)
	}
	replacement, err := NewShareLinkStore(dir, nil)
	if err != nil {
		t.Fatalf("NewShareLinkStore (replacement): %v", err)
	}
	if err := replacement.Insert(ctx, testLink("restored", "page-1", "h-restored")); err != nil {
		t.Fatalf("Insert (replacement): %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("Close (replacement): %v", err)
	}

	if err := store.Replace(dir); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if ids := storedLinkIDs(t, store); len(ids) != 1 || ids[0] != "restored" {
		t.Fatalf("expected Replace to reflect the file on disk (restored), got %v", ids)
	}
}

// A corrupt share_links.db is moved aside with its sidecars, never deleted,
// and the store keeps working on an empty database (ADR-0013).
func TestNewShareLinkStore_CorruptFileIsQuarantined(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "share_links.db")
	files := test_utils.WriteCorruptSQLiteDB(t, dbPath)

	store, err := NewShareLinkStore(dir, nil)
	if err != nil {
		t.Fatalf("NewShareLinkStore: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)
	if err := store.Insert(context.Background(), testLink("share-1", "page-1", "hash-1")); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	test_utils.AssertSQLiteQuarantined(t, dbPath, files)
}
