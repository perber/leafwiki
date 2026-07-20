package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func testEncryptionKey() []byte {
	return []byte("01234567890123456789012345678901") // 32 bytes
}

func TestNewTOTPService_RejectsShortKey(t *testing.T) {
	if _, err := NewTOTPService([]byte("too-short")); err == nil {
		t.Fatal("expected error for encryption key shorter than MinTOTPEncryptionKeyLen, got nil")
	}
}

func TestNewTOTPService_AcceptsMinimumLengthKey(t *testing.T) {
	if _, err := NewTOTPService(testEncryptionKey()); err != nil {
		t.Fatalf("expected 32-byte key to be accepted, got error: %v", err)
	}
}

func TestTOTPService_GenerateSecret_ReturnsOTPAuthURLAndEncryptedSecret(t *testing.T) {
	svc, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create TOTPService: %v", err)
	}

	generated, err := svc.GenerateSecret("alice@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	if generated.EncryptedSecret == "" {
		t.Fatal("expected non-empty encrypted secret")
	}
	if !strings.HasPrefix(generated.OTPAuthURL, "otpauth://totp/") {
		t.Fatalf("expected otpauth:// URL, got %q", generated.OTPAuthURL)
	}
	if !strings.Contains(generated.OTPAuthURL, "LeafWiki") {
		t.Fatalf("expected issuer LeafWiki in otpauth URL, got %q", generated.OTPAuthURL)
	}
	if !strings.Contains(generated.OTPAuthURL, "alice@example.com") {
		t.Fatalf("expected account name in otpauth URL, got %q", generated.OTPAuthURL)
	}

	// The plaintext secret must never be embedded in the returned URL query's
	// "secret" param in a way that also appears verbatim as EncryptedSecret.
	if strings.Contains(generated.OTPAuthURL, generated.EncryptedSecret) {
		t.Fatal("encrypted secret leaked into otpauth URL")
	}
}

func TestTOTPService_VerifyCode_AcceptsValidCurrentCode(t *testing.T) {
	svc, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create TOTPService: %v", err)
	}

	generated, err := svc.GenerateSecret("bob@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	plainSecret, err := svc.decrypt(generated.EncryptedSecret)
	if err != nil {
		t.Fatalf("failed to decrypt secret for test setup: %v", err)
	}

	code, err := totp.GenerateCode(plainSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate reference TOTP code: %v", err)
	}

	valid, err := svc.VerifyCode(generated.EncryptedSecret, code)
	if err != nil {
		t.Fatalf("VerifyCode returned unexpected error: %v", err)
	}
	if !valid {
		t.Fatal("expected valid current TOTP code to verify")
	}
}

func TestTOTPService_VerifyCode_RejectsWrongCode(t *testing.T) {
	svc, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create TOTPService: %v", err)
	}

	generated, err := svc.GenerateSecret("carol@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	valid, err := svc.VerifyCode(generated.EncryptedSecret, "000000")
	if err != nil {
		t.Fatalf("VerifyCode returned unexpected error: %v", err)
	}
	if valid {
		t.Fatal("expected wrong code to be rejected")
	}
}

func TestTOTPService_VerifyCode_TolerantOfOneStepClockDrift(t *testing.T) {
	svc, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create TOTPService: %v", err)
	}

	generated, err := svc.GenerateSecret("dave@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}
	plainSecret, err := svc.decrypt(generated.EncryptedSecret)
	if err != nil {
		t.Fatalf("failed to decrypt secret for test setup: %v", err)
	}

	// One period (30s) in the past must still validate (Skew: 1).
	pastCode, err := totp.GenerateCode(plainSecret, time.Now().Add(-30*time.Second))
	if err != nil {
		t.Fatalf("failed to generate past TOTP code: %v", err)
	}
	valid, err := svc.VerifyCode(generated.EncryptedSecret, pastCode)
	if err != nil {
		t.Fatalf("VerifyCode returned unexpected error: %v", err)
	}
	if !valid {
		t.Fatal("expected code from one period ago to be accepted within clock-skew tolerance")
	}

	// Three periods (90s) in the past must be rejected.
	staleCode, err := totp.GenerateCode(plainSecret, time.Now().Add(-90*time.Second))
	if err != nil {
		t.Fatalf("failed to generate stale TOTP code: %v", err)
	}
	valid, err = svc.VerifyCode(generated.EncryptedSecret, staleCode)
	if err != nil {
		t.Fatalf("VerifyCode returned unexpected error: %v", err)
	}
	if valid {
		t.Fatal("expected stale code well outside clock-skew tolerance to be rejected")
	}
}

func TestTOTPService_VerifyCode_ErrorsOnUndecryptableSecret(t *testing.T) {
	svc, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create TOTPService: %v", err)
	}

	if _, err := svc.VerifyCode("not-valid-base64-or-ciphertext!!", "123456"); err == nil {
		t.Fatal("expected error when the encrypted secret cannot be decrypted")
	}
}

func TestTOTPService_SecretsAreIsolatedPerEncryptionKey(t *testing.T) {
	svcA, err := NewTOTPService([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	if err != nil {
		t.Fatalf("failed to create TOTPService A: %v", err)
	}
	svcB, err := NewTOTPService([]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"))
	if err != nil {
		t.Fatalf("failed to create TOTPService B: %v", err)
	}

	generated, err := svcA.GenerateSecret("eve@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	if _, err := svcB.decrypt(generated.EncryptedSecret); err == nil {
		t.Fatal("expected decryption with a different key to fail")
	}
}

func TestTOTPService_GenerateRecoveryCodes(t *testing.T) {
	svc, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create TOTPService: %v", err)
	}

	codes, hashes, err := svc.GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes failed: %v", err)
	}
	if len(codes) != 8 || len(hashes) != 8 {
		t.Fatalf("expected 8 codes and 8 hashes, got %d codes, %d hashes", len(codes), len(hashes))
	}

	seen := map[string]bool{}
	for i, code := range codes {
		if seen[code] {
			t.Fatalf("recovery code %q generated more than once", code)
		}
		seen[code] = true

		if len(code) != 9 || code[4] != '-' {
			t.Fatalf("expected recovery code in XXXX-XXXX format, got %q", code)
		}

		if _, ok := VerifyRecoveryCode(code, []string{hashes[i]}); !ok {
			t.Fatalf("expected generated code %q to verify against its own hash", code)
		}
	}
}

func TestTOTPService_GenerateRecoveryCodes_DefaultsWhenNonPositive(t *testing.T) {
	svc, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create TOTPService: %v", err)
	}

	codes, hashes, err := svc.GenerateRecoveryCodes(0)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes failed: %v", err)
	}
	if len(codes) != defaultRecoveryCodeCount || len(hashes) != defaultRecoveryCodeCount {
		t.Fatalf("expected default count %d, got %d codes, %d hashes", defaultRecoveryCodeCount, len(codes), len(hashes))
	}
}

func TestVerifyRecoveryCode(t *testing.T) {
	svc, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create TOTPService: %v", err)
	}

	codes, hashes, err := svc.GenerateRecoveryCodes(3)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes failed: %v", err)
	}

	idx, ok := VerifyRecoveryCode(codes[1], hashes)
	if !ok || idx != 1 {
		t.Fatalf("expected match at index 1, got ok=%v idx=%d", ok, idx)
	}

	// Case-insensitive / whitespace-tolerant matching.
	idx, ok = VerifyRecoveryCode(strings.ToLower(" "+codes[2]+" "), hashes)
	if !ok || idx != 2 {
		t.Fatalf("expected case/whitespace-insensitive match at index 2, got ok=%v idx=%d", ok, idx)
	}

	if _, ok := VerifyRecoveryCode("ZZZZ-ZZZZ", hashes); ok {
		t.Fatal("expected unknown recovery code to not match")
	}
}
