package oidc

import "errors"

var (
	// ErrStateMismatch means the callback's state parameter didn't match the
	// one issued at the start of the flow (or the state cookie was missing/
	// expired/tampered with) — this is the actual CSRF defense for the OIDC
	// login flow, since it's a top-level browser navigation the app's usual
	// CSRF-cookie mechanism can't reach.
	ErrStateMismatch = errors.New("oidc: state mismatch")
	// ErrProviderExchangeFailed wraps a failure exchanging the authorization
	// code for tokens with the provider.
	ErrProviderExchangeFailed = errors.New("oidc: token exchange failed")
	// ErrIDTokenInvalid means the ID token failed signature/issuer/audience
	// verification, or carried a nonce that didn't match the one issued.
	ErrIDTokenInvalid = errors.New("oidc: id token invalid")
	// ErrMissingIDToken means the token response had no id_token at all.
	ErrMissingIDToken = errors.New("oidc: token response missing id_token")
)
