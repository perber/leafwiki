package sharing

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// A token is tokenPrefix plus 256 random bits, base64url without padding. The
// prefix makes a leaked token recognizable in logs and by secret scanners.
const (
	tokenPrefix      = "lws_"
	tokenSecretBytes = 32
)

var tokenLength = len(tokenPrefix) + base64.RawURLEncoding.EncodedLen(tokenSecretBytes)

func newToken() (string, error) {
	secret := make([]byte, tokenSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(secret), nil
}

// hashToken returns the stored form of a token. A fast hash is enough for a
// 256-bit random secret (same reasoning as auth.hashSecret for API keys).
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// looksLikeToken rejects anything newToken could not have produced before it
// reaches the database.
func looksLikeToken(token string) bool {
	if len(token) != tokenLength || !strings.HasPrefix(token, tokenPrefix) {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(token[len(tokenPrefix):])
	return err == nil
}
