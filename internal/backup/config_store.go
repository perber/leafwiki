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

// ErrNoEncryptionKey is returned when a config carrying secrets must be
// encrypted or decrypted but no key is available (e.g. --disable-auth, so
// there is no JWT secret to derive one from).
var ErrNoEncryptionKey = errors.New("no encryption key available for git backup credentials")

// ErrConfigCorrupt is returned when git-backup.json exists but cannot be parsed
// or its credentials cannot be decrypted. The manager surfaces this and stays
// idle rather than silently dropping a configured backup.
var ErrConfigCorrupt = errors.New("git backup configuration file is unreadable")

// persistedConfig is the JSON shape of git-backup.json. It mirrors the
// settings-relevant subset of Config: RootDir/AssetsDir are runtime paths
// supplied by the composition root and are never persisted, and the two secret
// fields hold SecretBox ciphertext, never plaintext.
type persistedConfig struct {
	Enabled           bool   `json:"enabled"`
	AuthorName        string `json:"authorName,omitempty"`
	AuthorEmail       string `json:"authorEmail,omitempty"`
	RemoteURL         string `json:"remoteUrl,omitempty"`
	Branch            string `json:"branch,omitempty"`
	SSHKeyPath        string `json:"sshKeyPath,omitempty"`
	SSHKeyEnc         string `json:"sshKeyEnc,omitempty"`
	SSHKnownHostsPath string `json:"sshKnownHostsPath,omitempty"`
	HTTPUsername      string `json:"httpUsername,omitempty"`
	HTTPPasswordEnc   string `json:"httpPasswordEnc,omitempty"`
	IntervalSeconds   int64  `json:"intervalSeconds,omitempty"`
}

// ConfigStore reads and writes git-backup.json, transparently
// encrypting/decrypting the SSH key and HTTP password fields with box.
type ConfigStore struct {
	path string
	box  *sharedcrypto.SecretBox // may be nil when no encryption key is configured
}

// NewConfigStore builds a store for <dataDir>/git-backup.json. box may be nil;
// in that case Load/Save succeed only for configs that carry no secrets.
func NewConfigStore(dataDir string, box *sharedcrypto.SecretBox) *ConfigStore {
	return &ConfigStore{path: filepath.Join(dataDir, ConfigFileName), box: box}
}

// Path returns the absolute path of the backing file (useful for logging).
func (s *ConfigStore) Path() string { return s.path }

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

	if pc.SSHKeyEnc != "" {
		plain, err := s.decrypt(pc.SSHKeyEnc)
		if err != nil {
			return Config{}, false, fmt.Errorf("%w: SSH key: %v", ErrConfigCorrupt, err)
		}
		cfg.SSHKey = plain
	}
	if pc.HTTPPasswordEnc != "" {
		plain, err := s.decrypt(pc.HTTPPasswordEnc)
		if err != nil {
			return Config{}, false, fmt.Errorf("%w: HTTP password: %v", ErrConfigCorrupt, err)
		}
		cfg.HTTPPassword = plain
	}

	return cfg, cfg.Enabled, nil
}

// Save writes cfg to git-backup.json atomically with 0600 permissions,
// encrypting the SSH key and HTTP password. RootDir/AssetsDir are ignored.
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
		enc, err := s.encrypt(cfg.SSHKey)
		if err != nil {
			return fmt.Errorf("encrypt SSH key: %w", err)
		}
		pc.SSHKeyEnc = enc
	}
	if cfg.HTTPPassword != "" {
		enc, err := s.encrypt(cfg.HTTPPassword)
		if err != nil {
			return fmt.Errorf("encrypt HTTP password: %w", err)
		}
		pc.HTTPPasswordEnc = enc
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

func (s *ConfigStore) encrypt(plain string) (string, error) {
	if s.box == nil {
		return "", ErrNoEncryptionKey
	}
	return s.box.Seal(plain)
}

func (s *ConfigStore) decrypt(enc string) (string, error) {
	if s.box == nil {
		return "", ErrNoEncryptionKey
	}
	return s.box.Open(enc)
}