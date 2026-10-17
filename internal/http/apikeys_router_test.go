package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
	wikibackup "github.com/perber/wiki/internal/wiki/backup"
	wikirestore "github.com/perber/wiki/internal/wiki/restore"
	wikisnapshot "github.com/perber/wiki/internal/wiki/snapshot"
)

// newAPIKeyRouterTest builds a Wiki + router pair mirroring createRouterTestInstance,
// but additionally wires the wiki's APIKeyService so Bearer API-key auth is active.
//
// It cannot use createWikiTestInstance: that helper leaves EnableAPIKeyManagement
// unset, so the wiki never constructs an APIKeyService (see wiki.go's initAuth) and
// every key operation hits the nil-service ErrAPIKeysDisabled guard.
func newAPIKeyRouterTest(t *testing.T) (*wiki.Wiki, http.Handler) {
	t.Helper()
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:             t.TempDir(),
		AdminPassword:          "adminpassword",
		JWTSecret:              "secretkey",
		AccessTokenTimeout:     15 * time.Minute,
		RefreshTokenTimeout:    7 * 24 * time.Hour,
		EnableRevision:         true,
		EnableAPIKeyManagement: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	// Nil managers: the routes answer 503 when reached, which is enough to test
	// the guards in front of them.
	w.SetSnapshotRoutes(wikisnapshot.NewRoutes(nil, nil, w.AuthService(), 0))
	w.SetRestoreRoutes(wikirestore.NewRoutes(nil, w.AuthService()))
	w.SetBackupRoutes(wikibackup.NewRoutes(nil, w.AuthService()))
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) {
		o.APIKeyService = w.APIKeyService()
		o.EnableAPIKeyManagement = true
	})
	return w, router
}

func bearerRequest(method, url, token string) *http.Request {
	return bearerJSONRequest(method, url, token, "")
}

func bearerJSONRequest(method, url, token, body string) *http.Request {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// TestAPIKeys_AdminCanCreateListAndRevoke exercises the full admin-facing
// management surface, and confirms the list response never leaks the secret
// or its hash.
func TestAPIKeys_AdminCanCreateListAndRevoke(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)

	owner, err := w.UserService().CreateUser("agent-owner", "agent-owner@example.com", "password123", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser err: %v", err)
	}

	createBody := `{"name":"agent key","userId":"` + owner.ID + `","role":"viewer"}`
	createRec := authenticatedRequest(t, router, http.MethodPost, "/api/api-keys", strings.NewReader(createBody))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating key, got %d: %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		Key struct {
			ID     string `json:"id"`
			Prefix string `json:"prefix"`
		} `json:"key"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if created.Secret == "" || !strings.HasPrefix(created.Secret, "lw_") {
		t.Fatalf("expected a lw_-prefixed secret, got %q", created.Secret)
	}
	if created.Key.ID == "" {
		t.Fatalf("expected a key id in the response")
	}

	listRec := authenticatedRequest(t, router, http.MethodGet, "/api/api-keys", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 listing keys, got %d: %s", listRec.Code, listRec.Body.String())
	}
	if strings.Contains(strings.ToLower(listRec.Body.String()), "hash") || strings.Contains(listRec.Body.String(), created.Secret) {
		t.Fatalf("list response must never leak the key hash or secret, got: %s", listRec.Body.String())
	}

	revokeRec := authenticatedRequest(t, router, http.MethodDelete, "/api/api-keys/"+created.Key.ID, nil)
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 revoking key, got %d: %s", revokeRec.Code, revokeRec.Body.String())
	}
}

// TestAPIKeys_NonAdminCannotManageKeys verifies the management surface is
// admin-only, matching every other admin-gated feature in the app.
func TestAPIKeys_NonAdminCannotManageKeys(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)

	owner, err := w.UserService().CreateUser("plain-editor", "plain-editor@example.com", "password123", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser err: %v", err)
	}

	createBody := `{"name":"k","userId":"` + owner.ID + `"}`
	rec := authenticatedRequestAs(t, router, "plain-editor", "password123", http.MethodPost, "/api/api-keys", strings.NewReader(createBody))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin creating a key, got %d: %s", rec.Code, rec.Body.String())
	}

	listRec := authenticatedRequestAs(t, router, "plain-editor", "password123", http.MethodGet, "/api/api-keys", nil)
	if listRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin listing keys, got %d: %s", listRec.Code, listRec.Body.String())
	}
}

// TestAPIKeys_AdminScopedKeyCannotManageKeys is the regression test for the
// code-review finding that an admin-scoped API key could list, create and
// revoke every key in the system via Bearer auth. CSRF no longer stops key
// writes, so RequireCookieSession is what closes this: even an admin-scoped
// key must be rejected from /api/api-keys, regardless of role.
func TestAPIKeys_AdminScopedKeyCannotManageKeys(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)

	owner, err := w.UserService().CreateUser("admin-agent-owner", "admin-agent-owner@example.com", "password123", coreauth.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser err: %v", err)
	}

	createBody := `{"name":"admin key","userId":"` + owner.ID + `","role":"admin"}`
	createRec := authenticatedRequest(t, router, http.MethodPost, "/api/api-keys", strings.NewReader(createBody))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating admin-scoped key, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, bearerRequest(http.MethodGet, "/api/api-keys", created.Secret))
	if listRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 listing keys with an admin-scoped Bearer key, got %d: %s", listRec.Code, listRec.Body.String())
	}
}

// TestAPIKeys_ViewerScopedKeyCanReadButNotWrite is the end-to-end proof of the
// permission model: a viewer-scoped key can reach a read endpoint, but is
// rejected on a mutating one by the role guard (a resolved key skips CSRF).
func TestAPIKeys_ViewerScopedKeyCanReadButNotWrite(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)

	owner, err := w.UserService().CreateUser("agent-owner", "agent-owner@example.com", "password123", coreauth.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser err: %v", err)
	}

	createBody := `{"name":"agent key","userId":"` + owner.ID + `","role":"viewer"}`
	createRec := authenticatedRequest(t, router, http.MethodPost, "/api/api-keys", strings.NewReader(createBody))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating key, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	readRec := httptest.NewRecorder()
	router.ServeHTTP(readRec, bearerRequest(http.MethodGet, "/api/tree", created.Secret))
	if readRec.Code != http.StatusOK {
		t.Fatalf("expected 200 reading tree with viewer key, got %d: %s", readRec.Code, readRec.Body.String())
	}

	writeReq := bearerRequest(http.MethodPost, "/api/pages", created.Secret)
	writeReq.Header.Set("Content-Type", "application/json")
	writeRec := httptest.NewRecorder()
	router.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 writing with a bearer key, got %d: %s", writeRec.Code, writeRec.Body.String())
	}
}

// TestAPIKeys_RevokedKeyIsRejectedImmediately verifies revocation takes effect
// on the very next request — no caching, no grace period.
func TestAPIKeys_RevokedKeyIsRejectedImmediately(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)

	owner, err := w.UserService().CreateUser("agent-owner", "agent-owner@example.com", "password123", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser err: %v", err)
	}

	createBody := `{"name":"agent key","userId":"` + owner.ID + `"}`
	createRec := authenticatedRequest(t, router, http.MethodPost, "/api/api-keys", strings.NewReader(createBody))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating key, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Key struct {
			ID string `json:"id"`
		} `json:"key"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	preRevokeRec := httptest.NewRecorder()
	router.ServeHTTP(preRevokeRec, bearerRequest(http.MethodGet, "/api/tree", created.Secret))
	if preRevokeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 before revocation, got %d: %s", preRevokeRec.Code, preRevokeRec.Body.String())
	}

	revokeRec := authenticatedRequest(t, router, http.MethodDelete, "/api/api-keys/"+created.Key.ID, nil)
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 revoking key, got %d: %s", revokeRec.Code, revokeRec.Body.String())
	}

	postRevokeRec := httptest.NewRecorder()
	router.ServeHTTP(postRevokeRec, bearerRequest(http.MethodGet, "/api/tree", created.Secret))
	if postRevokeRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after revocation, got %d: %s", postRevokeRec.Code, postRevokeRec.Body.String())
	}
}

// createAPIKeyFor creates a key for a fresh user with userRole, scoped to
// keyRole, via the admin session, and returns the Bearer secret.
func createAPIKeyFor(t *testing.T, w *wiki.Wiki, router http.Handler, username, userRole, keyRole string) string {
	t.Helper()
	owner, err := w.UserService().CreateUser(username, username+"@example.com", "password123", userRole)
	if err != nil {
		t.Fatalf("CreateUser err: %v", err)
	}
	body := `{"name":"` + username + ` key","userId":"` + owner.ID + `","role":"` + keyRole + `"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/api-keys", strings.NewReader(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating key, got %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	return created.Secret
}

// TestAPIKeys_EditorKeyCanWritePages proves a resolved key passes CSRF and
// then follows the normal role guards: an editor key can create a page with
// nothing but the Authorization header.
func TestAPIKeys_EditorKeyCanWritePages(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)
	secret := createAPIKeyFor(t, w, router, "editor-agent", coreauth.RoleEditor, coreauth.RoleEditor)

	req := bearerJSONRequest(http.MethodPost, "/api/pages", secret, `{"title":"From key","slug":"from-key"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating a page with an editor key, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAPIKeys_ViewerKeyOwnedByEditorCannotWrite checks role(user) ∩ role(key):
// the owner could write, but the viewer-scoped key must not.
func TestAPIKeys_ViewerKeyOwnedByEditorCannotWrite(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)
	secret := createAPIKeyFor(t, w, router, "editor-owner", coreauth.RoleEditor, coreauth.RoleViewer)

	req := bearerJSONRequest(http.MethodPost, "/api/pages", secret, `{"title":"Nope","slug":"nope"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 writing with a viewer-scoped key, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Editor or Admin role required") {
		t.Fatalf("expected the role guard to reject the write, got: %s", rec.Body.String())
	}
}

// TestAPIKeys_SessionWithoutCSRFTokenStillRejected guards against the CSRF
// skip leaking to cookie sessions: only a resolved key may skip the check.
func TestAPIKeys_SessionWithoutCSRFTokenStillRejected(t *testing.T) {
	_, router := newAPIKeyRouterTest(t)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"identifier": "admin", "password": "adminpassword"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d - %s", loginRec.Code, loginRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(`{"title":"X","slug":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range loginRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "CSRF") {
		t.Fatalf("expected a CSRF 403 for a cookie session without token, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAPIKeys_CredentialRoutesAreSessionOnly ensures an admin-scoped key
// cannot touch routes that manage credentials, sessions or users.
func TestAPIKeys_CredentialRoutesAreSessionOnly(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)
	secret := createAPIKeyFor(t, w, router, "admin-agent", coreauth.RoleAdmin, coreauth.RoleAdmin)

	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodPut, "/api/users/me/password"},
		{http.MethodPost, "/api/users/me/totp/setup/start"},
		{http.MethodPost, "/api/users/me/totp/setup/confirm"},
		{http.MethodPost, "/api/users/me/totp/disable"},
		{http.MethodPost, "/api/users"},
		{http.MethodPut, "/api/users/some-id"},
		{http.MethodDelete, "/api/users/some-id"},
		{http.MethodPost, "/api/users/invite"},
		{http.MethodPost, "/api/users/some-id/invite/resend"},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := bearerRequest(rt.method, rt.path, secret)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "requires a browser session") {
				t.Fatalf("expected the session-only guard to reject the request, got: %s", rec.Body.String())
			}
		})
	}
}

// TestAPIKeys_InstanceRecoveryRoutesAreSessionOnly covers the admin routes that
// can replace or exfiltrate the whole instance: restore overwrites users.db and
// api_keys.db, a snapshot ZIP contains both, and the backup config routes can
// point the stored git credentials at another server. An admin-scoped key must
// not reach them, while harmless status routes stay reachable.
func TestAPIKeys_InstanceRecoveryRoutesAreSessionOnly(t *testing.T) {
	w, router := newAPIKeyRouterTest(t)
	secret := createAPIKeyFor(t, w, router, "recovery-agent", coreauth.RoleAdmin, coreauth.RoleAdmin)

	blocked := []struct{ method, path string }{
		{http.MethodPost, "/api/admin/restore/upload"},
		{http.MethodPost, "/api/admin/restore/some-id"},
		{http.MethodPost, "/api/admin/restore/self-restart"},
		{http.MethodGet, "/api/admin/snapshot/some-id/download"},
		// Config routes accept a caller-chosen remoteUrl and fill blank secrets
		// from the stored config, so a key could make the server send the stored
		// git credentials to a server of its choosing.
		{http.MethodPost, "/api/admin/backup/config"},
		{http.MethodPost, "/api/admin/backup/config/test"},
		{http.MethodPost, "/api/admin/backup/disable"},
	}
	for _, rt := range blocked {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, bearerRequest(rt.method, rt.path, secret))

			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "requires a browser session") {
				t.Fatalf("expected the session-only guard to reject the key, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, bearerRequest(http.MethodGet, "/api/admin/snapshot/status", secret))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status routes to stay reachable with an admin key, got %d: %s", rec.Code, rec.Body.String())
	}
}
