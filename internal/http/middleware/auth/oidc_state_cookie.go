package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware/utils"
)

// OIDCStateCookie transports an OIDC login attempt's state/nonce/PKCE
// verifier between the login-initiating redirect and the provider's
// callback. No user session exists yet at that point, so these ephemeral,
// single-use values travel in a short-lived cookie rather than server-side
// storage — the same idiom as CSRFCookie, not a signed token: an httpOnly,
// Secure, SameSite=Lax, narrowly-Path-scoped cookie is already unreadable
// and unforgeable by anything except the browser that received it, which is
// exactly what state/PKCE need (state defends against CSRF on this
// top-level-navigation flow; the verifier defends against a stolen
// authorization code being exchanged by anyone but the browser that started
// the flow) — signing the value would add complexity without closing any
// gap that the cookie's own attributes don't already close.
type OIDCStateCookie struct {
	AllowInsecure bool
	SameSite      http.SameSite
	TTL           time.Duration
}

func NewOIDCStateCookie(allowInsecure bool, ttl time.Duration) *OIDCStateCookie {
	return &OIDCStateCookie{
		AllowInsecure: allowInsecure,
		SameSite:      http.SameSiteLaxMode, // not Strict: the callback is a top-level cross-site GET from the provider
		TTL:           ttl,
	}
}

// cookieName uses the "__Secure-" prefix, not "__Host-": a __Host- cookie is
// REQUIRED by browsers to have Path=/ (and no Domain), and silently rejected
// otherwise — Go's net/http and httptest/cookiejar don't enforce this, so the
// bug only showed up against a real browser. __Secure- only requires Secure,
// leaving Path free to stay scoped to /login/oidc.
func (c *OIDCStateCookie) cookieName(secure bool) string {
	if secure {
		return "__Secure-leafwiki_oidc_state"
	}
	return "leafwiki_oidc_state"
}

type oidcFlowState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
}

// Issue stores state/nonce/verifier for a fresh login attempt. Path is
// scoped to /login/oidc so this cookie is never sent on unrelated requests.
func (c *OIDCStateCookie) Issue(ctx *gin.Context, state, nonce, verifier string) error {
	secure, err := utils.RequireSecure(ctx, c.AllowInsecure)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(oidcFlowState{State: state, Nonce: nonce, Verifier: verifier})
	if err != nil {
		return err
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     c.cookieName(secure),
		Value:    base64.RawURLEncoding.EncodeToString(payload),
		Path:     "/login/oidc",
		HttpOnly: true,
		Secure:   secure,
		SameSite: c.SameSite,
		MaxAge:   int(c.TTL.Seconds()),
	})
	return nil
}

// ReadAndClear reads back the values Issue stored and immediately clears the
// cookie (MaxAge -1) so it's single-use regardless of whether the caller
// goes on to succeed or fail the rest of the callback.
func (c *OIDCStateCookie) ReadAndClear(ctx *gin.Context) (state, nonce, verifier string, err error) {
	secure, err := utils.RequireSecure(ctx, c.AllowInsecure)
	if err != nil {
		return "", "", "", err
	}
	name := c.cookieName(secure)

	defer func() {
		http.SetCookie(ctx.Writer, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/login/oidc",
			HttpOnly: true,
			Secure:   secure,
			SameSite: c.SameSite,
			MaxAge:   -1,
		})
	}()

	raw, err := ctx.Cookie(name)
	if err != nil {
		return "", "", "", err
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", "", "", err
	}
	var fs oidcFlowState
	if err := json.Unmarshal(payload, &fs); err != nil {
		return "", "", "", err
	}
	return fs.State, fs.Nonce, fs.Verifier, nil
}
