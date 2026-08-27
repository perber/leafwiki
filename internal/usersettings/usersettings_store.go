package usersettings

import (
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	_ "modernc.org/sqlite"
)

// ErrCodeUserSettingsStoreUnavailable identifies
// errUserSettingsStoreUnavailable's LocalizedError. Mirrors
// favorites.ErrCodeFavoritesStoreUnavailable / ErrCodeAPIKeyStoreUnavailable.
const ErrCodeUserSettingsStoreUnavailable = "usersettings_store_unavailable"

// errUserSettingsStoreUnavailable is returned while the store is suspended
// for an in-progress live restore (see UserSettingsStore.PauseForSwap).
func errUserSettingsStoreUnavailable() error {
	return sharederrors.NewLocalizedError(
		ErrCodeUserSettingsStoreUnavailable,
		"The server is restoring from a backup — please try again in a moment",
		"user settings store is suspended for an in-progress restore",
		nil,
	)
}

type UserSettingsStore struct {
	mu sync.Mutex
	db *sql.DB
	// suspended is set by PauseForSwap and makes every query method refuse
	// to serve queries against a possibly-stale db handle until Replace
	// reopens it.
	suspended bool
	log       *slog.Logger
}

func NewUserSettingsStore(storageDir string, log *slog.Logger) (*UserSettingsStore, error) {
	normalized := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	dbPath := filepath.Join(normalized, "usersettings.db")

	s := &UserSettingsStore{log: log}
	err := sqliteutil.RetryOnCorruption(dbPath, func() error {
		db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err != nil {
			return fmt.Errorf("failed to open user settings database: %w", err)
		}
		s.db = db
		if err := s.ensureSchema(); err != nil {
			_ = db.Close()
			s.db = nil
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *UserSettingsStore) ensureSchema() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_settings (
			user_id     TEXT PRIMARY KEY,
			language    TEXT NOT NULL,
			autosave    INTEGER NOT NULL,
			date_format TEXT NOT NULL DEFAULT 'locale',
			time_format TEXT NOT NULL DEFAULT 'locale',
			updated_at  TIMESTAMP NOT NULL
		);
	`); err != nil {
		return err
	}
	return s.ensureFormatColumns()
}

// ensureFormatColumns additively migrates a pre-format-preference
// user_settings table by adding date_format / time_format if missing.
// Existing rows get the "locale" default (follow the UI language). Idempotent:
// columns already present are skipped, so it is safe on every startup.
func (s *UserSettingsStore) ensureFormatColumns() error {
	existing, err := s.existingColumns()
	if err != nil {
		return err
	}

	migrations := []struct {
		column string
		ddl    string
	}{
		{"date_format", "ALTER TABLE user_settings ADD COLUMN date_format TEXT NOT NULL DEFAULT 'locale'"},
		{"time_format", "ALTER TABLE user_settings ADD COLUMN time_format TEXT NOT NULL DEFAULT 'locale'"},
	}

	for _, m := range migrations {
		if existing[m.column] {
			continue
		}
		if _, err := s.db.Exec(m.ddl); err != nil {
			return fmt.Errorf("failed to add column %s to user_settings table: %w", m.column, err)
		}
	}
	return nil
}

func (s *UserSettingsStore) existingColumns() (map[string]bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(user_settings)`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	cols := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// Get returns userID's saved settings, or DefaultUserSettings(userID) if the
// user has never saved any — a missing row is not an error.
func (s *UserSettingsStore) Get(userID string) (*UserSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.suspended || s.db == nil {
		return nil, errUserSettingsStoreUnavailable()
	}
	return s.getLocked(userID)
}

func (s *UserSettingsStore) getLocked(userID string) (*UserSettings, error) {
	row := s.db.QueryRow(
		`SELECT language, autosave, date_format, time_format, updated_at FROM user_settings WHERE user_id = ?`,
		userID,
	)

	var us UserSettings
	us.UserID = userID
	if err := row.Scan(&us.Language, &us.AutoSave, &us.DateFormat, &us.TimeFormat, &us.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return DefaultUserSettings(userID), nil
		}
		return nil, fmt.Errorf("failed to get user settings for user %s: %w", userID, err)
	}
	return &us, nil
}

// Upsert saves us, replacing any settings userID previously saved.
func (s *UserSettingsStore) Upsert(us *UserSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.suspended || s.db == nil {
		return errUserSettingsStoreUnavailable()
	}
	return s.upsertLocked(us)
}

func (s *UserSettingsStore) upsertLocked(us *UserSettings) error {
	_, err := s.db.Exec(
		`INSERT INTO user_settings (user_id, language, autosave, date_format, time_format, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   language = excluded.language,
		   autosave = excluded.autosave,
		   date_format = excluded.date_format,
		   time_format = excluded.time_format,
		   updated_at = excluded.updated_at`,
		us.UserID, us.Language, us.AutoSave, us.DateFormat, us.TimeFormat, us.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save user settings for user %s: %w", us.UserID, err)
	}
	return nil
}

// UpdateAtomic reads userID's current settings, applies mutate to them, and
// saves the result — get, mutate, and save all happen under a single lock so
// two concurrent updates for the same user can't race and silently drop one
// of the two changes.
func (s *UserSettingsStore) UpdateAtomic(userID string, mutate func(*UserSettings)) (*UserSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.suspended || s.db == nil {
		return nil, errUserSettingsStoreUnavailable()
	}

	current, err := s.getLocked(userID)
	if err != nil {
		return nil, err
	}
	mutate(current)
	if err := s.upsertLocked(current); err != nil {
		return nil, err
	}
	return current, nil
}

// DeleteAllForUser removes userID's saved settings, if any. Called on user delete.
func (s *UserSettingsStore) DeleteAllForUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.suspended || s.db == nil {
		return errUserSettingsStoreUnavailable()
	}

	_, err := s.db.Exec(`DELETE FROM user_settings WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user settings for user %s: %w", userID, err)
	}
	return nil
}

// PauseForSwap closes the current *sql.DB connection and marks the store
// suspended so every query method refuses to serve against a possibly-stale
// handle, releasing any OS-level file lock on usersettings.db before a live
// restore renames it. Idempotent: a second call is a safe no-op. Mirrors
// favorites.FavoritesStore.PauseForSwap / APIKeyStore.suspend.
func (s *UserSettingsStore) PauseForSwap() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.suspended = true
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// Replace reopens usersettings.db from storageDir and swaps the new
// connection in, closing the previous one afterward — used by a live restore
// once usersettings.db has been swapped in on disk (or left untouched, if
// this snapshot didn't capture one). Preserves this UserSettingsStore's
// identity: no caller needs to be told about the new connection, they
// already hold this pointer. Mirrors favorites.FavoritesStore.Replace.
func (s *UserSettingsStore) Replace(storageDir string) error {
	fresh, err := NewUserSettingsStore(storageDir, s.log)
	if err != nil {
		return err
	}

	s.mu.Lock()
	old := s.db
	s.db = fresh.db
	s.suspended = false
	s.mu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil && s.log != nil {
			s.log.Warn("failed to close previous user settings store after restore", "error", err)
		}
	}
	return nil
}

func (s *UserSettingsStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return err
		}
		s.db = nil
	}
	return nil
}
