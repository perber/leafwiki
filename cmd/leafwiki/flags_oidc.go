package main

import "github.com/urfave/cli/v3"

// oidcOptions holds the options for native OIDC login (authorization code +
// PKCE) against an external identity provider.
type oidcOptions struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURL  string
	buttonLabel  string
	only         bool
	adminGroup   string
	editorGroup  string
	roleSync     string
}

// Flags declares them. Adding an option here is the whole change: the flag,
// its default, its environment variable and its help text are one literal,
// and --help is generated from it.
func (o *oidcOptions) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "oidc-issuer",
			Destination: &o.issuer,
			Category:    catOIDC,
			Usage:       "OIDC provider discovery URL, e.g. https://auth.example.com/application/o/leafwiki/; unset disables OIDC login entirely",
			Sources:     envVars("LEAFWIKI_OIDC_ISSUER"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "oidc-client-id",
			Destination: &o.clientID,
			Category:    catOIDC,
			Usage:       "OIDC client ID (required when --oidc-issuer is set)",
			Sources:     envVars("LEAFWIKI_OIDC_CLIENT_ID"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "oidc-client-secret",
			Destination: &o.clientSecret,
			Category:    catOIDC,
			Usage:       "OIDC confidential client secret (env var preferred; required when --oidc-issuer is set)",
			Sources:     envVars("LEAFWIKI_OIDC_CLIENT_SECRET"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "oidc-redirect-url",
			Destination: &o.redirectURL,
			Category:    catOIDC,
			Usage:       "full callback URL registered with the provider, must end in /login/oidc/callback (required when --oidc-issuer is set)",
			Sources:     envVars("LEAFWIKI_OIDC_REDIRECT_URL"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "oidc-button-label",
			Destination: &o.buttonLabel,
			Category:    catOIDC,
			Usage:       "label for the login button, shown as \"Log in with <label>\"",
			Value:       "authentik",
			Sources:     envVars("LEAFWIKI_OIDC_BUTTON_LABEL"),
			Config:      trimmed,
		},
		&cli.BoolFlag{
			Name:        "oidc-only",
			Destination: &o.only,
			Category:    catOIDC,
			Usage:       "hide the local login form (reachable at /login?local=1 regardless)",
			Sources:     envBoolVars("LEAFWIKI_OIDC_ONLY"),
		},
		&cli.StringFlag{
			Name:        "oidc-admin-group",
			Destination: &o.adminGroup,
			Category:    catOIDC,
			Usage:       "OIDC \"groups\" claim value that maps to the admin role, re-evaluated on every login",
			Value:       "wiki-admins",
			Sources:     envVars("LEAFWIKI_OIDC_ADMIN_GROUP"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "oidc-editor-group",
			Destination: &o.editorGroup,
			Category:    catOIDC,
			Usage:       "OIDC \"groups\" claim value that maps to the editor role, re-evaluated on every login",
			Value:       "wiki-editors",
			Sources:     envVars("LEAFWIKI_OIDC_EDITOR_GROUP"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "oidc-role-sync",
			Destination: &o.roleSync,
			Category:    catOIDC,
			Usage:       "how much of a role OIDC may override on login: \"all\" (groups claim always wins) or \"admin\" (only promotes/demotes admin; editor/viewer are managed in LeafWiki)",
			Value:       "all",
			Sources:     envVars("LEAFWIKI_OIDC_ROLE_SYNC"),
			Config:      trimmed,
		},
	}
}
