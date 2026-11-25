package http_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/perber/wiki/internal/core/assets"
	coreoidc "github.com/perber/wiki/internal/core/oidc"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/wiki"
)

// fakeOIDCProvider is a minimal, self-contained stand-in for a real OIDC
// provider (Authentik), used because there's no way to exercise the real
// authorization-code+PKCE+ID-token-verification flow in CI without one.
//
// Since these tests never drive a real browser through the provider's own
// login page, there's no natural place for the fake provider to learn the
// nonce the client generated for a given flow (a real provider would see it
// in the authorization request's query string). Instead, the authorization
// "code" IS the encoded claims the test wants the eventual ID token to
// carry — test code captures the real nonce from the login-redirect's
// Location header and bakes it into the code it then presents to the
// callback. This keeps the fake server itself completely stateless.
type fakeOIDCProvider struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	issuer     string
	clientID   string
}

type fakeIDTokenClaims struct {
	Sub               string   `json:"sub"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	Groups            []string `json:"groups"`
	Nonce             string   `json:"nonce"`
}

const fakeOIDCKeyID = "test-key-1"

func newFakeOIDCProvider(t *testing.T, clientID string) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	p := &fakeOIDCProvider{privateKey: key, clientID: clientID}
	mux := http.NewServeMux()
	p.server = httptest.NewServer(mux)
	p.issuer = p.server.URL + "/op/"

	mux.HandleFunc("/op/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                p.issuer,
			"authorization_endpoint":                p.server.URL + "/op/authorize/",
			"token_endpoint":                        p.server.URL + "/op/token/",
			"jwks_uri":                              p.server.URL + "/op/jwks/",
			"end_session_endpoint":                  p.server.URL + "/op/end-session/",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})

	mux.HandleFunc("/op/jwks/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"use": "sig",
					"alg": "RS256",
					"kid": fakeOIDCKeyID,
					"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
				},
			},
		})
	})

	mux.HandleFunc("/op/token/", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		code := r.PostForm.Get("code")
		claims, err := p.decodeCode(code)
		if err != nil {
			http.Error(w, "invalid code", http.StatusBadRequest)
			return
		}

		idToken, err := p.signIDToken(claims)
		if err != nil {
			http.Error(w, "failed to sign id token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"id_token":     idToken,
			"expires_in":   3600,
		})
	})

	t.Cleanup(p.server.Close)
	return p
}

// encodeCode packs the claims the fake token endpoint should hand back into
// an opaque "authorization code" — see fakeOIDCProvider's doc comment.
func (p *fakeOIDCProvider) encodeCode(claims fakeIDTokenClaims) string {
	b, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (p *fakeOIDCProvider) decodeCode(code string) (fakeIDTokenClaims, error) {
	var claims fakeIDTokenClaims
	b, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return claims, err
	}
	err = json.Unmarshal(b, &claims)
	return claims, err
}

func (p *fakeOIDCProvider) signIDToken(claims fakeIDTokenClaims) (string, error) {
	now := time.Now()
	mapClaims := jwt.MapClaims{
		"iss":                claims.Sub, // placeholder, overwritten below
		"sub":                claims.Sub,
		"aud":                p.clientID,
		"exp":                now.Add(5 * time.Minute).Unix(),
		"iat":                now.Unix(),
		"nonce":              claims.Nonce,
		"preferred_username": claims.PreferredUsername,
		"email":              claims.Email,
		"groups":             claims.Groups,
	}
	mapClaims["iss"] = p.issuer

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, mapClaims)
	token.Header["kid"] = fakeOIDCKeyID
	return token.SignedString(p.privateKey)
}

// extractQueryParam pulls a single query parameter out of a URL string.
func extractQueryParam(t *testing.T, rawURL, key string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL %q: %v", rawURL, err)
	}
	return u.Query().Get(key)
}

func createRouterTestInstanceWithOIDC(w *wiki.Wiki, t *testing.T) *gin.Engine {
	svc := w.OIDCService()
	var buttonLabel string
	if svc != nil {
		buttonLabel = svc.ButtonLabel()
	}
	return httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            publicaccess.NewEnvManaged(false),
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		OIDCEnabled:             svc != nil,
		OIDCButtonLabel:         buttonLabel,
		OIDCOnly:                svc != nil && svc.OIDCOnly(),
	})
}

func createOIDCWikiTestInstance(t *testing.T, provider *fakeOIDCProvider, extra coreoidc.Config) *wiki.Wiki {
	cfg := coreoidc.Config{
		Issuer:       provider.issuer,
		ClientID:     provider.clientID,
		ClientSecret: "test-client-secret",
		RedirectURL:  "https://wiki.example.com/login/oidc/callback",
		ButtonLabel:  "authentik",
		AdminGroup:   "wiki-admins",
		EditorGroup:  "wiki-editors",
	}
	if extra.OIDCOnly {
		cfg.OIDCOnly = true
	}
	if extra.RoleSync != "" {
		cfg.RoleSync = extra.RoleSync
	}
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		OIDC:                cfg,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with OIDC: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// doOIDCLogin drives the full flow against router for a single simulated
// user and returns the final callback response (after-redirect state is
// NOT followed automatically — callers inspect rec directly).
func doOIDCLogin(t *testing.T, router http.Handler, provider *fakeOIDCProvider, claims fakeIDTokenClaims) *httptest.ResponseRecorder {
	t.Helper()

	startReq := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil)
	startRec := httptest.NewRecorder()
	router.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusFound {
		t.Fatalf("GET /api/auth/oidc/login: expected 302, got %d - %s", startRec.Code, startRec.Body.String())
	}

	authURL := startRec.Header().Get("Location")
	if authURL == "" {
		t.Fatal("expected a Location header redirecting to the provider")
	}
	state := extractQueryParam(t, authURL, "state")
	nonce := extractQueryParam(t, authURL, "nonce")
	if state == "" || nonce == "" {
		t.Fatalf("expected state and nonce in the authorize URL, got %q", authURL)
	}

	var stateCookie *http.Cookie
	for _, c := range startRec.Result().Cookies() {
		if strings.Contains(c.Name, "oidc_state") {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("expected an oidc_state cookie to be set")
	}

	claims.Nonce = nonce
	code := provider.encodeCode(claims)

	callbackReq := httptest.NewRequest(http.MethodGet, "/login/oidc/callback?code="+code+"&state="+state, nil)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	router.ServeHTTP(callbackRec, callbackReq)
	return callbackRec
}

func TestOIDC_NewUser_CreatedWithMappedRoleAndLogsIn(t *testing.T) {
	provider := newFakeOIDCProvider(t, "leafwiki")
	w := createOIDCWikiTestInstance(t, provider, coreoidc.Config{})
	router := createRouterTestInstanceWithOIDC(w, t)

	rec := doOIDCLogin(t, router, provider, fakeIDTokenClaims{
		Sub:               "sub-carol",
		PreferredUsername: "carol",
		Email:             "carol@example.com",
		Groups:            []string{"wiki-admins"},
	})

	if rec.Code != http.StatusFound {
		t.Fatalf("callback: expected 302, got %d - %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("expected redirect to /, got %q", loc)
	}

	var authCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if strings.Contains(c.Name, "_at") {
			authCookie = c
		}
	}
	if authCookie == nil {
		t.Fatal("expected an access-token cookie to be set after a successful callback")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(authCookie)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)

	var me struct {
		Username   string `json:"username"`
		Role       string `json:"role"`
		AuthSource string `json:"authSource"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &me); err != nil {
		t.Fatalf("unmarshal /api/auth/me: %v", err)
	}
	if me.Username != "carol" || me.Role != "admin" || me.AuthSource != "oidc" {
		t.Fatalf("unexpected /api/auth/me response: %+v", me)
	}
}

func TestOIDC_StateMismatch_RedirectsToLoginWithError(t *testing.T) {
	provider := newFakeOIDCProvider(t, "leafwiki")
	w := createOIDCWikiTestInstance(t, provider, coreoidc.Config{})
	router := createRouterTestInstanceWithOIDC(w, t)

	startReq := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil)
	startRec := httptest.NewRecorder()
	router.ServeHTTP(startRec, startReq)

	var stateCookie *http.Cookie
	for _, c := range startRec.Result().Cookies() {
		if strings.Contains(c.Name, "oidc_state") {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("expected an oidc_state cookie")
	}

	code := provider.encodeCode(fakeIDTokenClaims{Sub: "sub-x", PreferredUsername: "x"})
	callbackReq := httptest.NewRequest(http.MethodGet, "/login/oidc/callback?code="+code+"&state=wrong-state", nil)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	router.ServeHTTP(callbackRec, callbackReq)

	if callbackRec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d - %s", callbackRec.Code, callbackRec.Body.String())
	}
	loc := callbackRec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?") || !strings.Contains(loc, "oidc_error=state_mismatch") {
		t.Fatalf("expected a /login redirect with oidc_error=state_mismatch, got %q", loc)
	}
}

func TestOIDC_ExistingLocalAccount_LinkedByUsername_NotDuplicated(t *testing.T) {
	provider := newFakeOIDCProvider(t, "leafwiki")
	w := createOIDCWikiTestInstance(t, provider, coreoidc.Config{})
	router := createRouterTestInstanceWithOIDC(w, t)

	if _, err := w.UserService().CreateUser("dave", "dave@local.example.com", "password123", "editor"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	rec := doOIDCLogin(t, router, provider, fakeIDTokenClaims{
		Sub:               "sub-dave",
		PreferredUsername: "dave",
		Email:             "dave@idp.example.com",
		Groups:            nil,
	})
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("expected a successful login redirect, got %d %q", rec.Code, rec.Header().Get("Location"))
	}

	users, err := w.UserService().GetUsers()
	if err != nil {
		t.Fatalf("GetUsers: %v", err)
	}
	count := 0
	for _, u := range users {
		if u.Username == "dave" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one 'dave' user (linked, not duplicated), got %d", count)
	}
}

func TestOIDC_RoleChangedSinceLastLogin_ResyncsOnEveryLogin(t *testing.T) {
	provider := newFakeOIDCProvider(t, "leafwiki")
	w := createOIDCWikiTestInstance(t, provider, coreoidc.Config{})
	router := createRouterTestInstanceWithOIDC(w, t)

	// First login: admin group.
	doOIDCLogin(t, router, provider, fakeIDTokenClaims{
		Sub: "sub-carol", PreferredUsername: "carol", Groups: []string{"wiki-admins"},
	})
	carol, err := w.UserService().GetUserByUsername("carol")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if carol.Role != "admin" {
		t.Fatalf("expected admin role after first login, got %q", carol.Role)
	}

	// Second login: no longer in any mapped group.
	doOIDCLogin(t, router, provider, fakeIDTokenClaims{
		Sub: "sub-carol", PreferredUsername: "carol", Groups: nil,
	})
	carol, err = w.UserService().GetUserByID(carol.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if carol.Role != "viewer" {
		t.Fatalf("expected role downgraded to viewer on second login, got %q", carol.Role)
	}
}

func TestOIDC_RoleSyncAdmin_RemovedFromAdminGroupDemotesToEditor(t *testing.T) {
	provider := newFakeOIDCProvider(t, "leafwiki")
	w := createOIDCWikiTestInstance(t, provider, coreoidc.Config{RoleSync: coreoidc.RoleSyncAdmin})
	router := createRouterTestInstanceWithOIDC(w, t)

	doOIDCLogin(t, router, provider, fakeIDTokenClaims{
		Sub: "sub-carol", PreferredUsername: "carol", Groups: []string{"wiki-admins"},
	})
	carol, err := w.UserService().GetUserByUsername("carol")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if carol.Role != "admin" {
		t.Fatalf("expected admin role after first login, got %q", carol.Role)
	}

	doOIDCLogin(t, router, provider, fakeIDTokenClaims{
		Sub: "sub-carol", PreferredUsername: "carol", Groups: nil,
	})
	carol, err = w.UserService().GetUserByID(carol.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if carol.Role != "editor" {
		t.Fatalf("expected demotion to editor once out of the admin group, got %q", carol.Role)
	}
}

func TestOIDC_RoleSyncAdmin_EditorRoleSetInLeafWikiSurvivesRelogin(t *testing.T) {
	provider := newFakeOIDCProvider(t, "leafwiki")
	w := createOIDCWikiTestInstance(t, provider, coreoidc.Config{RoleSync: coreoidc.RoleSyncAdmin})
	router := createRouterTestInstanceWithOIDC(w, t)

	// First login creates the account as a viewer (not in any mapped group).
	doOIDCLogin(t, router, provider, fakeIDTokenClaims{
		Sub: "sub-erin", PreferredUsername: "erin", Groups: nil,
	})
	erin, err := w.UserService().GetUserByUsername("erin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if erin.Role != "viewer" {
		t.Fatalf("expected a new OIDC account with no matched group to start as viewer, got %q", erin.Role)
	}

	// An admin promotes erin to editor inside LeafWiki itself.
	if _, err := w.UserService().UpdateUser(erin.ID, erin.Username, erin.Email, "", "editor"); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	// Logging in again, still with no mapped group, must not undo that change.
	doOIDCLogin(t, router, provider, fakeIDTokenClaims{
		Sub: "sub-erin", PreferredUsername: "erin", Groups: nil,
	})
	erin, err = w.UserService().GetUserByID(erin.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if erin.Role != "editor" {
		t.Fatalf("expected the hand-set editor role to survive a relogin under RoleSyncAdmin, got %q", erin.Role)
	}
}
