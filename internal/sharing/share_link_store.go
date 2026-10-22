package sharing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	_ "modernc.org/sqlite"
)

// ShareLinkStore persists share links in share_links.db, storing only a
// SHA-256 hash of each token. It is real state: quarantined on corruption
// (ADR-0013) and part of snapshots (ADR-0016). Same shape as FavoritesStore.
type ShareLinkStore struct {
	mu        sync.Mutex
	db        *sql.DB
	suspended bool // set by PauseForSwap until Replace reopens the file
	log       *slog.Logger
}

// NewShareLinkStore opens (or creates) storageDir/share_links.db.
func NewShareLinkStore(storageDir string, log *slog.Logger) (*ShareLinkStore, error) {
	normalized := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	dbPath := filepath.Join(normalized, "share_links.db")

	s := &ShareLinkStore{log: log}
	err := sqliteutil.OpenWithPolicy(dbPath, sqliteutil.QuarantineOnCorruption, func() error {
		db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err != nil {
			return fmt.Errorf("failed to open share links database: %w", err)
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

func (s *ShareLinkStore) ensureSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS share_links (
			id                  TEXT PRIMARY KEY,
			token_hash          TEXT NOT NULL UNIQUE,
			page_id             TEXT NOT NULL,
			include_descendants INTEGER NOT NULL,
			expires_at          TIMESTAMP NULL,
			created_by          TEXT NOT NULL,
			created_at          TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS share_links_page_id_idx ON share_links(page_id);
	`)
	return err
}

// withDB runs fn, or fails while the store is suspended for a restore.
func (s *ShareLinkStore) withDB(fn func(db *sql.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.suspended || s.db == nil {
		return errShareLinkStoreUnavailable()
	}
	return fn(s.db)
}

// Insert stores link with its token hash.
func (s *ShareLinkStore) Insert(ctx context.Context, link *ShareLink) error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO share_links (id, token_hash, page_id, include_descendants, expires_at, created_by, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			link.ID, link.tokenHash, link.PageID, link.IncludeDescendants, nullableTime(link.ExpiresAt), link.CreatedBy, link.CreatedAt.UTC(),
		)
		if err != nil {
			return fmt.Errorf("failed to insert share link %s: %w", link.ID, err)
		}
		return nil
	})
}

// FindByTokenHash returns the link for tokenHash (ignoring expiry) or errLinkNotStored.
func (s *ShareLinkStore) FindByTokenHash(ctx context.Context, tokenHash string) (*ShareLink, error) {
	var link *ShareLink
	err := s.withDB(func(db *sql.DB) error {
		var err error
		link, err = scanLink(db.QueryRowContext(ctx,
			`SELECT id, token_hash, page_id, include_descendants, expires_at, created_by, created_at
			 FROM share_links WHERE token_hash = ?`, tokenHash))
		return err
	})
	if err != nil {
		return nil, err
	}
	return link, nil
}

// Delete removes the link id and reports whether it existed.
func (s *ShareLinkStore) Delete(ctx context.Context, id string) (bool, error) {
	var removed bool
	err := s.withDB(func(db *sql.DB) error {
		res, err := db.ExecContext(ctx, `DELETE FROM share_links WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("failed to delete share link %s: %w", id, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		removed = n > 0
		return nil
	})
	return removed, err
}

// PauseForSwap closes the database and keeps it closed until Replace, so a
// live restore can rename the file (restore.SwappableStore). Idempotent.
func (s *ShareLinkStore) PauseForSwap() error {
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

// Replace reopens share_links.db from storageDir after a restore.
func (s *ShareLinkStore) Replace(storageDir string) error {
	fresh, err := NewShareLinkStore(storageDir, s.log)
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
			s.log.Warn("failed to close previous share link store after restore", "error", err)
		}
	}
	return nil
}

// Close closes the database.
func (s *ShareLinkStore) Close() error {
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

func scanLink(row *sql.Row) (*ShareLink, error) {
	var (
		l         ShareLink
		expiresAt sql.NullTime
	)
	err := row.Scan(&l.ID, &l.tokenHash, &l.PageID, &l.IncludeDescendants, &expiresAt, &l.CreatedBy, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errLinkNotStored
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read share link: %w", err)
	}
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		l.ExpiresAt = &t
	}
	l.CreatedAt = l.CreatedAt.UTC()
	return &l, nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}
