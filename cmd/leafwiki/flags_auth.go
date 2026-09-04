package main

import (
	"time"

	"github.com/urfave/cli/v3"
)

// authOptions holds the options for authentication, tokens and the initial admin.
type authOptions struct {
	jwtSecret           string
	totpEncryptionKey   string
	disableAuth         bool
	publicAccess        bool
	accessTokenTimeout  time.Duration
	refreshTokenTimeout time.Duration
	adminUsername       string
	adminEmail          string
	adminPassword       string
	editorLimit         int
}

// Flags declares them. Adding an option here is the whole change: the flag,
// its default, its environment variable and its help text are one literal,
// and --help is generated from it.
func (o *authOptions) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "jwt-secret",
			Destination: &o.jwtSecret,
			Category:    catAuth,
			Usage:       "secret for signing auth tokens (JWT); required unless --disable-auth is set",
			Sources:     envVars("LEAFWIKI_JWT_SECRET"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "totp-encryption-key",
			Destination: &o.totpEncryptionKey,
			Category:    catAuth,
			Usage:       "key to encrypt per-user TOTP secrets at rest, min 32 bytes; unset keeps TOTP unavailable",
			Sources:     envVars("LEAFWIKI_TOTP_ENCRYPTION_KEY"),
			Validator:   validateTOTPEncryptionKey,
			Config:      trimmed,
		},
		&cli.BoolFlag{
			Name:        "disable-auth",
			Destination: &o.disableAuth,
			Category:    catAuth,
			Usage:       "disable authentication completely (WARNING: only use in trusted networks!)",
			Sources:     envBoolVars("LEAFWIKI_DISABLE_AUTH"),
		},
		&cli.BoolFlag{
			Name:        "public-access",
			Destination: &o.publicAccess,
			Category:    catAuth,
			Usage:       "allow public access to the wiki with read access",
			Sources:     envBoolVars("LEAFWIKI_PUBLIC_ACCESS"),
		},
		&cli.DurationFlag{
			Name:        "access-token-timeout",
			Destination: &o.accessTokenTimeout,
			Category:    catAuth,
			Usage:       "access token timeout duration (e.g. 24h, 15m)",
			Value:       15 * time.Minute,
			Sources:     envVars("LEAFWIKI_ACCESS_TOKEN_TIMEOUT"),
			DefaultText: "15m",
		},
		&cli.DurationFlag{
			Name:        "refresh-token-timeout",
			Destination: &o.refreshTokenTimeout,
			Category:    catAuth,
			Usage:       "refresh token timeout duration (e.g. 168h)",
			Value:       7 * 24 * time.Hour,
			Sources:     envVars("LEAFWIKI_REFRESH_TOKEN_TIMEOUT"),
			DefaultText: "168h",
		},
		&cli.StringFlag{
			Name:        "admin-username",
			Destination: &o.adminUsername,
			Category:    catAdmin,
			Usage:       "initial admin username, used only if no admin exists (default: admin)",
			Sources:     envVars("LEAFWIKI_ADMIN_USERNAME"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "admin-email",
			Destination: &o.adminEmail,
			Category:    catAdmin,
			Usage:       "initial admin email, used only if no admin exists (default: admin@localhost)",
			Sources:     envVars("LEAFWIKI_ADMIN_EMAIL"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "admin-password",
			Destination: &o.adminPassword,
			Category:    catAdmin,
			Usage:       "initial admin password, used only if no admin exists; min 8 characters",
			Sources:     envVars("LEAFWIKI_ADMIN_PASSWORD"),
			Config:      trimmed,
		},
		&cli.IntFlag{
			Name:        "editor-limit",
			Destination: &o.editorLimit,
			Category:    catAuth,
			Usage:       "maximum admin+editor users allowed; 0 = unlimited",
			Sources:     envVars("LEAFWIKI_EDITOR_LIMIT"),
		},
	}
}
