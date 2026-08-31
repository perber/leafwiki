package backup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sharedcrypto "github.com/perber/wiki/internal/core/shared/crypto"
)

func testBox(t *testing.T) *sharedcrypto.SecretBox {
	t.Helper()
	key, err := sharedcrypto.DeriveKey([]byte("test-jwt-secret"), "leafwiki:git-backup-credentials:v1")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	box, err := sharedcrypto.NewSecretBox(key)
	if err != nil {
		t.Fatalf("NewSecretBox: %v", err)
	}
	return box
}

func TestConfigStore_Load_MissingFile_ReturnsDisabledNoError(t *testing.T) {
	s := NewConfigStore(t.TempDir(), testBox(t))
	cfg, enabled, err := s.Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if enabled {
		t.Fatal("expected enabled=false for a missing config file")
	}
	if cfg != (Config{}) {
		t.Fatalf("expected zero Config, got %+v", cfg)
	}
}

func TestConfigStore_SaveLoad_RoundTripsIncludingSecrets(t *testing.T) {
	dir := t.TempDir()
	s := NewConfigStore(dir, testBox(t))

	in := Config{
		Enabled:      true,
		AuthorName:   "Backup Bot",
		AuthorEmail:  "bot@example.com",
		RemoteURL:    "https://github.com/acme/wiki-backup.git",
		Branch:       "main",
		HTTPUsername: "acme-bot",
		HTTPPassword: "ghp_supersecrettoken",
		Interval:     15 * time.Minute,
	}
	if err := s.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, enabled, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !enabled {
		t.Fatal("expected enabled=true")
	}
	if got.HTTPPassword != in.HTTPPassword || got.HTTPUsername != in.HTTPUsername {
		t.Fatalf("HTTP creds did not round trip: %+v", got)
	}
	if got.RemoteURL != in.RemoteURL || got.Branch != in.Branch || got.Interval != in.Interval {
		t.Fatalf("config did not round trip: %+v", got)
	}
}

func TestConfigStore_Save_DoesNotWriteSecretsInPlaintext(t *testing.T) {
	dir := t.TempDir()
	s := NewConfigStore(dir, testBox(t))

	secret := "ghp_this_must_not_appear_on_disk"
	if err := s.Save(Config{
		Enabled:      true,
		RemoteURL:    "https://example.com/repo.git",
		HTTPUsername: "u",
		HTTPPassword: secret,
		Interval:     2 * time.Minute,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("secret leaked to disk in plaintext:\n%s", raw)
	}
}

func TestConfigStore_Save_FileIs0600(t *testing.T) {
	dir := t.TempDir()
	s := NewConfigStore(dir, testBox(t))
	if err := s.Save(Config{Enabled: false}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("git-backup.json perm = %o, want 600", perm)
	}
}

func TestConfigStore_Load_CorruptJSON_ReturnsErrConfigCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s := NewConfigStore(dir, testBox(t))
	if _, _, err := s.Load(); !errors.Is(err, ErrConfigCorrupt) {
		t.Fatalf("expected ErrConfigCorrupt, got %v", err)
	}
}

func TestConfigStore_Load_WrongKey_ReturnsErrConfigCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := NewConfigStore(dir, testBox(t)).Save(Config{
		Enabled:      true,
		RemoteURL:    "https://example.com/repo.git",
		HTTPUsername: "u",
		HTTPPassword: "secret",
		Interval:     2 * time.Minute,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	otherKey, _ := sharedcrypto.DeriveKey([]byte("a-different-jwt-secret"), "leafwiki:git-backup-credentials:v1")
	otherBox, _ := sharedcrypto.NewSecretBox(otherKey)

	if _, _, err := NewConfigStore(dir, otherBox).Load(); !errors.Is(err, ErrConfigCorrupt) {
		t.Fatalf("expected ErrConfigCorrupt when the key changed, got %v", err)
	}
}

func TestConfigStore_Save_NoBoxButSecretPresent_ReturnsErrNoEncryptionKey(t *testing.T) {
	s := NewConfigStore(t.TempDir(), nil)
	err := s.Save(Config{Enabled: true, RemoteURL: "https://x/y.git", HTTPUsername: "u", HTTPPassword: "s", Interval: time.Hour})
	if !errors.Is(err, ErrNoEncryptionKey) {
		t.Fatalf("expected ErrNoEncryptionKey, got %v", err)
	}
}

func TestConfigStore_Save_NoBoxNoSecret_Succeeds(t *testing.T) {
	dir := t.TempDir()
	s := NewConfigStore(dir, nil)
	if err := s.Save(Config{Enabled: false, RemoteURL: "https://x/y.git", Interval: time.Hour}); err != nil {
		t.Fatalf("Save without secrets and without a box should work: %v", err)
	}
	if _, _, err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
}
