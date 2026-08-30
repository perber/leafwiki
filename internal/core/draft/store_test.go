package draft

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStorePersistsDraftAtomically(t *testing.T) {
	store := NewStore(t.TempDir())
	want := Draft{PageID: "page-1", BaseVersion: "v1", Title: "Page", Content: "draft", Tags: []string{"go"}, Properties: map[string]string{"status": "new"}}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("page-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != want.Content || got.BaseVersion != want.BaseVersion || got.Properties["status"] != "new" {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	entries, err := os.ReadDir(filepath.Dir(filepath.Join(store.dir, "page-1.json")))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "page-1.json" {
			t.Fatalf("unexpected non-atomic residue %q", entry.Name())
		}
	}
}

func TestStoreRejectsUnsafeIDsAndCorruptData(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Save(Draft{PageID: "../escape", BaseVersion: "v1", Title: "Page"}); !errors.Is(err, ErrInvalidPageID) {
		t.Fatalf("err = %v, want invalid ID", err)
	}
	if err := os.MkdirAll(store.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.dir, "page-1.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("page-1"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("err = %v, want corrupt", err)
	}
}

func TestStorePersistsPendingDraft(t *testing.T) {
	store := NewStore(t.TempDir())
	d, err := store.CreatePending("", "Pending", "pending")
	if err != nil {
		t.Fatal(err)
	}
	d.Content = "private"
	d.Properties = map[string]string{"status": "draft"}
	if err := store.SavePending(*d); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetPending(d.ID)
	if err != nil || got.Content != "private" || got.Properties["status"] != "draft" {
		t.Fatalf("pending = %#v, %v", got, err)
	}
	if err := store.DeletePending(d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPending(d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted pending = %v", err)
	}
}
