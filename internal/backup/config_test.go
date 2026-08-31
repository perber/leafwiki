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
	cfg.HTTPUsername, cfg.HTTPPassword = "u", "p"          // ...but only HTTP creds
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