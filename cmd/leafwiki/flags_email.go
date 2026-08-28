package main

import (
	"time"

	"github.com/urfave/cli/v3"
)

// emailOptions holds the options for outgoing email (SMTP).
type emailOptions struct {
	smtpHost               string
	smtpPort               int
	smtpUsername           string
	smtpPassword           string
	smtpFrom               string
	smtpFromName           string
	smtpSecurity           string
	smtpInsecureSkipVerify bool
	smtpTimeout            time.Duration
	publicURL              string
}

// Flags declares them. Adding an option here is the whole change: the flag,
// its default, its environment variable and its help text are one literal,
// and --help is generated from it.
func (o *emailOptions) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "smtp-host",
			Destination: &o.smtpHost,
			Category:    catEmail,
			Usage:       "SMTP host for password-reset/invite email; unset disables email entirely",
			Sources:     envVars("LEAFWIKI_SMTP_HOST"),
			Config:      trimmed,
		},
		&cli.IntFlag{
			Name:        "smtp-port",
			Destination: &o.smtpPort,
			Category:    catEmail,
			Usage:       "SMTP server port",
			Value:       587,
			Sources:     envVars("LEAFWIKI_SMTP_PORT"),
		},
		&cli.StringFlag{
			Name:        "smtp-username",
			Destination: &o.smtpUsername,
			Category:    catEmail,
			Usage:       "SMTP auth username",
			Sources:     envVars("LEAFWIKI_SMTP_USERNAME"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "smtp-password",
			Destination: &o.smtpPassword,
			Category:    catEmail,
			Usage:       "SMTP auth password (env var preferred)",
			Sources:     envVars("LEAFWIKI_SMTP_PASSWORD"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "smtp-from",
			Destination: &o.smtpFrom,
			Category:    catEmail,
			Usage:       "From address for outgoing email (required when --smtp-host is set)",
			Sources:     envVars("LEAFWIKI_SMTP_FROM"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "smtp-from-name",
			Destination: &o.smtpFromName,
			Category:    catEmail,
			Usage:       "From display name for outgoing email",
			Value:       "LeafWiki",
			Sources:     envVars("LEAFWIKI_SMTP_FROM_NAME"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "smtp-security",
			Destination: &o.smtpSecurity,
			Category:    catEmail,
			Usage:       "SMTP transport security: none, starttls, or tls",
			Value:       "starttls",
			Sources:     envVars("LEAFWIKI_SMTP_SECURITY"),
			Config:      trimmed,
		},
		&cli.BoolFlag{
			Name:        "smtp-insecure-skip-verify",
			Destination: &o.smtpInsecureSkipVerify,
			Category:    catEmail,
			Usage:       "skip TLS certificate verification for SMTP (do not use in production)",
			Sources:     envBoolVars("LEAFWIKI_SMTP_INSECURE_SKIP_VERIFY"),
		},
		&cli.DurationFlag{
			Name:        "smtp-timeout",
			Destination: &o.smtpTimeout,
			Category:    catEmail,
			Usage:       "timeout for a single SMTP send (e.g. 10s)",
			Value:       10 * time.Second,
			Sources:     envVars("LEAFWIKI_SMTP_TIMEOUT"),
			DefaultText: "10s",
		},
		&cli.StringFlag{
			Name:        "public-url",
			Destination: &o.publicURL,
			Category:    catEmail,
			Usage:       "base URL for links in outgoing email, e.g. https://wiki.example.com (required with --smtp-host)",
			Sources:     envVars("LEAFWIKI_PUBLIC_URL"),
			Config:      trimmed,
		},
	}
}
