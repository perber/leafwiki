// Package crypto provides small, dependency-free primitives for encrypting
// short secrets at rest with an application-level key.
//
// SecretBox wraps AES-256-GCM in the "nonce prefix + base64" envelope format
// already used for TOTP secrets in internal/core/auth. It is deliberately tiny
// and holds no mutable state beyond the derived cipher, so a single instance is
// safe for concurrent use.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// MinKeyLen is the minimum accepted length, in bytes, of the key material
// passed to NewSecretBox. Only the first MinKeyLen bytes are used, as the
// AES-256 key.
const MinKeyLen = 32

// ErrKeyTooShort is returned by NewSecretBox when the supplied key material is
// shorter than MinKeyLen.
var ErrKeyTooShort = fmt.Errorf("encryption key must be at least %d bytes", MinKeyLen)

// ErrCiphertextInvalid is returned by Open when the input is not a well-formed
// ciphertext this box could have produced (bad base64, too short, or failing
// the GCM authentication tag — e.g. wrong key or tampering).
var ErrCiphertextInvalid = errors.New("ciphertext is invalid or was produced with a different key")

// SecretBox seals and opens short secrets with AES-256-GCM.
type SecretBox struct {
	gcm cipher.AEAD
}

// NewSecretBox derives an AES-256-GCM cipher from key. key must be at least
// MinKeyLen bytes; only the first MinKeyLen bytes are used.
func NewSecretBox(key []byte) (*SecretBox, error) {
	if len(key) < MinKeyLen {
		return nil, ErrKeyTooShort
	}
	block, err := aes.NewCipher(key[:MinKeyLen])
	if err != nil {
		return nil, fmt.Errorf("init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init GCM: %w", err)
	}
	return &SecretBox{gcm: gcm}, nil
}

// Seal encrypts plaintext and returns a base64 string of nonce||ciphertext.
func (b *SecretBox) Seal(plaintext string) (string, error) {
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := b.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open reverses Seal. A malformed or unauthenticated input yields
// ErrCiphertextInvalid (wrapped), never a panic.
func (b *SecretBox) Open(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCiphertextInvalid, err)
	}
	nonceSize := b.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrCiphertextInvalid
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := b.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCiphertextInvalid, err)
	}
	return string(plaintext), nil
}

// DeriveKey expands secret into a MinKeyLen-byte key using HKDF-SHA256 with a
// fixed, purpose-specific info label. Different labels yield independent keys
// from the same secret, so one application secret (e.g. the JWT signing key)
// can safely key several unrelated SecretBoxes.
func DeriveKey(secret []byte, info string) ([]byte, error) {
	r := hkdf.New(sha256.New, secret, nil, []byte(info))
	key := make([]byte, MinKeyLen)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("hkdf expand: %w", err)
	}
	return key, nil
}
