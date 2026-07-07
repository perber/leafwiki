package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/shared"
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
	store *APIKeyStore
	users *UserService
}

func NewAPIKeyService(store *APIKeyStore, users *UserService) *APIKeyService {
	return &APIKeyService{store: store, users: users}
}

func (s *APIKeyService) Close() error {
	return s.store.Close()
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
// once here — only its bcrypt hash is ever persisted.
func (s *APIKeyService) CreateAPIKey(p CreateAPIKeyParams) (*APIKey, string, error) {
	role := p.Role
	if role == "" {
		role = RoleViewer
	}
	if !IsValidRole(role) {
		return nil, "", ErrUserInvalidRole
	}
	if _, err := s.users.GetUserByID(p.UserID); err != nil {
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

		if err := s.store.CreateAPIKey(key); err != nil {
			if err == ErrAPIKeyPrefixCollision && attempt < maxAttempts {
				continue
			}
			return nil, "", err
		}

		return key, apiKeyTokenPrefix + prefix + "_" + secret, nil
	}
}

func (s *APIKeyService) ListAPIKeys() ([]*APIKey, error) {
	return s.store.ListAll()
}

func (s *APIKeyService) RevokeAPIKey(id string) error {
	return s.store.Revoke(id)
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
// comment claims is hidden.
func (s *APIKeyService) Resolve(token string) (*User, error) {
	prefix, secret, ok := parseKeyToken(token)
	if !ok {
		return nil, ErrAPIKeyInvalid
	}

	key, err := s.store.GetByPrefix(prefix)
	found := err == nil
	if err != nil && err != ErrAPIKeyNotFound {
		slog.Default().Warn("api key resolve: prefix lookup failed", "error", err)
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

	owner, err := s.users.GetUserByID(key.UserID)
	if err != nil {
		slog.Default().Warn("api key resolve: owner lookup failed", "error", err, "keyID", key.ID)
		return nil, ErrAPIKeyInvalid
	}

	if key.LastUsedAt == nil || now.Sub(*key.LastUsedAt) > apiKeyLastUsedThrottle {
		if err := s.store.TouchLastUsed(key.ID, now); err != nil {
			slog.Default().Warn("failed to update api key last_used_at", "error", err, "keyID", key.ID)
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
