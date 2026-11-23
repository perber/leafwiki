// Package oidc is the HTTP surface for native OIDC login (authorization
// code + PKCE) against an external identity provider, as an additional login
// method alongside internal/wiki/auth's local password+TOTP login. Kept as
// its own package/registrar — like wikisnapshot/wikibackup — so the whole
// subsystem is trivially absent (no routes registered, no dependencies
// pulled in) when OIDC isn't configured.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"

	coreauth "github.com/perber/wiki/internal/core/auth"
	coreoidc "github.com/perber/wiki/internal/core/oidc"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"golang.org/x/oauth2"
)

// ErrOIDCDisabled is returned when an OIDC operation is called while no
// provider is configured.
var ErrOIDCDisabled = errors.New("oidc login is not configured")

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ─── StartOIDCLoginUseCase ────────────────────────────────────────────────────

type StartOIDCLoginOutput struct {
	AuthURL  string
	State    string
	Nonce    string
	Verifier string
}

type StartOIDCLoginUseCase struct {
	oidc func() *coreoidc.Service
}

func NewStartOIDCLoginUseCase(o func() *coreoidc.Service) *StartOIDCLoginUseCase {
	return &StartOIDCLoginUseCase{oidc: o}
}

func (uc *StartOIDCLoginUseCase) Execute(_ context.Context) (*StartOIDCLoginOutput, error) {
	svc := uc.oidc()
	if svc == nil {
		return nil, ErrOIDCDisabled
	}

	state, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	nonce, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	return &StartOIDCLoginOutput{
		AuthURL:  svc.AuthCodeURL(state, nonce, verifier),
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
	}, nil
}

// ─── HandleOIDCCallbackUseCase ────────────────────────────────────────────────

type HandleOIDCCallbackInput struct {
	Code  string
	State string
	Nonce string

	// ExpectedState/Verifier/ExpectedNonce come from the state cookie set by
	// StartOIDCLoginUseCase's output, not from the callback's query string —
	// the whole point of checking them is to confirm they match what the
	// query string claims.
	ExpectedState string
	Verifier      string
	ExpectedNonce string
}

type HandleOIDCCallbackOutput struct {
	Token *coreauth.AuthToken
}

type HandleOIDCCallbackUseCase struct {
	oidc    func() *coreoidc.Service
	auth    *coreauth.AuthService
	user    func() *coreauth.UserService
	metrics *httpmetrics.HTTPMetrics
}

func NewHandleOIDCCallbackUseCase(o func() *coreoidc.Service, a *coreauth.AuthService, u func() *coreauth.UserService, metrics *httpmetrics.HTTPMetrics) *HandleOIDCCallbackUseCase {
	return &HandleOIDCCallbackUseCase{oidc: o, auth: a, user: u, metrics: metrics}
}

func (uc *HandleOIDCCallbackUseCase) Execute(ctx context.Context, in HandleOIDCCallbackInput) (*HandleOIDCCallbackOutput, error) {
	svc := uc.oidc()
	if svc == nil || uc.auth == nil {
		return nil, ErrOIDCDisabled
	}

	// Constant-time compare: this is the actual CSRF defense for this leg of
	// the flow (a top-level browser navigation the app's own CSRF-cookie
	// mechanism can't reach), so it deserves the same care as a secret
	// comparison even though "state" itself isn't confidential.
	if in.ExpectedState == "" || subtle.ConstantTimeCompare([]byte(in.State), []byte(in.ExpectedState)) != 1 {
		uc.metrics.IncAuthLoginAttempt("oidc_state_mismatch")
		return nil, coreoidc.ErrStateMismatch
	}

	claims, err := svc.Exchange(ctx, in.Code, in.Verifier, in.ExpectedNonce)
	if err != nil {
		uc.metrics.IncAuthLoginAttempt("oidc_error")
		return nil, err
	}

	// MapRole's full result is always the right call for a brand-new
	// account's initial role, even under RoleSyncAdmin: that mode only
	// restricts what happens to a role an existing account already has (see
	// ResolveSyncedRole), and a just-created account has no prior role to
	// protect yet.
	initialRole := svc.MapRole(claims)
	user, err := uc.user().GetOrCreateOIDCUser(claims.Sub, claims.PreferredUsername, claims.Email, initialRole)
	if err != nil {
		uc.metrics.IncAuthLoginAttempt("oidc_error")
		return nil, fmt.Errorf("oidc: resolving user: %w", err)
	}
	uc.user().SyncOIDCRole(user.ID, svc.ResolveSyncedRole(claims, user.Role))

	token, err := uc.auth.IssueSessionForUser(user.ID)
	if err != nil {
		uc.metrics.IncAuthLoginAttempt("oidc_error")
		return nil, err
	}

	uc.metrics.IncAuthLoginAttempt("oidc_success")
	uc.metrics.IncAuthSession("issued")
	return &HandleOIDCCallbackOutput{Token: token}, nil
}
