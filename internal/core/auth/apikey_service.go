package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// dummySecretHash is compared against on an unknown-prefix lookup so Resolve
// does the same amount of work whether or not the prefix exists — otherwise
// an unknown prefix would return faster than a known prefix with a wrong
// secret, letting a caller enumerate which prefixes are registered purely by
// timing responses. Mirrors AuthService's dummyHash for the same reason (see
// auth_service.go).
var dummySecretHash = hashSecret("leafwiki-dummy-api-key-secret-for-timing-equalization")

// hashSecret computes the stored form of an API key secret. The secret is a
// 32-byte cryptographically random value (see generateSecret), not a human
// password — brute-forcing it is already infeasible regardless of hash speed,
// so a slow, adaptive hash (bcrypt) buys no extra security here and only adds
// cost (and, worse, turns a known prefix into a cheap CPU-exhaustion target).
// A fast cryptographic hash compared in constant time is the right tool for a
// high-entropy secret — the same choice this codebase already makes for CSRF
// tokens (see middleware/security/csrf.go).
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// apiKeyTokenPrefix marks a bearer credential as an API key (as opposed to a
// JWT access token), so the Bearer middleware can decide whether to act on it.
const apiKeyTokenPrefix = "lw_"

// apiKeyLastUsedThrottle bounds how often a key's last_used_at is written,
// so a hot key doesn't cause a database write on every request.
const apiKeyLastUsedThrottle = 5 * time.Minute

type APIKeyService struct {
	// mu guards only store — it's the one field Replace swaps after a restore
	// (api_keys.db is part of the snapshot ZIP, mirroring AuthService.mu /
	// userService).
	mu    sync.RWMutex
	store *APIKeyStore
	// authService is depended on (rather than caching a *UserService copy)
	// specifically so owner lookups (Resolve, CreateAPIKey) always go through
	// AuthService.UserService(), which already tracks a live restore's
	// ReplaceUserStore swap. A cached *UserService here would silently go
	// stale after the first restore — users.db is AuthService's data, not
	// APIKeyService's; only api_keys.db is swapped directly by this service
	// (see Replace). NewAPIKeyService is only ever called with a non-nil
	// AuthService (wiki.go constructs API key management inside the same
	// !AuthDisabled block that builds AuthService first).
	authService *AuthService
	log         *slog.Logger
}

func NewAPIKeyService(store *APIKeyStore, authService *AuthService) *APIKeyService {
	return &APIKeyService{store: store, authService: authService, log: slog.Default().With("component", "APIKeyService")}
}

// currentStore returns the current *APIKeyStore under a read lock. Callers
// use the returned pointer directly for the rest of their operation — the
// lock is only held long enough to copy the pointer, never across a query, so
// Replace swapping it mid-request never serializes unrelated requests on this
// mutex. Mirrors AuthService.users().
func (s *APIKeyService) currentStore() *APIKeyStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store
}

func (s *APIKeyService) Close() error {
	return s.currentStore().Close()
}

// PauseForSwap closes the current APIKeyStore's *sql.DB connection and
// prevents it from reconnecting, releasing any OS-level file lock on
// api_keys.db before a live restore renames it. Mirrors
// AuthService.PauseUserStoreForSwap — see that doc comment and
// APIKeyStore.suspend for the full Windows-rename rationale.
func (s *APIKeyService) PauseForSwap() error {
	return s.currentStore().suspend()
}

// Replace opens a fresh APIKeyStore against storageDir/api_keys.db and swaps
// it in, closing the previous one afterward. Used by a live restore after
// api_keys.db has been swapped in on disk (or left untouched, if this
// snapshot didn't capture one) — either way a fresh store re-reads whatever is
// actually on disk now, and NewAPIKeyStore's ensureSchema call brings a
// pre-existing-but-older api_keys.db up to the current schema. Mirrors
// AuthService.ReplaceUserStore.
func (s *APIKeyService) Replace(storageDir string) error {
	newStore, err := NewAPIKeyStore(storageDir)
	if err != nil {
		return err
	}

	s.mu.Lock()
	old := s.store
	s.store = newStore
	s.mu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			s.log.Warn("failed to close previous api key store after restore", "error", err)
		}
	}
	return nil
}

// CreateAPIKeyParams are the admin-supplied inputs for minting a new key.
type CreateAPIKeyParams struct {
	Name      string
	UserID    string // the user this key belongs to and acts as
	Role      string // narrows UserID's role; empty defaults to RoleViewer
	ExpiresAt *time.Time
	CreatedBy string // id of the admin creating the key
}

// CreateAPIKey creates and persists a new API key, returning the stored record
// together with the plaintext token. The token is shown to the caller exactly
// once here — only its SHA-256 hash is ever persisted.
func (s *APIKeyService) CreateAPIKey(p CreateAPIKeyParams) (*APIKey, string, error) {
	role := p.Role
	if role == "" {
		role = RoleViewer
	}
	if !IsValidRole(role) {
		return nil, "", ErrUserInvalidRole
	}
	if _, err := s.authService.UserService().GetUserByID(p.UserID); err != nil {
		return nil, "", err
	}

	id, err := shared.GenerateUniqueID()
	if err != nil {
		return nil, "", err
	}

	secret, err := generateSecret()
	if err != nil {
		return nil, "", err
	}
	hash := hashSecret(secret)
	store := s.currentStore()

	// A random-prefix collision is astronomically unlikely; retry the prefix
	// alone once defensively rather than fail the request outright. The
	// secret and its hash don't depend on the prefix, so they're generated
	// only once above rather than redone on every retry.
	const maxAttempts = 2
	for attempt := 1; ; attempt++ {
		prefix, err := generatePrefix()
		if err != nil {
			return nil, "", err
		}

		key := &APIKey{
			ID:        id,
			Name:      p.Name,
			UserID:    p.UserID,
			Prefix:    prefix,
			KeyHash:   hash,
			Role:      role,
			ExpiresAt: p.ExpiresAt,
			CreatedBy: p.CreatedBy,
			CreatedAt: time.Now(),
		}

		if err := store.CreateAPIKey(key); err != nil {
			if err == ErrAPIKeyPrefixCollision && attempt < maxAttempts {
				continue
			}
			return nil, "", err
		}

		return key, apiKeyTokenPrefix + prefix + "_" + secret, nil
	}
}

func (s *APIKeyService) ListAPIKeys() ([]*APIKey, error) {
	return s.currentStore().ListAll()
}

func (s *APIKeyService) RevokeAPIKey(id string) error {
	return s.currentStore().Revoke(id)
}

// DeleteAllForUser removes every API key owned by userID. Called on user delete.
func (s *APIKeyService) DeleteAllForUser(userID string) error {
	return s.currentStore().DeleteAllForUser(userID)
}

// AsStoreUnavailableErr reports whether err is a store's own "suspended for
// live restore" LocalizedError (APIKeyStore's or, via UserService,
// UserStore's), returning it as a *LocalizedError if so. Resolve uses this to
// propagate the distinct error instead of collapsing it into ErrAPIKeyInvalid;
// exported so callers outside this package (e.g. the Bearer-auth middleware)
// can do the same and read its message, without needing to know the
// underlying error codes themselves.
func AsStoreUnavailableErr(err error) (*sharederrors.LocalizedError, bool) {
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		return nil, false
	}
	if loc.Code == ErrCodeAPIKeyStoreUnavailable || loc.Code == userStoreUnavailableCode {
		return loc, true
	}
	return nil, false
}

// Resolve validates a raw "Authorization: Bearer <token>" value and returns the
// user the key acts as. The returned user's Role is narrowed to the
// intersection of the owning user's role and the key's own role (see
// intersectRole) — a key can only ever be as privileged as its owner, and never
// more privileged than the role it was issued with.
//
// Any malformed token, unknown prefix, or secret mismatch is reported as the
// single ErrAPIKeyInvalid, so a caller cannot distinguish "no such key" from
// "wrong secret" (avoids leaking which prefixes exist). To back that promise,
// the secret is hashed and compared even when the prefix is unknown (against
// dummySecretHash) — otherwise an unknown prefix would return faster than a
// known prefix with a wrong secret, leaking exactly the distinction this
// comment claims is hidden. The one exception is a suspended store (see
// AsStoreUnavailableErr): that short-circuits before the hash compare, but
// safely — the response is already distinguishable by status/content (503
// vs. ErrAPIKeyInvalid), so the early return adds no new timing side-channel.
func (s *APIKeyService) Resolve(token string) (*User, error) {
	prefix, secret, ok := parseKeyToken(token)
	if !ok {
		return nil, ErrAPIKeyInvalid
	}

	store := s.currentStore()
	key, err := store.GetByPrefix(prefix)
	if _, ok := AsStoreUnavailableErr(err); ok {
		return nil, err
	}
	found := err == nil
	if err != nil && err != ErrAPIKeyNotFound {
		s.log.Warn("api key resolve: prefix lookup failed", "error", err)
	}

	storedHash := dummySecretHash
	if found {
		storedHash = key.KeyHash
	}
	match := subtle.ConstantTimeCompare([]byte(hashSecret(secret)), []byte(storedHash)) == 1
	if !found || !match {
		return nil, ErrAPIKeyInvalid
	}

	now := time.Now()
	if key.RevokedAt != nil {
		return nil, ErrAPIKeyRevoked
	}
	if key.ExpiresAt != nil && !now.Before(*key.ExpiresAt) {
		return nil, ErrAPIKeyExpired
	}

	owner, err := s.authService.UserService().GetUserByID(key.UserID)
	if _, ok := AsStoreUnavailableErr(err); ok {
		return nil, err
	}
	if err != nil {
		s.log.Warn("api key resolve: owner lookup failed", "error", err, "keyID", key.ID)
		return nil, ErrAPIKeyInvalid
	}

	if key.LastUsedAt == nil || now.Sub(*key.LastUsedAt) > apiKeyLastUsedThrottle {
		if err := store.TouchLastUsed(key.ID, now); err != nil {
			s.log.Warn("failed to update api key last_used_at", "error", err, "keyID", key.ID)
		}
	}

	effective := *owner
	effective.Role = intersectRole(owner.Role, key.Role)
	effective.Password = ""
	return &effective, nil
}

// ─── pure helpers ────────────────────────────────────────────────────────────

// generatePrefix produces a fresh public, indexed lookup value for a new key.
func generatePrefix() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateSecret produces a fresh 32-byte random secret. It is never stored;
// only hashSecret's output is persisted.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateKeyToken produces a fresh (prefix, secret) pair together. Kept as a
// single call for callers (and tests) that don't need CreateAPIKey's
// generate-secret-once / retry-only-the-prefix split.
func generateKeyToken() (prefix string, secret string, err error) {
	prefix, err = generatePrefix()
	if err != nil {
		return "", "", err
	}
	secret, err = generateSecret()
	if err != nil {
		return "", "", err
	}
	return prefix, secret, nil
}

// LooksLikeAPIKeyToken reports whether raw is shaped like a LeafWiki API key
// token, without validating it. Used by the Bearer middleware to decide
// whether an Authorization header is meant for API-key auth at all, so other
// (or malformed) Bearer credentials fall through to normal auth untouched.
func LooksLikeAPIKeyToken(raw string) bool {
	return strings.HasPrefix(raw, apiKeyTokenPrefix)
}

// parseKeyToken splits a raw "lw_<prefix>_<secret>" token into its two halves.
func parseKeyToken(token string) (prefix, secret string, ok bool) {
	if !strings.HasPrefix(token, apiKeyTokenPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(token, apiKeyTokenPrefix)
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// roleRank orders roles from least to most privileged for intersectRole.
var roleRank = map[string]int{
	RoleViewer: 1,
	RoleEditor: 2,
	RoleAdmin:  3,
}

// intersectRole returns the effective role for a request authenticated by an
// API key: the MORE RESTRICTIVE of the owning user's current role and the
// role the key itself was issued with. This is the core of the permission
// model described in the issue proposal — a key can narrow what its owner
// could otherwise do, but can never widen it. For example: an admin user
// holding a viewer-scoped key can only read through that key; if the same
// user is later demoted to viewer, every one of their keys (regardless of
// the key's own role) is capped at viewer too.
//
// An unrecognized role (userRole or keyRole) fails safe to RoleViewer rather
// than erroring — Resolve has no error return path for this step, and
// refusing the request entirely here would be a bigger behavior change than
// the narrowing itself calls for.
func intersectRole(userRole, keyRole string) string {
	uRank, uOK := roleRank[userRole]
	kRank, kOK := roleRank[keyRole]
	if !uOK || !kOK {
		return RoleViewer
	}
	if uRank < kRank {
		return userRole
	}
	return keyRole
}
