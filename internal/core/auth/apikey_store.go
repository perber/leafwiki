package auth

import (
	"database/sql"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// APIKey is the persisted representation of an API key. The plaintext secret
// is never stored — only KeyHash (a SHA-256 hash of the secret half of the
// token) is kept, so a leaked database yields no usable keys.
type APIKey struct {
	ID         string
	Name       string
	UserID     string // the user this key belongs to and acts as
	Prefix     string // public, indexed lookup value
	KeyHash    string // SHA-256 hash of the secret
	Role       string // narrows UserID's role; never widens it
	ExpiresAt  *time.Time
	CreatedBy  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// IsActive reports whether the key can currently be used: not revoked and,
// if it has an expiry, not yet expired as of now.
func (k *APIKey) IsActive(now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && !now.Before(*k.ExpiresAt) {
		return false
	}
	return true
}

type APIKeyStore struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
}

func NewAPIKeyStore(storageDir string) (*APIKeyStore, error) {
	s := &APIKeyStore{
		storageDir: storageDir,
		filename:   "api_keys.db",
	}

	if err := s.ensureSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *APIKeyStore) withDB(fn func(db *sql.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		db, err := sql.Open("sqlite", databasePath(s.storageDir, s.filename))
		if err != nil {
			return err
		}
		s.db = db
	}

	return fn(s.db)
}

func (s *APIKeyStore) ensureSchema() error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS api_keys (
				id           TEXT PRIMARY KEY,
				name         TEXT NOT NULL,
				user_id      TEXT NOT NULL,
				prefix       TEXT NOT NULL UNIQUE,
				key_hash     TEXT NOT NULL,
				role         TEXT NOT NULL,
				expires_at   INTEGER,          -- unix sec, NULL = never expires
				created_by   TEXT NOT NULL,
				created_at   INTEGER NOT NULL, -- unix sec
				last_used_at INTEGER,          -- unix sec, NULL = never used
				revoked_at   INTEGER           -- unix sec, NULL = active
			);
		`)
		return err
	})
}

func (s *APIKeyStore) Close() error {
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

func (s *APIKeyStore) CreateAPIKey(key *APIKey) error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			INSERT INTO api_keys (id, name, user_id, prefix, key_hash, role, expires_at, created_by, created_at, last_used_at, revoked_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
		`, key.ID, key.Name, key.UserID, key.Prefix, key.KeyHash, key.Role,
			timeToNullInt64(key.ExpiresAt), key.CreatedBy, key.CreatedAt.Unix(),
			timeToNullInt64(key.LastUsedAt), timeToNullInt64(key.RevokedAt))
		if err != nil {
			return s.mapConstraintViolationToError(err)
		}
		return nil
	})
}

func (s *APIKeyStore) GetByPrefix(prefix string) (*APIKey, error) {
	var key *APIKey
	err := s.withDB(func(db *sql.DB) error {
		row := db.QueryRow(`
			SELECT id, name, user_id, prefix, key_hash, role, expires_at, created_by, created_at, last_used_at, revoked_at
			FROM api_keys
			WHERE prefix = ?;
		`, prefix)
		var scanErr error
		key, scanErr = scanAPIKey(row)
		return scanErr
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	return key, nil
}

func (s *APIKeyStore) GetByID(id string) (*APIKey, error) {
	var key *APIKey
	err := s.withDB(func(db *sql.DB) error {
		row := db.QueryRow(`
			SELECT id, name, user_id, prefix, key_hash, role, expires_at, created_by, created_at, last_used_at, revoked_at
			FROM api_keys
			WHERE id = ?;
		`, id)
		var scanErr error
		key, scanErr = scanAPIKey(row)
		return scanErr
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	return key, nil
}

func (s *APIKeyStore) ListAll() ([]*APIKey, error) {
	var keys []*APIKey
	err := s.withDB(func(db *sql.DB) error {
		rows, err := db.Query(`
			SELECT id, name, user_id, prefix, key_hash, role, expires_at, created_by, created_at, last_used_at, revoked_at
			FROM api_keys
			ORDER BY created_at DESC;
		`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			key, err := scanAPIKey(rows)
			if err != nil {
				return err
			}
			keys = append(keys, key)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// Revoke marks a key as revoked, effective immediately. Revoking an
// already-revoked key is a no-op (the original revocation time is kept).
// Returns ErrAPIKeyNotFound if no key with this id exists.
func (s *APIKeyStore) Revoke(id string) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			UPDATE api_keys
			SET revoked_at = ?
			WHERE id = ? AND revoked_at IS NULL;
		`, time.Now().Unix(), id)
		return err
	})
}

// TouchLastUsed records that a key was just used. Throttling (to avoid a
// write on every request) is the caller's responsibility.
func (s *APIKeyStore) TouchLastUsed(id string, at time.Time) error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			UPDATE api_keys
			SET last_used_at = ?
			WHERE id = ?;
		`, at.Unix(), id)
		return err
	})
}

func (s *APIKeyStore) mapConstraintViolationToError(err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed: api_keys.prefix") {
		return ErrAPIKeyPrefixCollision
	}
	return err
}

// rowScanner abstracts over *sql.Row and *sql.Rows so scanAPIKey can serve both.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(row rowScanner) (*APIKey, error) {
	key := &APIKey{}
	var expiresAt, lastUsedAt, revokedAt sql.NullInt64
	var createdAt int64

	if err := row.Scan(&key.ID, &key.Name, &key.UserID, &key.Prefix, &key.KeyHash, &key.Role,
		&expiresAt, &key.CreatedBy, &createdAt, &lastUsedAt, &revokedAt); err != nil {
		return nil, err
	}

	key.CreatedAt = time.Unix(createdAt, 0)
	key.ExpiresAt = nullInt64ToTime(expiresAt)
	key.LastUsedAt = nullInt64ToTime(lastUsedAt)
	key.RevokedAt = nullInt64ToTime(revokedAt)
	return key, nil
}

func timeToNullInt64(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

func nullInt64ToTime(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.Unix(v.Int64, 0)
	return &t
}
