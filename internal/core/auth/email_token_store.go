package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared"
	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	_ "modernc.org/sqlite"
)

// Token purposes. A token issued for one purpose is never valid for the
// other — Resolve enforces this explicitly (defense in depth even though
// tokens are already unguessable).
const (
	PurposePasswordReset = "password_reset"
	PurposeInvite        = "invite"
)

// Token lifetimes are internal constants, not operator-configurable flags —
// keeps the SMTP feature's surface area small.
const (
	PasswordResetTokenTTL = time.Hour
	InviteTokenTTL        = 7 * 24 * time.Hour
)

// dummyEmailTokenHash is compared against on an unknown-id lookup so Resolve
// does the same amount of work whether or not the id exists — mirrors
// dummySecretHash in apikey_service.go; see that comment for the full timing
// rationale.
var dummyEmailTokenHash = hashSecret("leafwiki-dummy-email-token-secret-for-timing-equalization")

// EmailToken is the persisted representation of a password-reset or invite
// token. The plaintext secret half is never stored — only tokenHash (a
// SHA-256 hash) is kept.
type EmailToken struct {
	ID         string
	UserID     string
	Purpose    string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time

	tokenHash string
}

// EmailTokenStore persists password-reset and invite tokens in their own
// SQLite file (email_tokens.db), modeled directly on SessionStore: own
// cleanup goroutine with the same ctx/cancel/done shutdown pattern, no
// suspend/replace machinery (these tokens are ephemeral and fine to lose on
// a restore, exactly like sessions.db already is).
type EmailTokenStore struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
	cancel     context.CancelFunc
	done       chan struct{}
	log        *slog.Logger
}

func emailTokenDatabasePath(storageDir, filename string) string {
	normalizedStorageDir := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	return filepath.Join(normalizedStorageDir, filename)
}

func NewEmailTokenStore(storageDir string) (*EmailTokenStore, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &EmailTokenStore{
		storageDir: storageDir,
		filename:   "email_tokens.db",
		cancel:     cancel,
		done:       make(chan struct{}),
		log:        slog.Default().With("component", "EmailTokenStore"),
	}

	err := sqliteutil.RetryOnCorruption(emailTokenDatabasePath(s.storageDir, s.filename), func() error {
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

	go func() {
		defer close(s.done)
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.CleanupExpired(); err != nil {
					s.log.Warn("failed to cleanup expired email tokens", "error", err)
				}
			}
		}
	}()

	return s, nil
}

func (s *EmailTokenStore) withDB(fn func(db *sql.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		db, err := sql.Open("sqlite", emailTokenDatabasePath(s.storageDir, s.filename)+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err != nil {
			return err
		}
		s.db = db
	}

	return fn(s.db)
}

func (s *EmailTokenStore) ensureSchema() error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS email_tokens (
				id TEXT PRIMARY KEY,
				token_hash TEXT NOT NULL,
				user_id TEXT NOT NULL,
				purpose TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				expires_at INTEGER NOT NULL,
				consumed_at INTEGER
			);

			CREATE INDEX IF NOT EXISTS email_tokens_user_id_idx
				ON email_tokens(user_id);
		`)
		return err
	})
}

func (s *EmailTokenStore) Close() error {
	s.cancel()
	<-s.done

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

// Issue creates a new token for userID/purpose valid for ttl and returns the
// URL-safe raw token ("id.secret") — only the SHA-256 hash of the secret is
// ever persisted.
func (s *EmailTokenStore) Issue(userID, purpose string, ttl time.Duration) (rawToken string, err error) {
	id, err := shared.GenerateUniqueID()
	if err != nil {
		return "", err
	}
	secret, err := generateSecret()
	if err != nil {
		return "", err
	}
	hash := hashSecret(secret)

	now := time.Now()
	err = s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			INSERT INTO email_tokens (id, token_hash, user_id, purpose, created_at, expires_at, consumed_at)
			VALUES (?, ?, ?, ?, ?, ?, NULL);
		`, id, hash, userID, purpose, now.Unix(), now.Add(ttl).Unix())
		return err
	})
	if err != nil {
		return "", err
	}

	return id + "." + secret, nil
}

// Resolve validates rawToken (as produced by Issue) for purpose and returns
// the token record if it is well-formed, known, matches purpose, and is
// neither expired nor already consumed. Every failure mode collapses into
// the single ErrEmailTokenInvalid so a caller cannot distinguish "unknown
// token" from "wrong purpose" from "expired" from "already used" — the
// secret is hashed and compared against a decoy hash even when the id is
// unknown, so an unknown id takes the same time as a known id with a wrong
// secret (mirrors APIKeyService.Resolve).
func (s *EmailTokenStore) Resolve(rawToken, purpose string) (*EmailToken, error) {
	id, secret, ok := parseEmailToken(rawToken)
	if !ok {
		return nil, ErrEmailTokenInvalid
	}

	tok, err := s.getByID(id)
	found := err == nil
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	storedHash := dummyEmailTokenHash
	if found {
		storedHash = tok.tokenHash
	}
	match := subtle.ConstantTimeCompare([]byte(hashSecret(secret)), []byte(storedHash)) == 1
	if !found || !match {
		return nil, ErrEmailTokenInvalid
	}

	if tok.Purpose != purpose {
		return nil, ErrEmailTokenInvalid
	}
	if tok.ConsumedAt != nil {
		return nil, ErrEmailTokenInvalid
	}
	if time.Now().After(tok.ExpiresAt) {
		return nil, ErrEmailTokenInvalid
	}

	return tok, nil
}

// Consume atomically marks id's row consumed. This is a DB-level
// compare-and-swap (UPDATE ... WHERE consumed_at IS NULL + RowsAffected==1),
// not check-then-write — mirrors UserStore.ConsumeRecoveryCodeHash — so two
// concurrent requests racing to consume the same token can't both succeed.
func (s *EmailTokenStore) Consume(id string) error {
	var rowsAffected int64
	err := s.withDB(func(db *sql.DB) error {
		result, err := db.Exec(`
			UPDATE email_tokens SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL;
		`, time.Now().Unix(), id)
		if err != nil {
			return err
		}
		rowsAffected, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return ErrEmailTokenInvalid
	}
	return nil
}

// CleanupExpired purges expired token rows regardless of consumed state.
func (s *EmailTokenStore) CleanupExpired() error {
	now := time.Now()
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM email_tokens WHERE expires_at <= ?;`, now.Unix())
		return err
	})
}

func (s *EmailTokenStore) getByID(id string) (*EmailToken, error) {
	tok := &EmailToken{}
	err := s.withDB(func(db *sql.DB) error {
		var createdAt, expiresAt int64
		var consumedAt sql.NullInt64
		err := db.QueryRow(`
			SELECT id, token_hash, user_id, purpose, created_at, expires_at, consumed_at
			FROM email_tokens WHERE id = ?;
		`, id).Scan(&tok.ID, &tok.tokenHash, &tok.UserID, &tok.Purpose, &createdAt, &expiresAt, &consumedAt)
		if err != nil {
			return err
		}
		tok.CreatedAt = time.Unix(createdAt, 0)
		tok.ExpiresAt = time.Unix(expiresAt, 0)
		if consumedAt.Valid {
			t := time.Unix(consumedAt.Int64, 0)
			tok.ConsumedAt = &t
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tok, nil
}

// parseEmailToken splits a raw "id.secret" token into its two halves.
func parseEmailToken(raw string) (id, secret string, ok bool) {
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
