package auth

import (
	"database/sql"
	"path"
	"time"

	_ "modernc.org/sqlite"
)

type SessionStore struct {
	storageDir string
	filename   string
	db         *sql.DB
}

func NewSessionStore(storageDir string) (*SessionStore, error) {
	s := &SessionStore{
		storageDir: storageDir,
		filename:   "sessions.db",
	}
	if err := s.Connect(); err != nil {
		return nil, err
	}
	return s, s.ensureSchema()
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
	`)
	return err
}

func (s *SessionStore) Close() error {
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
