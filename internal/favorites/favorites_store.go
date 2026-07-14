// Package favorites stores each user's private set of favorited pages.
// Unlike tags/links/properties/search, this data is not derived from the
// filesystem tree and must never be touched by resync (see ADR-0001).
package favorites

import (
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared"
	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	_ "modernc.org/sqlite"
)

const logCloseRowsFailed = "could not close rows"

type FavoritesStore struct {
	mu sync.Mutex
	db *sql.DB
}

func NewFavoritesStore(storageDir string) (*FavoritesStore, error) {
	normalized := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	dbPath := filepath.Join(normalized, "favorites.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open favorites database: %w", err)
	}

	s := &FavoritesStore{db: db}
	if err := s.ensureSchema(); err != nil {
		_ = db.Close()
		if !sqliteutil.IsSQLiteRecoverableError(err) {
			return nil, err
		}
		slog.Default().Warn("favorites database corrupt, removing and retrying", "error", err)
		sqliteutil.RemoveSQLiteFiles(dbPath)
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to reopen favorites database after recovery: %w", err)
		}
		s = &FavoritesStore{db: db}
		if err = s.ensureSchema(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return s, nil
}

func (s *FavoritesStore) ensureSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS favorites (
			user_id    TEXT NOT NULL,
			page_id    TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			PRIMARY KEY (user_id, page_id)
		);
		CREATE INDEX IF NOT EXISTS favorites_user_id_idx ON favorites(user_id);
	`)
	return err
}

// Add favorites pageID for userID. Idempotent — favoriting an already-favorited page is a no-op.
func (s *FavoritesStore) Add(userID, pageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO favorites (user_id, page_id, created_at) VALUES (?, ?, ?)`,
		userID, pageID, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed to add favorite for user %s, page %s: %w", userID, pageID, err)
	}
	return nil
}

// Remove un-favorites pageID for userID. Idempotent — removing a non-favorited page is a no-op.
func (s *FavoritesStore) Remove(userID, pageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM favorites WHERE user_id = ? AND page_id = ?`, userID, pageID)
	if err != nil {
		return fmt.Errorf("failed to remove favorite for user %s, page %s: %w", userID, pageID, err)
	}
	return nil
}

// ListPageIDsForUser returns the page IDs favorited by userID, most recently favorited first.
func (s *FavoritesStore) ListPageIDsForUser(userID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(
		`SELECT page_id FROM favorites WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer shared.LogClose(rows.Close, logCloseRowsFailed)

	var pageIDs []string
	for rows.Next() {
		var pageID string
		if err := rows.Scan(&pageID); err != nil {
			return nil, err
		}
		pageIDs = append(pageIDs, pageID)
	}
	return pageIDs, rows.Err()
}

// DeleteAllForPage removes every user's favorite of pageID. Called on page delete.
func (s *FavoritesStore) DeleteAllForPage(pageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM favorites WHERE page_id = ?`, pageID)
	if err != nil {
		return fmt.Errorf("failed to delete favorites for page %s: %w", pageID, err)
	}
	return nil
}

// DeleteAllForUser removes every favorite belonging to userID. Called on user delete.
func (s *FavoritesStore) DeleteAllForUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM favorites WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete favorites for user %s: %w", userID, err)
	}
	return nil
}

func (s *FavoritesStore) Close() error {
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
