package auth

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type APIKey struct {
	ID              string     `json:"id"`
	UserID          string     `json:"userId"`
	Name            string     `json:"name"`
	Prefix          string     `json:"prefix"`
	Last4           string     `json:"last4"`
	Scopes          []string   `json:"scopes"`
	CreatedByUserID string     `json:"createdByUserId"`
	CreatedAt       time.Time  `json:"createdAt"`
	LastUsedAt      *time.Time `json:"lastUsedAt"`
	RevokedAt       *time.Time `json:"revokedAt"`
}

type storedAPIKey struct {
	key        *APIKey
	secretHash string
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
		if s.db != nil {
			_ = s.db.Close()
			s.db = nil
		}
		return nil, err
	}
	return s, nil
}

func (s *APIKeyStore) Connect() error {
	if s.db != nil {
		return nil
	}
	db, err := sql.Open("sqlite", databasePath(s.storageDir, s.filename))
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

func (s *APIKeyStore) ensureSchema() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Connect(); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			secret_hash TEXT NOT NULL,
			prefix TEXT NOT NULL,
			last4 TEXT NOT NULL,
			scopes TEXT NOT NULL,
			created_by_user_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			last_used_at INTEGER,
			revoked_at INTEGER
		);

		CREATE INDEX IF NOT EXISTS api_keys_user_id_active_idx
			ON api_keys(user_id, revoked_at);
		CREATE INDEX IF NOT EXISTS api_keys_created_by_user_id_idx
			ON api_keys(created_by_user_id);
	`)
	return err
}

func (s *APIKeyStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	if err != nil {
		return err
	}
	s.db = nil
	return nil
}

func (s *APIKeyStore) CreateAPIKey(key *APIKey, secretHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Connect(); err != nil {
		return err
	}
	scopes, err := json.Marshal(key.Scopes)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO api_keys (
			id, user_id, name, secret_hash, prefix, last4, scopes,
			created_by_user_id, created_at, last_used_at, revoked_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL);
	`, key.ID, key.UserID, key.Name, secretHash, key.Prefix, key.Last4, string(scopes),
		key.CreatedByUserID, key.CreatedAt.Unix())
	return err
}

func (s *APIKeyStore) ListActiveAPIKeys(userID string) ([]*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Connect(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, name, prefix, last4, scopes, created_by_user_id,
		       created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE user_id = ? AND revoked_at IS NULL
		ORDER BY created_at DESC, id DESC;
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error("could not close api key rows", "error", err)
		}
	}()

	keys := make([]*APIKey, 0)
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *APIKeyStore) GetAPIKeyByID(id string) (*storedAPIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Connect(); err != nil {
		return nil, err
	}
	row := s.db.QueryRow(`
		SELECT id, user_id, name, secret_hash, prefix, last4, scopes,
		       created_by_user_id, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE id = ?;
	`, id)
	key, secretHash, err := scanStoredAPIKey(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	return &storedAPIKey{key: key, secretHash: secretHash}, nil
}

func (s *APIKeyStore) RevokeAPIKey(userID, keyID string, revokedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Connect(); err != nil {
		return err
	}
	result, err := s.db.Exec(`
		UPDATE api_keys
		SET revoked_at = ?
		WHERE id = ? AND user_id = ? AND revoked_at IS NULL;
	`, revokedAt.Unix(), keyID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

func (s *APIKeyStore) MarkAPIKeyUsed(keyID string, usedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Connect(); err != nil {
		return err
	}
	result, err := s.db.Exec(`
		UPDATE api_keys
		SET last_used_at = ?
		WHERE id = ? AND revoked_at IS NULL;
	`, usedAt.Unix(), keyID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

type apiKeyScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(scanner apiKeyScanner) (*APIKey, error) {
	var key APIKey
	var scopesRaw string
	var createdAt int64
	var lastUsedAt sql.NullInt64
	var revokedAt sql.NullInt64

	err := scanner.Scan(
		&key.ID,
		&key.UserID,
		&key.Name,
		&key.Prefix,
		&key.Last4,
		&scopesRaw,
		&key.CreatedByUserID,
		&createdAt,
		&lastUsedAt,
		&revokedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(scopesRaw), &key.Scopes); err != nil {
		return nil, err
	}
	hydrateAPIKeyTimes(&key, createdAt, lastUsedAt, revokedAt)
	return &key, nil
}

func scanStoredAPIKey(scanner apiKeyScanner) (*APIKey, string, error) {
	var key APIKey
	var secretHash string
	var scopesRaw string
	var createdAt int64
	var lastUsedAt sql.NullInt64
	var revokedAt sql.NullInt64

	err := scanner.Scan(
		&key.ID,
		&key.UserID,
		&key.Name,
		&secretHash,
		&key.Prefix,
		&key.Last4,
		&scopesRaw,
		&key.CreatedByUserID,
		&createdAt,
		&lastUsedAt,
		&revokedAt,
	)
	if err != nil {
		return nil, "", err
	}
	if err := json.Unmarshal([]byte(scopesRaw), &key.Scopes); err != nil {
		return nil, "", err
	}
	hydrateAPIKeyTimes(&key, createdAt, lastUsedAt, revokedAt)
	return &key, secretHash, nil
}

func hydrateAPIKeyTimes(key *APIKey, createdAt int64, lastUsedAt, revokedAt sql.NullInt64) {
	key.CreatedAt = time.Unix(createdAt, 0).UTC()
	if lastUsedAt.Valid {
		t := time.Unix(lastUsedAt.Int64, 0).UTC()
		key.LastUsedAt = &t
	}
	if revokedAt.Valid {
		t := time.Unix(revokedAt.Int64, 0).UTC()
		key.RevokedAt = &t
	}
}
