package auth

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	_ "modernc.org/sqlite"
)

type SessionStore struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
	cancel     context.CancelFunc
	done       chan struct{}
	log        *slog.Logger
}

func sessionDatabasePath(storageDir string, filename string) string {
	normalizedStorageDir := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	return filepath.Join(normalizedStorageDir, filename)
}

func NewSessionStore(storageDir string) (*SessionStore, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &SessionStore{
		storageDir: storageDir,
		filename:   "sessions.db",
		cancel:     cancel,
		done:       make(chan struct{}),
		log:        slog.Default().With("component", "SessionStore"),
	}

	err := sqliteutil.OpenWithPolicy(sessionDatabasePath(s.storageDir, s.filename), sqliteutil.QuarantineOnCorruption, func() error {
		if err := s.ensureSchema(); err != nil {
			if s.db != nil {
				_ = s.db.Close()
				s.db = nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		cancel()
		return nil, err
	}

	// Cleanup expired sessions periodically
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.CleanupExpiredSessions(); err != nil {
					s.log.Warn("failed to cleanup expired sessions", "error", err)
				}
			}
		}
	}()

	return s, nil

}

func (s *SessionStore) withDB(fn func(db *sql.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		db, err := sql.Open("sqlite", sessionDatabasePath(s.storageDir, s.filename)+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err != nil {
			return err
		}
		s.db = db
	}

	return fn(s.db)
}

func (s *SessionStore) ensureSchema() error {
	if err := s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS sessions (
				id TEXT PRIMARY KEY,          -- jti
				user_id TEXT NOT NULL,
				token_type TEXT NOT NULL,     -- "refresh"
				created_at INTEGER NOT NULL,  -- unix sec
				expires_at INTEGER NOT NULL,  -- unix sec
				revoked_at INTEGER,           -- unix sec, NULL = active
				oidc_id_token TEXT NOT NULL DEFAULT '' -- raw OIDC ID token for this
					-- session, used as the end_session_endpoint's id_token_hint on
					-- logout; empty for a local/invite-issued session. Carried
					-- forward to each new row on refresh (see SessionManager.RefreshToken).
			);

			CREATE INDEX IF NOT EXISTS sessions_user_id_idx
				ON sessions(user_id);
			CREATE INDEX IF NOT EXISTS sessions_user_id_token_type_idx
				ON sessions(user_id, token_type);
		`)
		return err
	}); err != nil {
		return err
	}
	return s.ensureOIDCIDTokenColumn()
}

// ensureOIDCIDTokenColumn additively migrates a pre-existing sessions.db by
// adding oidc_id_token if missing. Idempotent, safe to run on every startup —
// mirrors the pattern in UserStore.ensureOIDCColumns.
func (s *SessionStore) ensureOIDCIDTokenColumn() error {
	return s.withDB(func(db *sql.DB) error {
		existing := map[string]bool{}
		rows, err := db.Query(`PRAGMA table_info(sessions)`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var cid, notNull, pk int
			var name, ctype string
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
				_ = rows.Close()
				return err
			}
			existing[name] = true
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if existing["oidc_id_token"] {
			return nil
		}
		if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN oidc_id_token TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to add column oidc_id_token to sessions table: %w", err)
		}
		return nil
	})
}

func (s *SessionStore) Close() error {
	// Signal the cleanup goroutine to stop
	s.cancel()
	// Wait for the cleanup goroutine to finish
	<-s.done
	// Close the database connection
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

// CreateSession inserts a new session row. oidcIDToken is the raw OIDC ID
// token to associate with it (empty for a local/invite-issued session) — see
// the oidc_id_token column comment in ensureSchema.
func (s *SessionStore) CreateSession(id, userID, tokenType string, expiresAt time.Time, oidcIDToken string) error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			INSERT INTO sessions (id, user_id, token_type, created_at, expires_at, revoked_at, oidc_id_token)
			VALUES (?, ?, ?, ?, ?, NULL, ?);
		`, id, userID, tokenType, time.Now().Unix(), expiresAt.Unix(), oidcIDToken)
		return err
	})
}

// GetOIDCIDToken returns the raw OIDC ID token stored for the given session,
// or "" if the session doesn't exist or has no ID token (a local/invite
// session, or lookup failure) — this is always a best-effort read for an
// optional id_token_hint on logout, never a reason to fail the caller.
func (s *SessionStore) GetOIDCIDToken(id, userID, tokenType string) (string, error) {
	var idToken string
	err := s.withDB(func(db *sql.DB) error {
		return db.QueryRow(`
			SELECT oidc_id_token
			FROM sessions
			WHERE id = ? AND user_id = ? AND token_type = ?;
		`, id, userID, tokenType).Scan(&idToken)
	})
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return idToken, nil
}

func (s *SessionStore) IsActive(id, userID, tokenType string, now time.Time) (bool, error) {
	var expiresAt int64
	var revokedAt sql.NullInt64

	err := s.withDB(func(db *sql.DB) error {
		return db.QueryRow(`
			SELECT expires_at, revoked_at
			FROM sessions
			WHERE id = ? AND user_id = ? AND token_type = ?;
		`, id, userID, tokenType).Scan(&expiresAt, &revokedAt)
	})

	if err == sql.ErrNoRows {
		// no such session
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if revokedAt.Valid {
		return false, nil
	}
	if now.Unix() > expiresAt {
		return false, nil
	}
	return true, nil
}

func (s *SessionStore) RevokeSession(id string) error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			UPDATE sessions
			SET revoked_at = ?
			WHERE id = ? AND revoked_at IS NULL;
		`, time.Now().Unix(), id)
		return err
	})
}

func (s *SessionStore) RevokeAllSessionsForUser(userID string) error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			UPDATE sessions
			SET revoked_at = ?
			WHERE user_id = ? AND revoked_at IS NULL;
		`, time.Now().Unix(), userID)
		return err
	})
}

// RevokeAllSessionsForUserExcept revokes every active session for userID
// except the one identified by exceptID. If exceptID is empty, every session
// is revoked (same as RevokeAllSessionsForUser) — the safe fallback when the
// caller could not identify which session to preserve.
func (s *SessionStore) RevokeAllSessionsForUserExcept(userID, exceptID string) error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			UPDATE sessions
			SET revoked_at = ?
			WHERE user_id = ? AND revoked_at IS NULL AND id != ?;
		`, time.Now().Unix(), userID, exceptID)
		return err
	})
}

// DeleteAllSessions removes every session row regardless of state (active,
// expired, or already revoked) — used after a restore, where the previous
// sessions.db content is no longer meaningful against a swapped-in users.db.
func (s *SessionStore) DeleteAllSessions() error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM sessions;`)
		return err
	})
}

func (s *SessionStore) CleanupExpiredSessions() error {
	now := time.Now()
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			DELETE FROM sessions
			WHERE expires_at <= ?;
		`, now.Unix())
		return err
	})
}
