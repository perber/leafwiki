package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/perber/wiki/internal/core/shared"
	sharedcrypto "github.com/perber/wiki/internal/core/shared/crypto"
)

// ConfigFileName is the on-disk name of the settings-managed git backup
// configuration, stored in the LeafWiki data directory alongside branding.json.
const ConfigFileName = "git-backup.json"

// CredentialsKeyInfo is the HKDF info label that derives the git-backup
// credential-encryption key from the JWT secret. Kept here so the composition
// root and tests agree on it.
const CredentialsKeyInfo = "leafwiki:git-backup-credentials:v1"

// ErrNoEncryptionKey is returned when stored ciphertext must be decrypted but no
// key is available. This only happens when a config was saved while
// authentication was enabled and the server was later restarted with
// --disable-auth: the JWT secret that keyed the SecretBox is gone. A config
// saved under --disable-auth in the first place carries its secrets in
// plaintext (see Save) and never hits this path.
var ErrNoEncryptionKey = errors.New("no encryption key available to decrypt git backup credentials")

// ErrConfigCorrupt is returned when git-backup.json exists but cannot be parsed
// or its credentials cannot be decrypted. The manager surfaces this and stays
// idle rather than silently dropping a configured backup.
var ErrConfigCorrupt = errors.New("git backup configuration file is unreadable")

// persistedConfig is the JSON shape of git-backup.json. It mirrors the
// settings-relevant subset of Config: RootDir/AssetsDir are runtime paths
// supplied by the composition root and are never persisted.
//
// Each secret has two mutually exclusive fields: the "*Enc" one holds SecretBox
// ciphertext (written when an encryption key is available, i.e. auth is on), the
// plaintext one is written only under --disable-auth, where there is no JWT
// secret to derive a key from. Load prefers the encrypted field and falls back
// to the plaintext one.
type persistedConfig struct {
	Enabled           bool   `json:"enabled"`
	AuthorName        string `json:"authorName,omitempty"`
	AuthorEmail       string `json:"authorEmail,omitempty"`
	RemoteURL         string `json:"remoteUrl,omitempty"`
	Branch            string `json:"branch,omitempty"`
	SSHKeyPath        string `json:"sshKeyPath,omitempty"`
	SSHKeyEnc         string `json:"sshKeyEnc,omitempty"`
	SSHKeyPlain       string `json:"sshKey,omitempty"`
	SSHKnownHostsPath string `json:"sshKnownHostsPath,omitempty"`
	HTTPUsername      string `json:"httpUsername,omitempty"`
	HTTPPasswordEnc   string `json:"httpPasswordEnc,omitempty"`
	HTTPPasswordPlain string `json:"httpPassword,omitempty"`
	IntervalSeconds   int64  `json:"intervalSeconds,omitempty"`
}

// ConfigStore reads and writes git-backup.json, transparently
// encrypting/decrypting the SSH key and HTTP password fields with box.
type ConfigStore struct {
	path string
	box  *sharedcrypto.SecretBox // may be nil when no encryption key is configured
}

// NewConfigStore builds a store for <dataDir>/git-backup.json. box may be nil
// (under --disable-auth there is no JWT secret to derive a key from); in that
// case secrets are stored in plaintext in the 0600 file instead of encrypted.
func NewConfigStore(dataDir string, box *sharedcrypto.SecretBox) *ConfigStore {
	return &ConfigStore{path: filepath.Join(dataDir, ConfigFileName), box: box}
}

// Path returns the absolute path of the backing file (useful for logging).
func (s *ConfigStore) Path() string { return s.path }

// CanEncrypt reports whether the store has an encryption key, i.e. whether
// saved credentials are encrypted at rest rather than stored in plaintext.
func (s *ConfigStore) CanEncrypt() bool { return s.box != nil }

// Exists reports whether git-backup.json is present on disk.
func (s *ConfigStore) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Load reads git-backup.json. The bool reports whether a config exists AND has
// Enabled == true. A missing file is not an error: (Config{}, false, nil). A
// present-but-unparseable or undecryptable file returns ErrConfigCorrupt.
func (s *ConfigStore) Load() (Config, bool, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("%w: %v", ErrConfigCorrupt, err)
	}

	var pc persistedConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return Config{}, false, fmt.Errorf("%w: %v", ErrConfigCorrupt, err)
	}

	cfg := Config{
		Enabled:           pc.Enabled,
		AuthorName:        pc.AuthorName,
		AuthorEmail:       pc.AuthorEmail,
		RemoteURL:         pc.RemoteURL,
		Branch:            pc.Branch,
		SSHKeyPath:        pc.SSHKeyPath,
		SSHKnownHostsPath: pc.SSHKnownHostsPath,
		HTTPUsername:      pc.HTTPUsername,
		Interval:          time.Duration(pc.IntervalSeconds) * time.Second,
	}

	switch {
	case pc.SSHKeyEnc != "":
		plain, err := s.decrypt(pc.SSHKeyEnc)
		if err != nil {
			return Config{}, false, fmt.Errorf("%w: SSH key: %v", ErrConfigCorrupt, err)
		}
		cfg.SSHKey = plain
	case pc.SSHKeyPlain != "":
		cfg.SSHKey = pc.SSHKeyPlain
	}
	switch {
	case pc.HTTPPasswordEnc != "":
		plain, err := s.decrypt(pc.HTTPPasswordEnc)
		if err != nil {
			return Config{}, false, fmt.Errorf("%w: HTTP password: %v", ErrConfigCorrupt, err)
		}
		cfg.HTTPPassword = plain
	case pc.HTTPPasswordPlain != "":
		cfg.HTTPPassword = pc.HTTPPasswordPlain
	}

	return cfg, cfg.Enabled, nil
}

// Save writes cfg to git-backup.json atomically with 0600 permissions. The SSH
// key and HTTP password are encrypted when the store has a key, and written in
// plaintext otherwise (under --disable-auth there is no key to derive from, and
// with no accounts on the instance at-rest encryption keyed from a co-located
// secret would add nothing over the 0600 file). RootDir/AssetsDir are ignored.
func (s *ConfigStore) Save(cfg Config) error {
	pc := persistedConfig{
		Enabled:           cfg.Enabled,
		AuthorName:        cfg.AuthorName,
		AuthorEmail:       cfg.AuthorEmail,
		RemoteURL:         cfg.RemoteURL,
		Branch:            cfg.Branch,
		SSHKeyPath:        cfg.SSHKeyPath,
		SSHKnownHostsPath: cfg.SSHKnownHostsPath,
		HTTPUsername:      cfg.HTTPUsername,
		IntervalSeconds:   int64(cfg.Interval / time.Second),
	}

	if cfg.SSHKey != "" {
		if s.box != nil {
			enc, err := s.box.Seal(cfg.SSHKey)
			if err != nil {
				return fmt.Errorf("encrypt SSH key: %w", err)
			}
			pc.SSHKeyEnc = enc
		} else {
			pc.SSHKeyPlain = cfg.SSHKey
		}
	}
	if cfg.HTTPPassword != "" {
		if s.box != nil {
			enc, err := s.box.Seal(cfg.HTTPPassword)
			if err != nil {
				return fmt.Errorf("encrypt HTTP password: %w", err)
			}
			pc.HTTPPasswordEnc = enc
		} else {
			pc.HTTPPasswordPlain = cfg.HTTPPassword
		}
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal git backup config: %w", err)
	}
	if err := shared.WriteFileAtomic(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", ConfigFileName, err)
	}
	return nil
}

func (s *ConfigStore) decrypt(enc string) (string, error) {
	if s.box == nil {
		return "", ErrNoEncryptionKey
	}
	return s.box.Open(enc)
}
