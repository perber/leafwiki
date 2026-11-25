// Package oidc provides optional OpenID Connect login (authorization code +
// PKCE) against an external identity provider (e.g. Authentik), as an
// additional login method alongside LeafWiki's own local password+TOTP
// login. Modeled on internal/core/email: a plain Config whose zero value is
// a valid, disabled configuration, and a Service built from it once an
// operator actually configures an issuer.
package oidc

// Config describes an optional OIDC integration. The zero value is a valid,
// disabled configuration — LeafWiki keeps working with zero OIDC
// configuration for self-hosters who don't want it (see Enabled).
type Config struct {
	// Issuer is the provider's discovery URL, e.g.
	// "https://auth.example.com/application/o/leafwiki/". Passed through to
	// the OIDC discovery client completely unmodified — in particular, never
	// trim or add a trailing slash here: go-oidc's discovery already handles
	// a trailing slash correctly, and the "iss" value returned by the
	// provider's discovery document must match this string exactly (the
	// library enforces this), so altering it risks breaking that check for
	// providers that round-trip the slash.
	Issuer       string
	ClientID     string
	ClientSecret string
	// RedirectURL must exactly match what's registered with the provider —
	// see internal/wiki/oidc for the callback route this must point at.
	RedirectURL string
	// ButtonLabel is shown on the login button ("Log in with <ButtonLabel>").
	ButtonLabel string
	// OIDCOnly hides the local username/password form on the login page,
	// except via the documented ?local=1 escape hatch.
	OIDCOnly bool
	// AdminGroup/EditorGroup are the names (as they appear in the OIDC
	// "groups" claim) that map to RoleAdmin/RoleEditor on every login; a user
	// in neither falls back to the wiki's lowest role. See MapRole.
	AdminGroup  string
	EditorGroup string
	// RoleSync controls how much of a synced user's role OIDC is allowed to
	// override on each login. Zero value behaves as RoleSyncAll. See
	// ResolveSyncedRole.
	RoleSync RoleSync
}

// Enabled reports whether OIDC has been configured at all. Issuer is the
// single source of truth for this, matching how Config.Enabled works for
// SMTP (internal/core/email) and how the CLI/ENV layer decides OIDCEnabled
// (see cmd/leafwiki/main.go's validateOIDCConfig).
func (c Config) Enabled() bool {
	return c.Issuer != ""
}
