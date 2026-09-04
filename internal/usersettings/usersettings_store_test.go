package usersettings

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/test_utils"
)

func newTestStore(t *testing.T) *UserSettingsStore {
	t.Helper()
	store, err := NewUserSettingsStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	return store
}

func TestUserSettingsStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewUserSettingsStore(tmp, nil)
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
		store, err := NewUserSettingsStore(tmp, nil)
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

func TestUserSettingsStore_PauseForSwap_ClosesDBAndBlocksQueries(t *testing.T) {
	store := newTestStore(t)
	if err := store.Upsert(&UserSettings{UserID: "user-1", Language: "en", AutoSave: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}
	if store.db != nil {
		t.Fatal("expected db to be nil immediately after PauseForSwap")
	}

	_, err := store.Get("user-1")
	if err == nil {
		t.Fatal("expected a query against a suspended store to fail, not silently reconnect")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != ErrCodeUserSettingsStoreUnavailable {
		t.Fatalf("expected %s, got %v", ErrCodeUserSettingsStoreUnavailable, err)
	}
	if store.db != nil {
		t.Fatal("expected the failed query to NOT have reopened db — that's exactly the race this fix prevents")
	}

	// PauseForSwap must be safe to call again while already suspended.
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("second PauseForSwap: %v", err)
	}
}

func TestUserSettingsStore_Replace_ReopensAgainstNewFileAndClosesOld(t *testing.T) {
	dir := t.TempDir()
	store, err := NewUserSettingsStore(dir, nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })

	if err := store.Upsert(&UserSettings{UserID: "user-1", Language: "en", AutoSave: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}

	// Simulate a restore having swapped in a different usersettings.db at the
	// same path while this store's handle was released.
	if err := os.Remove(filepath.Join(dir, "usersettings.db")); err != nil {
		t.Fatalf("remove original usersettings.db: %v", err)
	}
	replacement, err := NewUserSettingsStore(dir, nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore (replacement): %v", err)
	}
	if err := replacement.Upsert(&UserSettings{UserID: "user-1", Language: "de", AutoSave: false, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert (replacement): %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("Close (replacement): %v", err)
	}

	if err := store.Replace(dir); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, err := store.Get("user-1")
	if err != nil {
		t.Fatalf("Get after Replace: %v", err)
	}
	if got.Language != "de" || got.AutoSave != false {
		t.Fatalf("expected Replace to reflect the file actually on disk (Language=de, AutoSave=false), got %+v", got)
	}
}

// TestUserSettingsStore_MigratesLegacySchema_AddsFormatColumns pins the
// additive migration: opening a pre-format-preference usersettings.db must
// add date_format / time_format and default any existing row to "locale",
// without disturbing the row's other values.
func TestUserSettingsStore_MigratesLegacySchema_AddsFormatColumns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "usersettings.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE user_settings (
		user_id    TEXT PRIMARY KEY,
		language   TEXT NOT NULL,
		autosave   INTEGER NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO user_settings (user_id, language, autosave, updated_at) VALUES (?, ?, ?, ?)`,
		"user-1", "de", 1, time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	store, err := NewUserSettingsStore(dir, nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore (migrating): %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })

	got, err := store.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Language != "de" {
		t.Fatalf("expected legacy language preserved, got %+v", got)
	}
	if got.DateFormat != DefaultDateFormat || got.TimeFormat != DefaultTimeFormat {
		t.Fatalf("expected migrated row to default to locale formats, got %+v", got)
	}

	if err := store.Upsert(&UserSettings{
		UserID: "user-2", Language: "en", AutoSave: true,
		DateFormat: "iso", TimeFormat: "12h", UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got2, err := store.Get("user-2")
	if err != nil {
		t.Fatalf("Get user-2: %v", err)
	}
	if got2.DateFormat != "iso" || got2.TimeFormat != "12h" {
		t.Fatalf("expected iso/12h round-trip, got %+v", got2)
	}
}
