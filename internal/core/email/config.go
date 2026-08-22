package email

import "time"

// Security selects how Client establishes transport security with the SMTP
// server. SecurityNone sends plaintext with no auth attempt and is only
// intended for local dev/test SMTP servers (e.g. Mailpit) — never production.
type Security string

const (
	SecurityNone     Security = "none"
	SecurityStartTLS Security = "starttls"
	SecurityTLS      Security = "tls"
)

const defaultTimeout = 10 * time.Second

// Config describes an optional SMTP integration. The zero value is a valid,
// disabled configuration — LeafWiki keeps working with zero email
// configuration for self-hosters who don't want it (see Enabled).
type Config struct {
	Host               string
	Port               int
	Username           string
	Password           string
	From               string
	FromName           string
	Security           Security
	InsecureSkipVerify bool
	Timeout            time.Duration
	PublicURL          string
}

// Enabled reports whether SMTP has been configured at all. Host is the single
// source of truth for this, matching how the CLI/ENV layer decides
// SMTPEnabled (see cmd/leafwiki/main.go's validateSMTPConfig).
func (c Config) Enabled() bool {
	return c.Host != ""
}

// TimeoutOrDefault returns Timeout, or a 10s default if unset — this is the
// first synchronous third-party network I/O in the codebase, so every send
// gets a bounded deadline even if the operator didn't configure one.
func (c Config) TimeoutOrDefault() time.Duration {
	if c.Timeout <= 0 {
		return defaultTimeout
	}
	return c.Timeout
}
