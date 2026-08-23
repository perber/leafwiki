package usersettings

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	_ "modernc.org/sqlite"
)

type UserSettingsStore struct {
	mu sync.Mutex
	db *sql.DB
}

func NewUserSettingsStore(storageDir string) (*UserSettingsStore, error) {
	normalized := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	dbPath := filepath.Join(normalized, "usersettings.db")

	s := &UserSettingsStore{}
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
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_settings (
			user_id    TEXT PRIMARY KEY,
			language   TEXT NOT NULL,
			autosave   INTEGER NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);
	`)
	return err
}

// Get returns userID's saved settings, or DefaultUserSettings(userID) if the
// user has never saved any — a missing row is not an error.
func (s *UserSettingsStore) Get(userID string) (*UserSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getLocked(userID)
}

func (s *UserSettingsStore) getLocked(userID string) (*UserSettings, error) {
	row := s.db.QueryRow(
		`SELECT language, autosave, updated_at FROM user_settings WHERE user_id = ?`,
		userID,
	)

	var us UserSettings
	us.UserID = userID
	if err := row.Scan(&us.Language, &us.AutoSave, &us.UpdatedAt); err != nil {
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

	return s.upsertLocked(us)
}

func (s *UserSettingsStore) upsertLocked(us *UserSettings) error {
	_, err := s.db.Exec(
		`INSERT INTO user_settings (user_id, language, autosave, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   language = excluded.language,
		   autosave = excluded.autosave,
		   updated_at = excluded.updated_at`,
		us.UserID, us.Language, us.AutoSave, us.UpdatedAt,
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

	_, err := s.db.Exec(`DELETE FROM user_settings WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user settings for user %s: %w", userID, err)
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
