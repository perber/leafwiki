package oidc

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	coreoidc "github.com/perber/wiki/internal/core/oidc"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the OIDC login flow.
type Routes struct {
	startLogin      *StartOIDCLoginUseCase
	callbackUseCase *HandleOIDCCallbackUseCase
	oidcStateCookie *authmw.OIDCStateCookie
}

// RoutesConfig holds the dependencies to build an oidc Routes instance.
type RoutesConfig struct {
	StartLogin     *StartOIDCLoginUseCase
	HandleCallback *HandleOIDCCallbackUseCase
}

// NewRoutes constructs the oidc RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		startLogin:      cfg.StartLogin,
		callbackUseCase: cfg.HandleCallback,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	r.oidcStateCookie = ctx.OIDCStateCookie

	loginRateLimiter := security.NewRateLimiter(10, 5*time.Minute, true)

	nonAuth := ctx.Base.Group("/api")
	nonAuth.GET("/auth/oidc/login", loginRateLimiter, r.handleStartLogin(ctx))

	// NOT under /api, and NOT the SPA's catch-all: a specific route
	// registered on ctx.Base is matched before gin's NoRoute fallback
	// (which is what serves the SPA's index.html for everything else)
	// regardless of registration order, so this intercepts the provider's
	// redirect before the frontend ever sees this path — see the OIDC plan
	// for why the redirect_uri registered with the provider is deliberately
	// this frontend-looking, non-/api path.
	ctx.Base.GET("/login/oidc/callback", loginRateLimiter, r.handleCallback(ctx))
}

func (r *Routes) handleStartLogin(ctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		out, err := r.startLogin.Execute(c.Request.Context())
		if err != nil {
			respondOIDCError(c, ctx, err)
			return
		}
		if err := r.oidcStateCookie.Issue(c, out.State, out.Nonce, out.Verifier); err != nil {
			respondOIDCError(c, ctx, err)
			return
		}
		c.Redirect(http.StatusFound, out.AuthURL)
	}
}

func (r *Routes) handleCallback(ctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		if errMsg := c.Query("error"); errMsg != "" {
			// The provider itself rejected/aborted the flow (e.g. user denied
			// consent) — nothing to exchange, straight to the error redirect.
			redirectToLoginWithError(c, ctx, "provider_error")
			return
		}

		code := c.Query("code")
		state := c.Query("state")

		expectedState, expectedNonce, verifier, err := r.oidcStateCookie.ReadAndClear(c)
		if err != nil {
			redirectToLoginWithError(c, ctx, "state_mismatch")
			return
		}

		out, err := r.callbackUseCase.Execute(c.Request.Context(), HandleOIDCCallbackInput{
			Code:          code,
			State:         state,
			ExpectedState: expectedState,
			Verifier:      verifier,
			ExpectedNonce: expectedNonce,
		})
		if err != nil {
			redirectToLoginWithError(c, ctx, errorCode(err))
			return
		}

		if _, err := ctx.CSRFCookie.Issue(c); err != nil {
			redirectToLoginWithError(c, ctx, "cookie_failed")
			return
		}
		if err := ctx.AuthCookies.Set(c, out.Token.Token, out.Token.RefreshToken); err != nil {
			redirectToLoginWithError(c, ctx, "cookie_failed")
			return
		}

		target := ctx.Opts.BasePath
		if target == "" {
			target = "/"
		} else {
			target += "/"
		}
		c.Redirect(http.StatusFound, target)
	}
}

// respondOIDCError handles a failure starting the flow (before any redirect
// to the provider happened) — still safe to send the browser to /login
// directly, same as a callback failure.
func respondOIDCError(c *gin.Context, ctx httpinternal.RouterContext, err error) {
	redirectToLoginWithError(c, ctx, errorCode(err))
}

func redirectToLoginWithError(c *gin.Context, ctx httpinternal.RouterContext, code string) {
	loginPath := ctx.Opts.BasePath + "/login"
	q := url.Values{"oidc_error": {code}}
	c.Redirect(http.StatusFound, loginPath+"?"+q.Encode())
}

// errorCode maps an internal error to a short, non-sensitive code suitable
// for a query-string redirect — never the raw error message, which could
// leak provider implementation details.
func errorCode(err error) string {
	switch {
	case errors.Is(err, coreoidc.ErrStateMismatch):
		return "state_mismatch"
	case errors.Is(err, coreoidc.ErrProviderExchangeFailed):
		return "exchange_failed"
	case errors.Is(err, coreoidc.ErrIDTokenInvalid), errors.Is(err, coreoidc.ErrMissingIDToken):
		return "invalid_token"
	case errors.Is(err, coreauth.ErrEditorLimitReached):
		return "editor_limit"
	case errors.Is(err, ErrOIDCDisabled):
		return "disabled"
	default:
		return "unknown"
	}
}
