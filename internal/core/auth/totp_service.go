// TOTPService generates and verifies per-user TOTP secrets and recovery codes
// for the optional native-login two-factor authentication feature. Secrets
// are encrypted at rest with an application-level AES-256-GCM key so that a
// stolen users.db file alone cannot be used to generate valid codes; the login
// handshake and self-service setup/disable flows that consume this service
// are added in later phases.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	totpIssuer = "LeafWiki"

	// MinTOTPEncryptionKeyLen is the minimum accepted length, in bytes, for the
	// operator-supplied key used to encrypt TOTP secrets at rest.
	MinTOTPEncryptionKeyLen = 32

	defaultRecoveryCodeCount = 10
	recoveryCodeByteLen      = 5 // 5 random bytes -> 8 base32 chars, formatted as XXXX-XXXX
)

// TOTPService is safe for concurrent use; it holds no mutable state beyond the
// AES-GCM cipher derived once from the configured encryption key.
type TOTPService struct {
	gcm cipher.AEAD
}

// NewTOTPService creates a TOTPService that encrypts/decrypts TOTP secrets
// with encryptionKey. The key must be at least MinTOTPEncryptionKeyLen bytes;
// only the first MinTOTPEncryptionKeyLen bytes are used as the AES-256-GCM key.
func NewTOTPService(encryptionKey []byte) (*TOTPService, error) {
	if len(encryptionKey) < MinTOTPEncryptionKeyLen {
		return nil, fmt.Errorf("TOTP encryption key must be at least %d bytes, got %d", MinTOTPEncryptionKeyLen, len(encryptionKey))
	}
	block, err := aes.NewCipher(encryptionKey[:MinTOTPEncryptionKeyLen])
	if err != nil {
		return nil, fmt.Errorf("failed to initialize TOTP secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize TOTP secret cipher: %w", err)
	}
	return &TOTPService{gcm: gcm}, nil
}

// GeneratedSecret carries a freshly generated TOTP secret in the forms the
// setup flow needs. The plaintext secret is never returned or persisted
// directly; only EncryptedSecret is stored, and OTPAuthURL is used by the
// frontend to render a QR code for the authenticator app. Secret (the
// plaintext, base32-encoded form) is only ever handed to the setup-start HTTP
// response for manual entry — it is never persisted; storage uses
// EncryptedSecret exclusively.
type GeneratedSecret struct {
	Secret          string
	EncryptedSecret string
	OTPAuthURL      string
}

// GenerateSecret creates a new random TOTP secret for accountName (typically
// the user's username or email) and returns it pre-encrypted for storage,
// plus the otpauth:// URI for QR-code rendering.
func (s *TOTPService) GenerateSecret(accountName string) (*GeneratedSecret, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	encrypted, err := s.encrypt(key.Secret())
	if err != nil {
		return nil, err
	}

	return &GeneratedSecret{
		Secret:          key.Secret(),
		EncryptedSecret: encrypted,
		OTPAuthURL:      key.URL(),
	}, nil
}

// VerifyCode reports whether code is a valid TOTP code for encryptedSecret at
// the current time, tolerating one adjacent 30s window for clock drift.
// A returned error means the secret could not be decrypted (a configuration
// or data-integrity problem). A malformed code (wrong length, non-numeric —
// e.g. a recovery code typed into the TOTP field) is not an error, just an
// invalid one: reported as (false, nil), same as a wrong-but-well-formed code.
func (s *TOTPService) VerifyCode(encryptedSecret, code string) (bool, error) {
	secret, err := s.decrypt(encryptedSecret)
	if err != nil {
		return false, err
	}
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false, nil
	}
	return valid, nil
}

// GenerateRecoveryCodes returns n freshly generated recovery codes in
// plaintext (to be shown to the user exactly once) and their bcrypt hashes
// (the only form that should ever be persisted). If n <= 0, defaultRecoveryCodeCount is used.
func (s *TOTPService) GenerateRecoveryCodes(n int) (codes []string, hashes []string, err error) {
	if n <= 0 {
		n = defaultRecoveryCodeCount
	}

	codes = make([]string, 0, n)
	hashes = make([]string, 0, n)
	for i := 0; i < n; i++ {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate recovery code: %w", err)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to hash recovery code: %w", err)
		}
		codes = append(codes, code)
		hashes = append(hashes, string(hash))
	}
	return codes, hashes, nil
}

func generateRecoveryCode() (string, error) {
	b := make([]byte, recoveryCodeByteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	encoded := strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
	// 5 bytes -> 8 base32 chars; split into two groups of 4 for readability, e.g. "ABCD-EFGH".
	return encoded[:4] + "-" + encoded[4:8], nil
}

// VerifyRecoveryCode reports whether code matches any of hashes. On a match,
// matchedIndex is the position of the matched hash so the caller can remove
// it (via UserStore.UpdateRecoveryCodeHashes) to enforce single use.
func VerifyRecoveryCode(code string, hashes []string) (matchedIndex int, ok bool) {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	for i, hash := range hashes {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(normalized)) == nil {
			return i, true
		}
	}
	return -1, false
}

func (s *TOTPService) encrypt(plaintext string) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce for TOTP secret encryption: %w", err)
	}
	ciphertext := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *TOTPService) decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted TOTP secret: %w", err)
	}
	nonceSize := s.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("encrypted TOTP secret is too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := s.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt TOTP secret: %w", err)
	}
	return string(plaintext), nil
}
