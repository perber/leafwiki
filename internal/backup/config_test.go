package backup

import (
	"testing"
	"time"
)

func validSettingsConfig() Config {
	return Config{
		RemoteURL:    "https://github.com/acme/wiki-backup.git",
		Branch:       "main",
		AuthorName:   "Backup Bot",
		AuthorEmail:  "bot@example.com",
		HTTPUsername: "acme-bot",
		HTTPPassword: "ghp_token",
		Interval:     30 * time.Minute,
	}
}

func TestConfig_ValidateForSettings_IntervalBounds(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		wantErr  bool
	}{
		{"below minimum", MinSettingsInterval - time.Second, true},
		{"exactly minimum", MinSettingsInterval, false},
		{"mid range", time.Hour, false},
		{"exactly maximum", MaxSettingsInterval, false},
		{"above maximum", MaxSettingsInterval + time.Minute, true},
		{"zero (manual-only not allowed via settings)", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validSettingsConfig()
			cfg.Interval = tc.interval
			err := cfg.ValidateForSettings()
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateForSettings() err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestConfig_ValidateForSettings_RequiresRemote(t *testing.T) {
	cfg := validSettingsConfig()
	cfg.RemoteURL = ""
	if err := cfg.ValidateForSettings(); err == nil {
		t.Fatal("expected an error when RemoteURL is empty")
	}
}

func TestConfig_ValidateForSettings_RejectsCredentialTransportMismatch(t *testing.T) {
	cfg := validSettingsConfig()
	cfg.RemoteURL = "git@github.com:acme/wiki-backup.git" // SSH remote...
	cfg.HTTPUsername, cfg.HTTPPassword = "u", "p"         // ...but only HTTP creds
	cfg.SSHKey, cfg.SSHKeyPath = "", ""
	if err := cfg.ValidateForSettings(); err == nil {
		t.Fatal("expected an error for an SSH remote with no SSH key")
	}
}

func TestConfig_WithSettingsDefaults_FillsOptionalFields(t *testing.T) {
	got := Config{RemoteURL: "https://x/y.git"}.WithSettingsDefaults()
	if got.Branch != DefaultBranch || got.AuthorName != DefaultAuthorName || got.AuthorEmail != DefaultAuthorEmail {
		t.Fatalf("defaults not applied: %+v", got)
	}
}

func TestConfig_WithKeptSecrets(t *testing.T) {
	prev := Config{SSHKey: "OLD-KEY", HTTPPassword: "OLD-PASS"}

	// Blank incoming secrets are backfilled from prev.
	got := Config{}.WithKeptSecrets(prev)
	if got.SSHKey != "OLD-KEY" || got.HTTPPassword != "OLD-PASS" {
		t.Fatalf("blank secrets not kept: %+v", got)
	}

	// Provided incoming secrets win.
	got = Config{SSHKey: "NEW-KEY", HTTPPassword: "NEW-PASS"}.WithKeptSecrets(prev)
	if got.SSHKey != "NEW-KEY" || got.HTTPPassword != "NEW-PASS" {
		t.Fatalf("provided secrets should win: %+v", got)
	}

	// Mixed: keep one, replace the other.
	got = Config{SSHKey: "NEW-KEY"}.WithKeptSecrets(prev)
	if got.SSHKey != "NEW-KEY" || got.HTTPPassword != "OLD-PASS" {
		t.Fatalf("mixed keep/replace wrong: %+v", got)
	}
}

func TestConfig_WithoutForeignTransportCreds(t *testing.T) {
	full := Config{
		SSHKey: "k", SSHKeyPath: "/k", SSHKnownHostsPath: "/kh",
		HTTPUsername: "u", HTTPPassword: "p",
	}

	https := full
	https.RemoteURL = "https://example.com/r.git"
	https = https.WithoutForeignTransportCreds()
	if https.SSHKey != "" || https.SSHKeyPath != "" || https.SSHKnownHostsPath != "" {
		t.Fatalf("HTTP(S) remote kept SSH creds: %+v", https)
	}
	if https.HTTPUsername != "u" || https.HTTPPassword != "p" {
		t.Fatalf("HTTP(S) remote dropped its own creds: %+v", https)
	}

	ssh := full
	ssh.RemoteURL = "git@example.com:r.git"
	ssh = ssh.WithoutForeignTransportCreds()
	if ssh.HTTPUsername != "" || ssh.HTTPPassword != "" {
		t.Fatalf("SSH remote kept HTTP creds: %+v", ssh)
	}
	if ssh.SSHKey != "k" || ssh.SSHKeyPath != "/k" || ssh.SSHKnownHostsPath != "/kh" {
		t.Fatalf("SSH remote dropped its own creds: %+v", ssh)
	}

	// Unknown / local-only remote: nothing is touched.
	local := full
	local.RemoteURL = ""
	if local.WithoutForeignTransportCreds() != full {
		t.Fatalf("local-only remote should keep everything")
	}
}
