package mcp_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perber/wiki/internal/core/assets"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
	wikimcp "github.com/perber/wiki/internal/wiki/mcp"
	xoauth2 "golang.org/x/oauth2"
)

const (
	oauthClientID = "leafwiki-local-mcp"
	oauthScope    = "leafwiki:mcp"
)

func TestLocalMCPOAuthMetadata(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)

	tests := []struct {
		name              string
		basePath          string
		authMetadataPaths []string
		prMetadataPaths   []string
		issuer            string
		resource          string
	}{
		{
			name:              "root",
			authMetadataPaths: []string{"/.well-known/oauth-authorization-server"},
			prMetadataPaths:   []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"},
			issuer:            "http://leafwiki.local",
			resource:          "http://leafwiki.local/mcp",
		},
		{
			name:              "base path",
			basePath:          "/wiki",
			authMetadataPaths: []string{"/.well-known/oauth-authorization-server", "/.well-known/oauth-authorization-server/wiki"},
			prMetadataPaths:   []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource/wiki/mcp"},
			issuer:            "http://leafwiki.local/wiki",
			resource:          "http://leafwiki.local/wiki/mcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
				AllowInsecure:           true,
				BasePath:                tt.basePath,
				AccessTokenTimeout:      15 * time.Minute,
				RefreshTokenTimeout:     7 * 24 * time.Hour,
				MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
				MCPEnabled:              true,
			})

			for _, path := range tt.authMetadataPaths {
				authMeta := getJSONMap(t, router, "http://leafwiki.local"+path)
				assertStringField(t, authMeta, "issuer", tt.issuer)
				assertStringField(t, authMeta, "authorization_endpoint", tt.issuer+"/oauth/authorize")
				assertStringField(t, authMeta, "token_endpoint", tt.issuer+"/oauth/token")
				assertStringField(t, authMeta, "registration_endpoint", tt.issuer+"/oauth/register")
				assertStringSliceField(t, authMeta, "response_types_supported", []string{"code"})
				assertStringSliceField(t, authMeta, "grant_types_supported", []string{"authorization_code", "refresh_token"})
				assertStringSliceField(t, authMeta, "code_challenge_methods_supported", []string{"S256"})
				assertStringSliceField(t, authMeta, "scopes_supported", []string{oauthScope})
				assertStringSliceField(t, authMeta, "token_endpoint_auth_methods_supported", []string{"none"})
				if _, exists := authMeta["revocation_endpoint"]; exists {
					t.Fatalf("authorization metadata advertised revocation_endpoint: %#v", authMeta)
				}
			}

			for _, path := range tt.prMetadataPaths {
				rec := performRequest(t, router, http.MethodGet, "http://leafwiki.local"+path, nil, nil)
				if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
					t.Fatalf("GET %s content-type = %q, want JSON; body=%s", path, contentType, rec.Body.String())
				}
				prMeta := decodeJSONResponse(t, rec, http.StatusOK)
				assertStringField(t, prMeta, "resource", tt.resource)
				assertStringSliceField(t, prMeta, "authorization_servers", []string{tt.issuer})
				assertStringSliceField(t, prMeta, "scopes_supported", []string{oauthScope})

				optionsRec := performRequest(t, router, http.MethodOptions, "http://leafwiki.local"+path, nil, nil)
				if optionsRec.Code != http.StatusNoContent {
					t.Fatalf("OPTIONS protected resource metadata = %d, want 204: %s", optionsRec.Code, optionsRec.Body.String())
				}
			}
		})
	}
}

func TestLocalMCPOAuthDynamicClientRegistration(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	router := newLocalMCPTestRouter(w, oauthRouterOptions(""))

	invalidRegistrations := []struct {
		name string
		body string
	}{
		{
			name: "missing redirect uris",
			body: `{"token_endpoint_auth_method":"none"}`,
		},
		{
			name: "non loopback redirect uri",
			body: `{"redirect_uris":["http://example.com/callback"],"token_endpoint_auth_method":"none"}`,
		},
		{
			name: "confidential token auth method",
			body: `{"redirect_uris":["http://127.0.0.1:49152/callback"],"token_endpoint_auth_method":"client_secret_basic"}`,
		},
		{
			name: "client secret",
			body: `{"redirect_uris":["http://127.0.0.1:49152/callback"],"token_endpoint_auth_method":"none","client_secret":"secret"}`,
		},
		{
			name: "unsupported grant",
			body: `{"redirect_uris":["http://127.0.0.1:49152/callback"],"grant_types":["client_credentials"],"token_endpoint_auth_method":"none"}`,
		},
		{
			name: "unsupported response type",
			body: `{"redirect_uris":["http://127.0.0.1:49152/callback"],"response_types":["token"],"token_endpoint_auth_method":"none"}`,
		},
		{
			name: "unsupported scope",
			body: `{"redirect_uris":["http://127.0.0.1:49152/callback"],"scope":"leafwiki:mcp other","token_endpoint_auth_method":"none"}`,
		},
	}

	for _, tt := range invalidRegistrations {
		t.Run(tt.name, func(t *testing.T) {
			rec := performJSON(t, router, "http://leafwiki.local/oauth/register", tt.body)
			payload := decodeJSONResponse(t, rec, http.StatusBadRequest)
			assertStringField(t, payload, "error", "invalid_client_metadata")
		})
	}

	registration := registerOAuthClient(t, router, "", `{
		"client_name":"codex",
		"redirect_uris":["http://127.0.0.1:49152/callback"],
		"grant_types":["authorization_code","refresh_token"],
		"response_types":["code"],
		"token_endpoint_auth_method":"none",
		"scope":"leafwiki:mcp"
	}`)
	clientID := stringFromMap(t, registration, "client_id")
	if clientID == "" || clientID == oauthClientID {
		t.Fatalf("dynamic client_id = %q, want generated non-default id; payload=%#v", clientID, registration)
	}
	assertStringSliceField(t, registration, "redirect_uris", []string{"http://127.0.0.1:49152/callback"})
	assertStringField(t, registration, "token_endpoint_auth_method", "none")
	assertStringSliceField(t, registration, "grant_types", []string{"authorization_code", "refresh_token"})
	assertStringSliceField(t, registration, "response_types", []string{"code"})
	assertStringField(t, registration, "scope", oauthScope)

	cookies := loginCookies(t, router, "admin", "admin")
	redirectURI := "http://127.0.0.1:49152/callback"
	verifier := "oauth-dynamic-client-verifier-abcdefghijklmnopqrstuvwxyz0123456789"
	q := validAuthorizeQueryForClient(clientID, redirectURI, "dynamic-client-state", pkceS256(verifier), "http://leafwiki.local/mcp")
	rec := performRequest(t, router, http.MethodGet, "http://leafwiki.local/oauth/authorize?"+q.Encode(), cookies, nil)
	form := approvalFormFromAuthorizeRedirect(t, rec, "")
	approved := performFormWithCookiesAndHeaders(t, router, "http://leafwiki.local/oauth/authorize", form, cookies, nil)
	if approved.Code != http.StatusFound {
		t.Fatalf("dynamic client approved authorize = %d, want 302: %s", approved.Code, approved.Body.String())
	}
	redirected, err := url.Parse(approved.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse dynamic client authorize redirect: %v", err)
	}
	if got := redirected.Query().Get("state"); got != "dynamic-client-state" {
		t.Fatalf("dynamic client authorize state = %q, want dynamic-client-state", got)
	}
	code := redirected.Query().Get("code")
	if code == "" {
		t.Fatalf("dynamic client authorize redirect missing code: %s", redirected.String())
	}

	token := exchangeCodeForClient(t, router, "", clientID, code, redirectURI, verifier)
	assertStringField(t, token, "token_type", "Bearer")
	assertStringField(t, token, "scope", oauthScope)
	if stringFromMap(t, token, "access_token") == "" || stringFromMap(t, token, "refresh_token") == "" {
		t.Fatalf("dynamic client token response missing tokens: %#v", token)
	}
}

func TestLocalMCPOAuthDynamicClientRegistrationDefaultsRefreshAndBindsRefreshClient(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	router := newLocalMCPTestRouter(w, oauthRouterOptions(""))
	redirectURI := "http://127.0.0.1:49152/callback"

	registration := registerOAuthClient(t, router, "", `{
		"client_name":"Codex CLI",
		"redirect_uris":["http://127.0.0.1:49152/callback"],
		"response_types":["code"],
		"token_endpoint_auth_method":"none",
		"scope":"leafwiki:mcp"
	}`)
	clientID := stringFromMap(t, registration, "client_id")
	assertStringSliceField(t, registration, "grant_types", []string{"authorization_code", "refresh_token"})

	cookies := loginCookies(t, router, "admin", "admin")
	verifier := "oauth-dcr-default-refresh-verifier-abcdefghijklmnopqrstuvwxyz0123456789"
	q := validAuthorizeQueryForClient(clientID, redirectURI, "dcr-default-refresh-state", pkceS256(verifier), "http://leafwiki.local/mcp")
	rec := performRequest(t, router, http.MethodGet, "http://leafwiki.local/oauth/authorize?"+q.Encode(), cookies, nil)
	form := approvalFormFromAuthorizeRedirect(t, rec, "")
	details := approvalDetails(t, router, "", form.Get("approval_token"), cookies, nil)
	assertStringField(t, details, "clientLabel", "Codex CLI")
	assertStringField(t, details, "clientId", clientID)
	assertStringField(t, details, "redirectUri", redirectURI)
	assertStringField(t, details, "scope", oauthScope)
	assertStringField(t, details, "resource", "http://leafwiki.local/mcp")

	approved := performFormWithCookiesAndHeaders(t, router, "http://leafwiki.local/oauth/authorize", form, cookies, nil)
	if approved.Code != http.StatusFound {
		t.Fatalf("dynamic default client approved authorize = %d, want 302: %s", approved.Code, approved.Body.String())
	}
	redirected, err := url.Parse(approved.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse dynamic default client authorize redirect: %v", err)
	}
	code := redirected.Query().Get("code")
	if code == "" {
		t.Fatalf("dynamic default client authorize redirect missing code: %s", redirected.String())
	}
	token := exchangeCodeForClient(t, router, "", clientID, code, redirectURI, verifier)
	refreshToken := stringFromMap(t, token, "refresh_token")
	if refreshToken == "" {
		t.Fatalf("dynamic default client token response missing refresh token: %#v", token)
	}

	wrongClientRefresh := performForm(t, router, "http://leafwiki.local/oauth/token", url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oauthClientID},
		"refresh_token": {refreshToken},
	})
	wrongClientError := decodeJSONResponse(t, wrongClientRefresh, http.StatusUnauthorized)
	assertStringField(t, wrongClientError, "error", "invalid_grant")

	refreshed := decodeJSONResponse(t, performForm(t, router, "http://leafwiki.local/oauth/token", url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	}), http.StatusOK)
	if stringFromMap(t, refreshed, "access_token") == "" {
		t.Fatalf("dynamic default client refresh response missing access token: %#v", refreshed)
	}
}

func TestLocalMCPOAuthDynamicClientRegistrationHonorsAuthorizationCodeOnlyGrant(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	router := newLocalMCPTestRouter(w, oauthRouterOptions(""))
	redirectURI := "http://127.0.0.1:49152/callback"

	registration := registerOAuthClient(t, router, "", `{
		"client_name":"Auth Code Only Client",
		"redirect_uris":["http://127.0.0.1:49152/callback"],
		"grant_types":["authorization_code"],
		"response_types":["code"],
		"token_endpoint_auth_method":"none",
		"scope":"leafwiki:mcp"
	}`)
	clientID := stringFromMap(t, registration, "client_id")
	assertStringSliceField(t, registration, "grant_types", []string{"authorization_code"})

	cookies := loginCookies(t, router, "admin", "admin")
	verifier := "oauth-dcr-auth-code-only-verifier-abcdefghijklmnopqrstuvwxyz0123456789"
	q := validAuthorizeQueryForClient(clientID, redirectURI, "dcr-auth-code-only-state", pkceS256(verifier), "http://leafwiki.local/mcp")
	rec := performRequest(t, router, http.MethodGet, "http://leafwiki.local/oauth/authorize?"+q.Encode(), cookies, nil)
	form := approvalFormFromAuthorizeRedirect(t, rec, "")
	approved := performFormWithCookiesAndHeaders(t, router, "http://leafwiki.local/oauth/authorize", form, cookies, nil)
	if approved.Code != http.StatusFound {
		t.Fatalf("auth-code-only approved authorize = %d, want 302: %s", approved.Code, approved.Body.String())
	}
	redirected, err := url.Parse(approved.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse auth-code-only authorize redirect: %v", err)
	}
	code := redirected.Query().Get("code")
	if code == "" {
		t.Fatalf("auth-code-only authorize redirect missing code: %s", redirected.String())
	}
	token := exchangeCodeForClient(t, router, "", clientID, code, redirectURI, verifier)
	if stringFromMap(t, token, "access_token") == "" {
		t.Fatalf("auth-code-only client token response missing access token: %#v", token)
	}
	if refreshToken, ok := token["refresh_token"]; ok {
		t.Fatalf("auth-code-only client token response included refresh_token %#v; payload=%#v", refreshToken, token)
	}
}

func TestLocalMCPOAuthAuthorizeValidationAndLoginRedirect(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	router := newLocalMCPTestRouter(w, oauthRouterOptions(""))
	verifier := "oauth-test-verifier-abcdefghijklmnopqrstuvwxyz0123456789"
	challenge := pkceS256(verifier)
	validRedirect := "http://localhost:49152/callback"

	invalidRequests := []struct {
		name     string
		override func(url.Values)
	}{
		{name: "unknown client", override: func(q url.Values) { q.Set("client_id", "unknown-client") }},
		{name: "non loopback redirect", override: func(q url.Values) { q.Set("redirect_uri", "http://example.com/callback") }},
		{name: "unsupported loopback redirect", override: func(q url.Values) { q.Set("redirect_uri", "http://127.0.0.2:49152/callback") }},
		{name: "redirect fragment", override: func(q url.Values) { q.Set("redirect_uri", "http://localhost:49152/callback#frag") }},
	}

	for _, tt := range invalidRequests {
		t.Run(tt.name, func(t *testing.T) {
			q := validAuthorizeQuery(validRedirect, "state-1", challenge, "http://leafwiki.local/mcp")
			tt.override(q)
			rec := performRequest(t, router, http.MethodGet, "http://leafwiki.local/oauth/authorize?"+q.Encode(), nil, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("authorize %s = %d, want 400: %s", tt.name, rec.Code, rec.Body.String())
			}
		})
	}

	redirectedErrors := []struct {
		name      string
		override  func(url.Values)
		wantError string
	}{
		{name: "missing pkce", override: func(q url.Values) { q.Del("code_challenge") }, wantError: "invalid_request"},
		{name: "plain pkce", override: func(q url.Values) { q.Set("code_challenge_method", "plain") }, wantError: "invalid_request"},
		{name: "resource mismatch", override: func(q url.Values) { q.Set("resource", "http://leafwiki.local/not-mcp") }, wantError: "invalid_request"},
		{name: "mixed duplicate resource", override: func(q url.Values) { q.Add("resource", "http://leafwiki.local/not-mcp") }, wantError: "invalid_request"},
		{name: "unsupported scope", override: func(q url.Values) { q.Set("scope", "leafwiki:mcp other") }, wantError: "invalid_scope"},
	}

	for _, tt := range redirectedErrors {
		t.Run(tt.name+" redirects to client", func(t *testing.T) {
			q := validAuthorizeQuery(validRedirect, "redirect-error-state", challenge, "http://leafwiki.local/mcp")
			tt.override(q)
			rec := performRequest(t, router, http.MethodGet, "http://leafwiki.local/oauth/authorize?"+q.Encode(), nil, nil)
			if rec.Code != http.StatusFound {
				t.Fatalf("authorize %s = %d, want 302: %s", tt.name, rec.Code, rec.Body.String())
			}
			redirected, err := url.Parse(rec.Header().Get("Location"))
			if err != nil {
				t.Fatalf("parse authorize error redirect: %v", err)
			}
			if got := redirected.Scheme + "://" + redirected.Host + redirected.Path; got != validRedirect {
				t.Fatalf("authorize error redirect target = %q, want %q", got, validRedirect)
			}
			if got := redirected.Query().Get("state"); got != "redirect-error-state" {
				t.Fatalf("authorize error redirect state = %q, want redirect-error-state", got)
			}
			if got := redirected.Query().Get("error"); got != tt.wantError {
				t.Fatalf("authorize error = %q, want %q in %s", got, tt.wantError, redirected.String())
			}
			if code := redirected.Query().Get("code"); code != "" {
				t.Fatalf("authorize error redirect included code %q", code)
			}
		})
	}

	q := validAuthorizeQuery(validRedirect, "login-state", challenge, "http://leafwiki.local/mcp")
	authorizeURL := "http://leafwiki.local/oauth/authorize?" + q.Encode()
	rec := performRequest(t, router, http.MethodGet, authorizeURL, nil, nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("unauthenticated authorize = %d, want 302: %s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/login?") {
		t.Fatalf("unauthenticated authorize location = %q, want /login", location)
	}
	loginURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse login redirect %q: %v", location, err)
	}
	if got := loginURL.Query().Get("returnTo"); got != authorizeURL {
		t.Fatalf("login returnTo = %q, want %q", got, authorizeURL)
	}

	cookies := loginCookies(t, router, "admin", "admin")
	for _, redirectURI := range []string{
		"http://localhost:49152/callback",
		"http://127.0.0.1:49152/callback",
		"http://[::1]:49152/callback",
	} {
		t.Run("authenticated approval "+redirectURI, func(t *testing.T) {
			q := validAuthorizeQuery(redirectURI, "roundtrip-state", challenge, "")
			rec := performRequest(t, router, http.MethodGet, "http://leafwiki.local/oauth/authorize?"+q.Encode(), cookies, nil)
			form := approvalFormFromAuthorizeRedirect(t, rec, "")
			approved := performFormWithCookiesAndHeaders(t, router, "http://leafwiki.local/oauth/authorize", form, cookies, nil)
			if approved.Code != http.StatusFound {
				t.Fatalf("approved authorize = %d, want 302: %s", approved.Code, approved.Body.String())
			}
			redirected, err := url.Parse(approved.Header().Get("Location"))
			if err != nil {
				t.Fatalf("parse approved authorize redirect: %v", err)
			}
			if got := redirected.Query().Get("state"); got != "roundtrip-state" {
				t.Fatalf("redirect state = %q, want roundtrip-state", got)
			}
			if redirected.Query().Get("code") == "" {
				t.Fatalf("authorize redirect did not include code: %s", redirected.String())
			}
		})
	}
}

func TestLocalMCPOAuthRemoteUserAuthorizeRequiresApproval(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	trustedProxies, err := authmw.ParseTrustedProxies("192.0.2.1")
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	opts := oauthRouterOptions("")
	opts.HTTPRemoteUser = httpinternal.HTTPRemoteUserConfig{
		Enabled:        true,
		HeaderName:     "X-Remote-User",
		TrustedProxies: trustedProxies,
		UserService:    w.UserService(),
	}
	router := newLocalMCPTestRouter(w, opts)
	verifier := "oauth-remote-user-verifier-abcdefghijklmnopqrstuvwxyz0123456789"
	q := validAuthorizeQuery("http://localhost:49152/callback", "remote-user-state", pkceS256(verifier), "http://leafwiki.local/mcp")
	headers := map[string]string{"X-Remote-User": "admin"}

	rec := performRequestWithHeaders(t, router, http.MethodGet, "http://leafwiki.local/oauth/authorize?"+q.Encode(), nil, nil, headers)
	if strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Fatalf("remote-user authorize redirected to login: %q", rec.Header().Get("Location"))
	}

	form := approvalFormFromAuthorizeRedirect(t, rec, "")
	approved := performFormWithCookiesAndHeaders(t, router, "http://leafwiki.local/oauth/authorize", form, nil, headers)
	if approved.Code != http.StatusFound {
		t.Fatalf("approved remote-user authorize = %d, want 302: %s", approved.Code, approved.Body.String())
	}
	redirected, err := url.Parse(approved.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse remote-user authorize redirect: %v", err)
	}
	if got := redirected.Query().Get("state"); got != "remote-user-state" {
		t.Fatalf("remote-user authorize state = %q, want remote-user-state", got)
	}
	if redirected.Query().Get("code") == "" {
		t.Fatalf("remote-user authorize redirect missing code: %s", redirected.String())
	}
}

func TestLocalMCPOAuthTokenExchangeAndRefresh(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	router := newLocalMCPTestRouter(w, oauthRouterOptions(""))
	cookies := loginCookies(t, router, "admin", "admin")
	redirectURI := "http://localhost:49152/callback"
	verifier := "oauth-token-verifier-abcdefghijklmnopqrstuvwxyz0123456789"
	resource := "http://leafwiki.local/mcp"

	badCode := authorizeCode(t, router, cookies, redirectURI, "bad-verifier", verifier, resource)
	badForm := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {oauthClientID},
		"redirect_uri":  {redirectURI},
		"code":          {badCode},
		"code_verifier": {"wrong-verifier"},
	}
	badRec := performForm(t, router, "http://leafwiki.local/oauth/token", badForm)
	if badRec.Code == http.StatusOK {
		t.Fatalf("token exchange with wrong verifier succeeded: %s", badRec.Body.String())
	}
	if got := badRec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("token exchange error content-type = %q, want JSON", got)
	}
	badTokenError := decodeJSONResponse(t, badRec, http.StatusUnauthorized)
	assertStringField(t, badTokenError, "error", "invalid_grant")

	code := authorizeCode(t, router, cookies, redirectURI, "token-state", verifier, resource)
	token := exchangeCode(t, router, code, redirectURI, verifier)
	accessToken := stringFromMap(t, token, "access_token")
	refreshToken := stringFromMap(t, token, "refresh_token")
	assertStringField(t, token, "token_type", "Bearer")
	assertStringField(t, token, "scope", oauthScope)
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("token response missing access or refresh token: %#v", token)
	}

	refreshForm := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oauthClientID},
		"refresh_token": {refreshToken},
	}
	refreshed := decodeJSONResponse(t, performForm(t, router, "http://leafwiki.local/oauth/token", refreshForm), http.StatusOK)
	if stringFromMap(t, refreshed, "access_token") == "" {
		t.Fatalf("refresh token response missing access token: %#v", refreshed)
	}

	deleted, err := w.UserService().CreateUser("refresh-deleted", "refresh-deleted@example.com", "deletedpass", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("create refresh-deleted user: %v", err)
	}
	deletedCookies := loginCookies(t, router, "refresh-deleted", "deletedpass")
	deletedCode := authorizeCode(t, router, deletedCookies, redirectURI, "refresh-deleted-state", verifier+"2", resource)
	deletedToken := exchangeCode(t, router, deletedCode, redirectURI, verifier+"2")
	deletedRefresh := stringFromMap(t, deletedToken, "refresh_token")
	if err := w.UserService().DeleteUser(deleted.ID); err != nil {
		t.Fatalf("delete refresh-deleted user: %v", err)
	}
	deletedRefreshForm := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oauthClientID},
		"refresh_token": {deletedRefresh},
	}
	deletedRefreshError := decodeJSONResponse(t, performForm(t, router, "http://leafwiki.local/oauth/token", deletedRefreshForm), http.StatusUnauthorized)
	assertStringField(t, deletedRefreshError, "error", "invalid_grant")

	rec := performRequest(t, router, http.MethodPost, "http://leafwiki.local/oauth/revoke", nil, strings.NewReader(""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /oauth/revoke = %d, want 404", rec.Code)
	}
}

func TestLocalMCPOAuthTokenLifetimesComeFromWikiOptions(t *testing.T) {
	w := newLocalMCPAuthTestWikiWithOptions(t, wiki.WikiOptions{
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	opts := oauthRouterOptions("")
	opts.AccessTokenTimeout = time.Minute
	opts.RefreshTokenTimeout = 2 * time.Minute
	router := newLocalMCPTestRouter(w, opts)

	cookies := loginCookies(t, router, "admin", "admin")
	verifier := "oauth-lifetime-verifier-abcdefghijklmnopqrstuvwxyz0123456789"
	code := authorizeCode(t, router, cookies, "http://localhost:49152/callback", "lifetime-state", verifier, "http://leafwiki.local/mcp")
	token := exchangeCode(t, router, code, "http://localhost:49152/callback", verifier)

	expiresIn, ok := token["expires_in"].(float64)
	if !ok {
		t.Fatalf("expires_in has type %T, want number; payload=%#v", token["expires_in"], token)
	}
	if expiresIn < float64((14 * time.Minute).Seconds()) {
		t.Fatalf("expires_in = %v seconds, want wiki-configured lifetime near 15m", expiresIn)
	}
}

func TestLocalMCPOAuthExpiredBearerTokenRejected(t *testing.T) {
	w := newLocalMCPAuthTestWikiWithOptions(t, wiki.WikiOptions{AccessTokenTimeout: -time.Minute})
	trustedProxies, err := authmw.ParseTrustedProxies("192.0.2.1")
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	opts := oauthRouterOptions("")
	opts.HTTPRemoteUser = httpinternal.HTTPRemoteUserConfig{
		Enabled:        true,
		HeaderName:     "X-Remote-User",
		TrustedProxies: trustedProxies,
		UserService:    w.UserService(),
	}
	router := newLocalMCPTestRouter(w, opts)
	headers := map[string]string{"X-Remote-User": "admin"}
	verifier := "oauth-expired-verifier-abcdefghijklmnopqrstuvwxyz0123456789"
	q := validAuthorizeQuery("http://localhost:49152/callback", "expired-state", pkceS256(verifier), "http://leafwiki.local/mcp")
	rec := performRequestWithHeaders(t, router, http.MethodGet, "http://leafwiki.local/oauth/authorize?"+q.Encode(), nil, nil, headers)
	form := approvalFormFromAuthorizeRedirect(t, rec, "")
	approved := performFormWithCookiesAndHeaders(t, router, "http://leafwiki.local/oauth/authorize", form, nil, headers)
	if approved.Code != http.StatusFound {
		t.Fatalf("approved expired-token authorize = %d, want 302: %s", approved.Code, approved.Body.String())
	}
	redirected, err := url.Parse(approved.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse expired-token authorize redirect: %v", err)
	}
	code := redirected.Query().Get("code")
	if code == "" {
		t.Fatalf("expired-token authorize redirect missing code: %s", redirected.String())
	}
	token := stringFromMap(t, exchangeCode(t, router, code, "http://localhost:49152/callback", verifier), "access_token")

	req := httptest.NewRequest(http.MethodPost, "http://leafwiki.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /mcp with expired bearer = %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

func TestLocalMCPRegistration_AuthEnabledWritesCSRFAndAuthorParity(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	router := newLocalMCPTestRouter(w, oauthRouterOptions(""))
	admin, err := w.UserService().GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("get admin user: %v", err)
	}
	adminCookies := loginCookies(t, router, "admin", "admin")
	adminToken := oauthAccessTokenWithCookies(t, router, adminCookies, "admin-parity-state")
	adminSession := connectLocalMCPWithToken(t, router, "/mcp", adminToken)

	noCSRFBody := strings.NewReader(`{"title":"HTTP Missing CSRF","slug":"http-missing-csrf","kind":"page"}`)
	noCSRF := performRequest(t, router, http.MethodPost, "http://leafwiki.local/api/pages", adminCookies, noCSRFBody)
	if noCSRF.Code != http.StatusForbidden {
		t.Fatalf("POST /api/pages without CSRF = %d, want 403: %s", noCSRF.Code, noCSRF.Body.String())
	}

	page := exerciseOAuthWriterCRUD(t, router, adminSession, adminCookies, "admin", admin.ID)
	metadata := nestedMap(t, page, "metadata")
	creator := nestedMap(t, metadata, "creator")
	lastAuthor := nestedMap(t, metadata, "lastAuthor")
	assertStringField(t, metadata, "creatorId", admin.ID)
	assertStringField(t, metadata, "lastAuthorId", admin.ID)
	assertStringField(t, creator, "username", "admin")
	assertStringField(t, lastAuthor, "username", "admin")

	editor, err := w.UserService().CreateUser("oauth-editor", "oauth-editor@example.com", "editorpass", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("create oauth-editor user: %v", err)
	}
	editorCookies := loginCookies(t, router, "oauth-editor", "editorpass")
	editorToken := oauthAccessTokenWithCookies(t, router, editorCookies, "editor-crud-state")
	editorSession := connectLocalMCPWithToken(t, router, "/mcp", editorToken)
	_ = exerciseOAuthWriterCRUD(t, router, editorSession, editorCookies, "editor", editor.ID)
}

func exerciseOAuthWriterCRUD(t *testing.T, router http.Handler, session *sdkmcp.ClientSession, cookies []*http.Cookie, label, userID string) map[string]any {
	t.Helper()

	titlePrefix := strings.ToUpper(label[:1]) + label[1:]
	created := callToolStructured(t, session, "create_page", map[string]any{
		"title": titlePrefix + " OAuth Page",
		"slug":  label + "-oauth-page",
		"kind":  "page",
	})
	createdPage := nestedMap(t, created, "page")
	pageID := stringField(t, createdPage, "id")
	createdVersion := stringField(t, createdPage, "version")
	createdMetadata := nestedMap(t, createdPage, "metadata")
	assertStringField(t, createdMetadata, "creatorId", userID)
	assertStringField(t, createdMetadata, "lastAuthorId", userID)

	httpPage := decodeJSONResponse(t, performRequest(t, router, http.MethodGet, "http://leafwiki.local/api/pages/"+pageID, cookies, nil), http.StatusOK)
	httpMetadata := nestedMap(t, httpPage, "metadata")
	assertStringField(t, httpMetadata, "creatorId", userID)
	assertStringField(t, httpMetadata, "lastAuthorId", userID)
	assertStringField(t, httpPage, "id", pageID)

	updated := callToolStructured(t, session, "update_page", map[string]any{
		"id":      pageID,
		"version": createdVersion,
		"title":   titlePrefix + " OAuth Page Updated",
		"slug":    label + "-oauth-page",
		"content": "Updated over authenticated MCP\n",
	})
	updatedPage := nestedMap(t, updated, "page")
	updatedMetadata := nestedMap(t, updatedPage, "metadata")
	assertStringField(t, updatedMetadata, "lastAuthorId", userID)
	updatedVersion := stringField(t, updatedPage, "version")

	deleted := callToolStructured(t, session, "delete_page", map[string]any{
		"id":        pageID,
		"version":   updatedVersion,
		"recursive": false,
	})
	assertStringField(t, deleted, "message", "Page deleted")

	notFound := performRequest(t, router, http.MethodGet, "http://leafwiki.local/api/pages/"+pageID, cookies, nil)
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("GET deleted %s page = %d, want 404: %s", label, notFound.Code, notFound.Body.String())
	}
	return updatedPage
}

func TestLocalMCPRegistration_AuthEnabledOAuthBearerProtection(t *testing.T) {
	w := newLocalMCPAuthTestWikiWithOptions(t, wiki.WikiOptions{EnableRevision: true})
	opts := oauthRouterOptions("")
	opts.EnableRevision = true
	opts.EnableLinkRefactor = true
	router := newLocalMCPTestRouter(w, opts)

	rec := performRequest(t, router, http.MethodPost, "http://leafwiki.local/mcp", nil, strings.NewReader("{}"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /mcp without bearer = %d, want 401: %s", rec.Code, rec.Body.String())
	}
	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if !strings.Contains(wwwAuth, `resource_metadata="http://leafwiki.local/.well-known/oauth-protected-resource/mcp"`) ||
		!strings.Contains(wwwAuth, `scope="leafwiki:mcp"`) {
		t.Fatalf("WWW-Authenticate = %q, want resource metadata and scope", wwwAuth)
	}

	rec = performRequest(t, router, http.MethodPost, "http://leafwiki.local/mcp", nil, strings.NewReader("{}"))
	rec.Result().Body.Close()
	req := httptest.NewRequest(http.MethodPost, "http://leafwiki.local/mcp", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /mcp with invalid bearer = %d, want 401: %s", rec.Code, rec.Body.String())
	}

	adminToken := oauthAccessTokenForUser(t, router, "admin", "admin", "admin-state")
	adminSession := connectLocalMCPWithToken(t, router, "/mcp", adminToken)
	current := callToolStructured(t, adminSession, "get_current_user", nil)
	user := nestedMap(t, current, "user")
	assertStringField(t, user, "username", "admin")
	expectedTools := append(append([]string{}, baseToolNames...), wikimcp.RevisionToolNames()...)
	expectedTools = append(expectedTools, wikimcp.LinkRefactorToolNames()...)
	assertToolNames(t, listAllToolNames(t, adminSession), expectedTools)

	viewer, err := w.UserService().CreateUser("viewer", "viewer@example.com", "viewerpass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("create viewer user: %v", err)
	}
	viewerToken := oauthAccessTokenForUser(t, router, "viewer", "viewerpass", "viewer-state")
	viewerSession := connectLocalMCPWithToken(t, router, "/mcp", viewerToken)
	_ = callToolStructured(t, viewerSession, "get_tree", nil)

	page := nestedMap(t, callToolStructured(t, adminSession, "create_page", map[string]any{
		"title": "Viewer Gate Fixture",
		"slug":  "viewer-gate-fixture",
		"kind":  "page",
	}), "page")
	pageID := stringField(t, page, "id")
	pageVersion := stringField(t, page, "version")
	updatedContent := "viewer gate fixture revision"
	updated := nestedMap(t, callToolStructured(t, adminSession, "update_page", map[string]any{
		"id":      pageID,
		"version": pageVersion,
		"title":   "Viewer Gate Fixture",
		"slug":    "viewer-gate-fixture",
		"content": updatedContent,
	}), "page")
	currentVersion := stringField(t, updated, "version")
	_ = callToolStructured(t, adminSession, "upload_asset", map[string]any{
		"pageId":        pageID,
		"filename":      "viewer-gate.txt",
		"contentBase64": base64.StdEncoding.EncodeToString([]byte("viewer gate asset")),
	})
	latestRevision := nestedMap(t, callToolStructured(t, adminSession, "get_latest_revision", map[string]any{"pageId": pageID}), "revision")
	latestRevisionID := stringField(t, latestRevision, "id")

	editorOnlyTools := []struct {
		name string
		args map[string]any
	}{
		{name: "suggest_slug", args: map[string]any{"title": "Viewer Slug"}},
		{name: "create_page", args: map[string]any{"title": "Viewer Write", "slug": "viewer-write"}},
		{name: "update_page", args: map[string]any{"id": pageID, "version": currentVersion, "title": "Viewer Gate Fixture", "slug": "viewer-gate-fixture", "content": "viewer update"}},
		{name: "delete_page", args: map[string]any{"id": pageID, "version": currentVersion, "recursive": false}},
		{name: "move_page", args: map[string]any{"id": pageID, "version": currentVersion}},
		{name: "sort_pages", args: map[string]any{"parentId": "", "orderedIds": []any{pageID}}},
		{name: "ensure_page", args: map[string]any{"path": "viewer/ensured", "title": "Viewer Ensured"}},
		{name: "convert_page", args: map[string]any{"id": pageID, "version": currentVersion, "targetKind": "section"}},
		{name: "copy_page", args: map[string]any{"id": pageID, "title": "Viewer Copy", "slug": "viewer-copy"}},
		{name: "upload_asset", args: map[string]any{"pageId": pageID, "filename": "viewer.txt", "contentBase64": base64.StdEncoding.EncodeToString([]byte("viewer"))}},
		{name: "rename_asset", args: map[string]any{"pageId": pageID, "oldFilename": "viewer-gate.txt", "newFilename": "viewer-renamed.txt"}},
		{name: "delete_asset", args: map[string]any{"pageId": pageID, "filename": "viewer-gate.txt"}},
		{name: "restore_revision", args: map[string]any{"pageId": pageID, "revisionId": latestRevisionID}},
		{name: "preview_page_refactor", args: map[string]any{"pageId": pageID, "kind": "page", "title": "Viewer Preview", "slug": "viewer-preview"}},
		{name: "apply_page_refactor", args: map[string]any{"pageId": pageID, "version": currentVersion, "kind": "page", "title": "Viewer Apply", "slug": "viewer-apply"}},
	}
	for _, tt := range editorOnlyTools {
		t.Run("viewer denied "+tt.name, func(t *testing.T) {
			errText := callToolError(t, viewerSession, tt.name, tt.args)
			if !strings.Contains(strings.ToLower(errText), "editor") && !strings.Contains(strings.ToLower(errText), "admin") {
				t.Fatalf("viewer %s error = %q, want editor/admin permission detail", tt.name, errText)
			}
		})
	}
	_ = viewer

	editor, err := w.UserService().CreateUser("editor", "editor@example.com", "editorpass", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("create editor user: %v", err)
	}
	editorToken := oauthAccessTokenForUser(t, router, "editor", "editorpass", "editor-state")
	if _, err := w.UserService().UpdateUser(editor.ID, editor.Username, editor.Email, "", coreauth.RoleViewer); err != nil {
		t.Fatalf("downgrade editor user: %v", err)
	}
	downgradedSession := connectLocalMCPWithToken(t, router, "/mcp", editorToken)
	downgradedErr := callToolError(t, downgradedSession, "create_page", map[string]any{
		"title": "Downgraded Write",
		"slug":  "downgraded-write",
	})
	if !strings.Contains(strings.ToLower(downgradedErr), "editor") && !strings.Contains(strings.ToLower(downgradedErr), "admin") {
		t.Fatalf("downgraded create_page error = %q, want editor/admin permission detail", downgradedErr)
	}

	deleted, err := w.UserService().CreateUser("deleted", "deleted@example.com", "deletedpass", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("create deleted user: %v", err)
	}
	deletedToken := oauthAccessTokenForUser(t, router, "deleted", "deletedpass", "deleted-state")
	if err := w.UserService().DeleteUser(deleted.ID); err != nil {
		t.Fatalf("delete user before MCP request: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "http://leafwiki.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Authorization", "Bearer "+deletedToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /mcp with deleted user token = %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

func TestLocalMCPRegistration_AuthEnabledBasePathMetadataChallenge(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	router := newLocalMCPTestRouter(w, oauthRouterOptions("/wiki"))

	rec := performRequest(t, router, http.MethodPost, "http://leafwiki.local/wiki/mcp", nil, strings.NewReader("{}"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /wiki/mcp without bearer = %d, want 401: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, `resource_metadata="http://leafwiki.local/.well-known/oauth-protected-resource/wiki/mcp"`) {
		t.Fatalf("base-path WWW-Authenticate = %q, want base-path resource metadata", got)
	}
}

func TestLocalMCPRegistration_AuthEnabledBasePathOAuthSession(t *testing.T) {
	w := newLocalMCPAuthTestWiki(t)
	router := newLocalMCPTestRouter(w, oauthRouterOptions("/wiki"))
	cookies := loginCookiesAt(t, router, "/wiki", "admin", "admin")
	token := oauthAccessTokenWithCookiesAt(t, router, cookies, "/wiki", "base-path-session-state", "http://leafwiki.local/wiki/mcp")

	session := connectLocalMCPWithToken(t, router, "/wiki/mcp", token)
	current := callToolStructured(t, session, "get_current_user", nil)
	user := nestedMap(t, current, "user")
	assertStringField(t, user, "username", "admin")

	config := callToolStructured(t, session, "get_config", nil)
	assertStringField(t, config, "basePath", "/wiki")
	assertToolNames(t, listAllToolNames(t, session), baseToolNames)
}

type staticOAuthHandler struct {
	token string
}

func (h staticOAuthHandler) TokenSource(context.Context) (xoauth2.TokenSource, error) {
	return xoauth2.StaticTokenSource(&xoauth2.Token{AccessToken: h.token}), nil
}

func (h staticOAuthHandler) Authorize(context.Context, *http.Request, *http.Response) error {
	return fmt.Errorf("unexpected oauth authorize callback")
}

func newLocalMCPAuthTestWiki(t *testing.T) *wiki.Wiki {
	t.Helper()

	return newLocalMCPAuthTestWikiWithOptions(t, wiki.WikiOptions{})
}

func newLocalMCPAuthTestWikiWithOptions(t *testing.T, overrides wiki.WikiOptions) *wiki.Wiki {
	t.Helper()

	options := wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		EnableRevision:      overrides.EnableRevision,
		MaxRevisionHistory:  overrides.MaxRevisionHistory,
	}
	if overrides.AccessTokenTimeout != 0 {
		options.AccessTokenTimeout = overrides.AccessTokenTimeout
	}
	if overrides.RefreshTokenTimeout != 0 {
		options.RefreshTokenTimeout = overrides.RefreshTokenTimeout
	}

	w, err := wiki.NewWiki(&options)
	if err != nil {
		t.Fatalf("NewWiki failed: %v", err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Fatalf("Close wiki failed: %v", err)
		}
	})
	return w
}

func oauthRouterOptions(basePath string) httpinternal.RouterOptions {
	return httpinternal.RouterOptions{
		AllowInsecure:           true,
		BasePath:                basePath,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	}
}

func getJSONMap(t *testing.T, router http.Handler, target string) map[string]any {
	t.Helper()

	return decodeJSONResponse(t, performRequest(t, router, http.MethodGet, target, nil, nil), http.StatusOK)
}

func decodeJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) map[string]any {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("response status = %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode JSON response: %v; body=%s", err, rec.Body.String())
	}
	return out
}

func performRequest(t *testing.T, router http.Handler, method, target string, cookies []*http.Cookie, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	return performRequestWithHeaders(t, router, method, target, cookies, body, nil)
}

func performRequestWithHeaders(t *testing.T, router http.Handler, method, target string, cookies []*http.Cookie, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, body)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func performJSON(t *testing.T, router http.Handler, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func registerOAuthClient(t *testing.T, router http.Handler, basePath, body string) map[string]any {
	t.Helper()

	return decodeJSONResponse(t, performJSON(t, router, "http://leafwiki.local"+basePath+"/oauth/register", body), http.StatusCreated)
}

func performForm(t *testing.T, router http.Handler, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	return performFormWithCookiesAndHeaders(t, router, target, form, nil, nil)
}

func performFormWithCookiesAndHeaders(t *testing.T, router http.Handler, target string, form url.Values, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func approvalFormFromAuthorizeRedirect(t *testing.T, rec *httptest.ResponseRecorder, basePath string) url.Values {
	t.Helper()

	if rec.Code != http.StatusFound {
		t.Fatalf("authorize approval redirect = %d, want 302: %s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	redirected, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse approval redirect %q: %v", location, err)
	}
	if got, want := redirected.Path, basePath+"/oauth/approve"; got != want {
		t.Fatalf("approval redirect path = %q, want %q; location=%s", got, want, location)
	}
	form := redirected.Query()
	if form.Get("approval_token") == "" {
		t.Fatalf("approval redirect missing approval_token: %s", location)
	}
	form.Set("decision", "approve")
	return form
}

func approvalDetails(t *testing.T, router http.Handler, basePath, token string, cookies []*http.Cookie, headers map[string]string) map[string]any {
	t.Helper()

	target := "http://leafwiki.local" + basePath + "/oauth/approval?approval_token=" + url.QueryEscape(token)
	return decodeJSONResponse(t, performRequestWithHeaders(t, router, http.MethodGet, target, cookies, nil, headers), http.StatusOK)
}

func validAuthorizeQuery(redirectURI, state, challenge, resource string) url.Values {
	return validAuthorizeQueryForClient(oauthClientID, redirectURI, state, challenge, resource)
}

func validAuthorizeQueryForClient(clientID, redirectURI, state, challenge, resource string) url.Values {
	q := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {oauthScope},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	if resource != "" {
		q.Set("resource", resource)
	}
	return q
}

func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return strings.TrimRight(base64.URLEncoding.EncodeToString(sum[:]), "=")
}

func loginCookies(t *testing.T, router http.Handler, identifier, password string) []*http.Cookie {
	t.Helper()

	return loginCookiesAt(t, router, "", identifier, password)
}

func loginCookiesAt(t *testing.T, router http.Handler, basePath, identifier, password string) []*http.Cookie {
	t.Helper()

	body := fmt.Sprintf(`{"identifier":%q,"password":%q}`, identifier, password)
	req := httptest.NewRequest(http.MethodPost, "http://leafwiki.local"+basePath+"/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s = %d, want 200: %s", identifier, rec.Code, rec.Body.String())
	}
	return rec.Result().Cookies()
}

func authorizeCode(t *testing.T, router http.Handler, cookies []*http.Cookie, redirectURI, state, verifier, resource string) string {
	t.Helper()

	return authorizeCodeAt(t, router, cookies, "", redirectURI, state, verifier, resource)
}

func authorizeCodeAt(t *testing.T, router http.Handler, cookies []*http.Cookie, basePath, redirectURI, state, verifier, resource string) string {
	t.Helper()

	q := validAuthorizeQuery(redirectURI, state, pkceS256(verifier), resource)
	authorizeURL := "http://leafwiki.local" + basePath + "/oauth/authorize"
	rec := performRequest(t, router, http.MethodGet, authorizeURL+"?"+q.Encode(), cookies, nil)
	form := approvalFormFromAuthorizeRedirect(t, rec, basePath)
	approved := performFormWithCookiesAndHeaders(t, router, authorizeURL, form, cookies, nil)
	if approved.Code != http.StatusFound {
		t.Fatalf("approved authorize for code = %d, want 302: %s", approved.Code, approved.Body.String())
	}
	redirected, err := url.Parse(approved.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize redirect: %v", err)
	}
	code := redirected.Query().Get("code")
	if code == "" {
		t.Fatalf("authorize redirect missing code: %s", redirected.String())
	}
	if got := redirected.Query().Get("state"); got != state {
		t.Fatalf("authorize redirect state = %q, want %q", got, state)
	}
	return code
}

func exchangeCode(t *testing.T, router http.Handler, code, redirectURI, verifier string) map[string]any {
	t.Helper()

	return exchangeCodeAt(t, router, "", code, redirectURI, verifier)
}

func exchangeCodeAt(t *testing.T, router http.Handler, basePath, code, redirectURI, verifier string) map[string]any {
	t.Helper()

	return exchangeCodeForClient(t, router, basePath, oauthClientID, code, redirectURI, verifier)
}

func exchangeCodeForClient(t *testing.T, router http.Handler, basePath, clientID, code, redirectURI, verifier string) map[string]any {
	t.Helper()

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"code":          {code},
		"code_verifier": {verifier},
	}
	return decodeJSONResponse(t, performForm(t, router, "http://leafwiki.local"+basePath+"/oauth/token", form), http.StatusOK)
}

func oauthAccessTokenForUser(t *testing.T, router http.Handler, username, password, state string) string {
	t.Helper()

	cookies := loginCookies(t, router, username, password)
	return oauthAccessTokenWithCookies(t, router, cookies, state)
}

func oauthAccessTokenWithCookies(t *testing.T, router http.Handler, cookies []*http.Cookie, state string) string {
	t.Helper()

	return oauthAccessTokenWithCookiesAt(t, router, cookies, "", state, "http://leafwiki.local/mcp")
}

func oauthAccessTokenWithCookiesAt(t *testing.T, router http.Handler, cookies []*http.Cookie, basePath, state, resource string) string {
	t.Helper()

	verifier := "oauth-access-verifier-" + state + "-abcdefghijklmnopqrstuvwxyz0123456789"
	code := authorizeCodeAt(t, router, cookies, basePath, "http://localhost:49152/callback", state, verifier, resource)
	return stringFromMap(t, exchangeCodeAt(t, router, basePath, code, "http://localhost:49152/callback", verifier), "access_token")
}

func connectLocalMCPWithToken(t *testing.T, handler http.Handler, path, token string) *sdkmcp.ClientSession {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "leafwiki-test", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{
		Endpoint:             server.URL + path,
		HTTPClient:           server.Client(),
		DisableStandaloneSSE: true,
		OAuthHandler:         staticOAuthHandler{token: token},
	}, nil)
	if err != nil {
		t.Fatalf("Connect MCP client with token failed: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func assertStringField(t *testing.T, got map[string]any, field, want string) {
	t.Helper()

	if value := stringFromMap(t, got, field); value != want {
		t.Fatalf("%s = %q, want %q; payload=%#v", field, value, want, got)
	}
}

func stringFromMap(t *testing.T, got map[string]any, field string) string {
	t.Helper()

	value, ok := got[field].(string)
	if !ok {
		t.Fatalf("%s has type %T, want string; payload=%#v", field, got[field], got)
	}
	return value
}

func assertStringSliceField(t *testing.T, got map[string]any, field string, want []string) {
	t.Helper()

	raw, ok := got[field].([]any)
	if !ok {
		t.Fatalf("%s has type %T, want array; payload=%#v", field, got[field], got)
	}
	if len(raw) != len(want) {
		t.Fatalf("%s length = %d, want %d; payload=%#v", field, len(raw), len(want), got)
	}
	for i, expected := range want {
		value, ok := raw[i].(string)
		if !ok || value != expected {
			t.Fatalf("%s[%d] = %#v, want %q; payload=%#v", field, i, raw[i], expected, got)
		}
	}
}

var _ sdkauth.OAuthHandler = staticOAuthHandler{}
