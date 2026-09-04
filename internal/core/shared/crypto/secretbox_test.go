package crypto

import (
	"errors"
	"strings"
	"testing"
)

func mustBox(t *testing.T, key []byte) *SecretBox {
	t.Helper()
	b, err := NewSecretBox(key)
	if err != nil {
		t.Fatalf("NewSecretBox: %v", err)
	}
	return b
}

func TestNewSecretBox_KeyTooShort_ReturnsError(t *testing.T) {
	if _, err := NewSecretBox([]byte("short")); !errors.Is(err, ErrKeyTooShort) {
		t.Fatalf("expected ErrKeyTooShort, got %v", err)
	}
}

func TestSecretBox_SealOpen_RoundTrips(t *testing.T) {
	b := mustBox(t, []byte("0123456789abcdef0123456789abcdef"))
	for _, plain := range []string{"", "hunter2", strings.Repeat("x", 4096), "ünïcödé 🔐"} {
		sealed, err := b.Seal(plain)
		if err != nil {
			t.Fatalf("Seal(%q): %v", plain, err)
		}
		if sealed == plain && plain != "" {
			t.Fatalf("Seal(%q) returned plaintext unchanged", plain)
		}
		got, err := b.Open(sealed)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if got != plain {
			t.Fatalf("round trip: got %q want %q", got, plain)
		}
	}
}

func TestSecretBox_SealIsNondeterministic(t *testing.T) {
	b := mustBox(t, []byte("0123456789abcdef0123456789abcdef"))
	a, _ := b.Seal("same")
	c, _ := b.Seal("same")
	if a == c {
		t.Fatal("two Seal calls produced identical ciphertext (nonce not random?)")
	}
}

func TestSecretBox_Open_WrongKey_FailsAuthentication(t *testing.T) {
	a := mustBox(t, []byte("0123456789abcdef0123456789abcdef"))
	other := mustBox(t, []byte("ffffffffffffffffffffffffffffffff"))
	sealed, _ := a.Seal("secret")
	if _, err := other.Open(sealed); !errors.Is(err, ErrCiphertextInvalid) {
		t.Fatalf("expected ErrCiphertextInvalid, got %v", err)
	}
}

func TestSecretBox_Open_Tampered_FailsAuthentication(t *testing.T) {
	b := mustBox(t, []byte("0123456789abcdef0123456789abcdef"))
	sealed, _ := b.Seal("secret")
	tampered := sealed[:len(sealed)-2] + "AA"
	if _, err := b.Open(tampered); !errors.Is(err, ErrCiphertextInvalid) {
		t.Fatalf("expected ErrCiphertextInvalid, got %v", err)
	}
}

func TestSecretBox_Open_NotBase64_ReturnsInvalid(t *testing.T) {
	b := mustBox(t, []byte("0123456789abcdef0123456789abcdef"))
	if _, err := b.Open("!!! not base64 !!!"); !errors.Is(err, ErrCiphertextInvalid) {
		t.Fatalf("expected ErrCiphertextInvalid, got %v", err)
	}
}

func TestDeriveKey_DeterministicPerLabel_IndependentAcrossLabels(t *testing.T) {
	secret := []byte("an application signing secret")
	k1a, err := DeriveKey(secret, "leafwiki:test:a")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	k1b, _ := DeriveKey(secret, "leafwiki:test:a")
	k2, _ := DeriveKey(secret, "leafwiki:test:b")

	if len(k1a) != MinKeyLen {
		t.Fatalf("derived key length = %d, want %d", len(k1a), MinKeyLen)
	}
	if string(k1a) != string(k1b) {
		t.Fatal("DeriveKey not deterministic for the same label")
	}
	if string(k1a) == string(k2) {
		t.Fatal("DeriveKey produced the same key for different labels")
	}
}

func TestDeriveKey_ProducesUsableBoxKey(t *testing.T) {
	key, err := DeriveKey([]byte("secret"), "leafwiki:test")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	b := mustBox(t, key)
	sealed, _ := b.Seal("payload")
	got, err := b.Open(sealed)
	if err != nil || got != "payload" {
		t.Fatalf("round trip with derived key failed: got %q err %v", got, err)
	}
}
