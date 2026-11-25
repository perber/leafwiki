package oidc

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Claims are the subset of an OIDC ID token's claims this app cares about.
type Claims struct {
	Sub               string   `json:"sub"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Email             string   `json:"email"`
	Groups            []string `json:"groups"`
}

// Service wraps a discovered OIDC provider plus the oauth2 client built from
// it. Constructed once at boot (see wiki.initOIDC) — discovery is a blocking
// network call, matching how NewTOTPService/NewUserStore do their one-time
// setup work synchronously rather than lazily.
type Service struct {
	cfg      Config
	provider *oidc.Provider
	oauth2   oauth2.Config
	verifier *oidc.IDTokenVerifier

	// EndSessionURL is the provider's end_session_endpoint, if advertised in
	// its discovery document. Not a standard field on oidc.Provider's typed
	// struct, so it's pulled from the raw discovery JSON once here instead.
	// Empty if the provider doesn't advertise one — callers should treat
	// that as "no IdP-side logout available."
	EndSessionURL string
}

// NewService performs OIDC discovery against cfg.Issuer and returns a ready
// Service. Discovery failure is returned as an error (not swallowed into a
// disabled-but-silent state): an operator who configured a real issuer wants
// a loud failure at boot if it's unreachable or misconfigured, not a broken
// login button discovered only when a visitor tries to use it.
func NewService(ctx context.Context, cfg Config) (*Service, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery against %q failed: %w", cfg.Issuer, err)
	}

	var discovery struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&discovery); err != nil {
		return nil, fmt.Errorf("oidc: parsing discovery document from %q failed: %w", cfg.Issuer, err)
	}

	return &Service{
		cfg:      cfg,
		provider: provider,
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail},
		},
		verifier:      provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		EndSessionURL: discovery.EndSessionEndpoint,
	}, nil
}

// AuthCodeURL builds the provider's authorization URL for a fresh login
// attempt: state for CSRF protection on this top-level-navigation flow,
// nonce for ID-token replay protection, and a PKCE S256 challenge derived
// from codeVerifier (the raw verifier itself never leaves the server — see
// internal/wiki/oidc's state cookie).
func (s *Service) AuthCodeURL(state, nonce, codeVerifier string) string {
	return s.oauth2.AuthCodeURL(state,
		oauth2.S256ChallengeOption(codeVerifier),
		oidc.Nonce(nonce),
	)
}

// Exchange trades an authorization code for tokens, verifies the returned ID
// token's signature/issuer/audience/expiry and nonce, and returns its claims
// plus the raw (still-encoded) ID token string itself — the latter is opaque
// to this app beyond verification, kept only so a caller can store it and
// later send it back to the provider as the end_session_endpoint's
// id_token_hint on logout. codeVerifier must be the exact PKCE verifier that
// produced the challenge passed to AuthCodeURL for this same flow.
func (s *Service) Exchange(ctx context.Context, code, codeVerifier, expectedNonce string) (*Claims, string, error) {
	token, err := s.oauth2.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrProviderExchangeFailed, err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, "", ErrMissingIDToken
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrIDTokenInvalid, err)
	}
	if idToken.Nonce != expectedNonce {
		return nil, "", fmt.Errorf("%w: nonce mismatch", ErrIDTokenInvalid)
	}

	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, "", fmt.Errorf("%w: parsing claims: %w", ErrIDTokenInvalid, err)
	}
	return &claims, rawIDToken, nil
}

// MapRole maps claims.Groups to a LeafWiki role using this service's
// configured admin/editor group names. Used for a brand-new account's
// initial role, and for an existing one when RoleSync is RoleSyncAll.
func (s *Service) MapRole(claims *Claims) string {
	return MapRole(claims.Groups, s.cfg.AdminGroup, s.cfg.EditorGroup)
}

// ResolveSyncedRole decides the role to assign an existing user on this
// login, honoring this service's configured RoleSync mode. See
// ResolveSyncedRole (the package-level function) for the semantics.
func (s *Service) ResolveSyncedRole(claims *Claims, currentRole string) string {
	return ResolveSyncedRole(s.cfg.RoleSync, claims.Groups, s.cfg.AdminGroup, s.cfg.EditorGroup, currentRole)
}

// ButtonLabel returns the configured login-button label.
func (s *Service) ButtonLabel() string { return s.cfg.ButtonLabel }

// OIDCOnly reports whether the local login form should be hidden.
func (s *Service) OIDCOnly() bool { return s.cfg.OIDCOnly }
