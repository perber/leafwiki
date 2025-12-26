package auth

import (
	"context"
	"database/sql"
	"log"
	"path"
	"time"

	_ "modernc.org/sqlite"
)

type SessionStore struct {
	storageDir string
	filename   string
	db         *sql.DB
	cancel     context.CancelFunc
	done       chan struct{}
}

func NewSessionStore(storageDir string) (*SessionStore, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &SessionStore{
		storageDir: storageDir,
		filename:   "sessions.db",
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	if err := s.Connect(); err != nil {
		cancel()
		return nil, err
	}

	err := s.ensureSchema()
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
					log.Printf("failed to cleanup expired sessions: %v", err)
				}
			}
		}
	}()

	return s, nil

}

func (s *SessionStore) Connect() error {
	if s.db != nil {
		return nil
	}
	db, err := sql.Open("sqlite", path.Join(s.storageDir, s.filename))
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

func (s *SessionStore) ensureSchema() error {
	if err := s.Connect(); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,          -- jti
			user_id TEXT NOT NULL,
			token_type TEXT NOT NULL,     -- "refresh"
			created_at INTEGER NOT NULL,  -- unix sec
			expires_at INTEGER NOT NULL,  -- unix sec
			revoked_at INTEGER            -- unix sec, NULL = active
		);

		CREATE INDEX IF NOT EXISTS sessions_user_id_idx
			ON sessions(user_id);
		CREATE INDEX IF NOT EXISTS sessions_user_id_token_type_idx
			ON sessions(user_id, token_type);
	`)
	return err
}

func (s *SessionStore) Close() error {
	// Signal the cleanup goroutine to stop
	if s.cancel != nil {
		s.cancel()
	}
	// Wait for the cleanup goroutine to finish
	if s.done != nil {
		<-s.done
	}
	// Close the database connection
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return err
		}
		s.db = nil
	}
	return nil
}

func (s *SessionStore) CreateSession(id, userID, tokenType string, expiresAt time.Time) error {
	if err := s.Connect(); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO sessions (id, user_id, token_type, created_at, expires_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, NULL);
	`, id, userID, tokenType, time.Now().Unix(), expiresAt.Unix())
	return err
}

func (s *SessionStore) IsActive(id, userID, tokenType string, now time.Time) (bool, error) {
	if err := s.Connect(); err != nil {
		return false, err
	}
	var expiresAt int64
	var revokedAt sql.NullInt64

	err := s.db.QueryRow(`
		SELECT expires_at, revoked_at
		FROM sessions
		WHERE id = ? AND user_id = ? AND token_type = ?;
	`, id, userID, tokenType).Scan(&expiresAt, &revokedAt)

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
	if err := s.Connect(); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		UPDATE sessions
		SET revoked_at = ?
		WHERE id = ? AND revoked_at IS NULL;
	`, time.Now().Unix(), id)
	return err
}

func (s *SessionStore) RevokeAllSessionsForUser(userID string) error {
	if err := s.Connect(); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		UPDATE sessions
		SET revoked_at = ?
		WHERE user_id = ? AND revoked_at IS NULL;
	`, time.Now().Unix(), userID)
	return err
}

func (s *SessionStore) CleanupExpiredSessions() error {
	now := time.Now()
	if err := s.Connect(); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		DELETE FROM sessions
		WHERE expires_at <= ?;
	`, now.Unix())
	return err
}
