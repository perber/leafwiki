package usersettings

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/perber/wiki/internal/test_utils"
)

func newTestStore(t *testing.T) *UserSettingsStore {
	t.Helper()
	store, err := NewUserSettingsStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUserSettingsStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	return store
}

func TestUserSettingsStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewUserSettingsStore(tmp)
	if err != nil {
		t.Fatalf("NewUserSettingsStore: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(tmp, "usersettings.db")); err != nil {
		t.Fatalf("expected usersettings.db to exist: %v", err)
	}
}

func TestUserSettingsStore_IdempotentSchema(t *testing.T) {
	tmp := t.TempDir()
	for i := 0; i < 3; i++ {
		store, err := NewUserSettingsStore(tmp)
		if err != nil {
			t.Fatalf("NewUserSettingsStore (run %d): %v", i, err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close (run %d): %v", i, err)
		}
	}
}

func TestUserSettingsStore_Get_NoRow_ReturnsDefaults(t *testing.T) {
	store := newTestStore(t)

	got, err := store.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := DefaultUserSettings("user-1")
	if got.Language != want.Language || got.AutoSave != want.AutoSave {
		t.Fatalf("expected defaults %+v, got %+v", want, got)
	}
}

func TestUserSettingsStore_Upsert_ThenGet_ReturnsSavedValues(t *testing.T) {
	store := newTestStore(t)

	saved := &UserSettings{
		UserID:    "user-1",
		Language:  "en",
		AutoSave:  false,
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := store.Upsert(saved); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := store.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Language != saved.Language || got.AutoSave != saved.AutoSave {
		t.Fatalf("expected %+v, got %+v", saved, got)
	}
}

func TestUserSettingsStore_Upsert_Twice_UpdatesInPlace(t *testing.T) {
	store := newTestStore(t)

	if err := store.Upsert(&UserSettings{UserID: "user-1", Language: "en", AutoSave: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert (1): %v", err)
	}
	if err := store.Upsert(&UserSettings{UserID: "user-1", Language: "en", AutoSave: false, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert (2): %v", err)
	}

	got, err := store.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AutoSave != false {
		t.Fatalf("expected AutoSave=false after second upsert, got %v", got.AutoSave)
	}

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM user_settings WHERE user_id = ?`, "user-1").Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one row for user-1, got %d", count)
	}
}

// TestUserSettingsStore_UpdateAtomic_ConcurrentCallsDoNotDropEachOthersChanges
// pins the fix for a read-modify-write race: two concurrent UpdateAtomic
// calls for the same user, each mutating a different field, must both land —
// a naive separate Get+Upsert would let the second overwrite the first.
func TestUserSettingsStore_UpdateAtomic_ConcurrentCallsDoNotDropEachOthersChanges(t *testing.T) {
	store := newTestStore(t)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = store.UpdateAtomic("user-1", func(us *UserSettings) { us.Language = "en" })
	}()
	go func() {
		defer wg.Done()
		_, _ = store.UpdateAtomic("user-1", func(us *UserSettings) { us.AutoSave = false })
	}()
	wg.Wait()

	got, err := store.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Language != "en" || got.AutoSave != false {
		t.Fatalf("expected both concurrent updates to have landed (Language=en, AutoSave=false), got %+v", got)
	}
}

func TestUserSettingsStore_DeleteAllForUser_RemovesRowAndGetFallsBackToDefaults(t *testing.T) {
	store := newTestStore(t)

	if err := store.Upsert(&UserSettings{UserID: "user-1", Language: "en", AutoSave: false, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.DeleteAllForUser("user-1"); err != nil {
		t.Fatalf("DeleteAllForUser: %v", err)
	}

	got, err := store.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := DefaultUserSettings("user-1")
	if got.Language != want.Language || got.AutoSave != want.AutoSave {
		t.Fatalf("expected defaults after delete %+v, got %+v", want, got)
	}
}

func TestUserSettingsStore_Upsert_IsolatedPerUser(t *testing.T) {
	store := newTestStore(t)

	if err := store.Upsert(&UserSettings{UserID: "user-1", Language: "en", AutoSave: false, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert user-1: %v", err)
	}
	if err := store.Upsert(&UserSettings{UserID: "user-2", Language: "en", AutoSave: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert user-2: %v", err)
	}

	got1, err := store.Get("user-1")
	if err != nil {
		t.Fatalf("Get user-1: %v", err)
	}
	got2, err := store.Get("user-2")
	if err != nil {
		t.Fatalf("Get user-2: %v", err)
	}
	if got1.AutoSave == got2.AutoSave {
		t.Fatalf("expected user-1 and user-2 settings to be independent, both AutoSave=%v", got1.AutoSave)
	}
}
