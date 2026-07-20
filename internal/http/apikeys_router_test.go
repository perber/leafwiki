package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/assets"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
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
		AdminPassword:          "admin",
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
	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		APIKeyService:           w.APIKeyService(),
		EnableAPIKeyManagement:  true,
	})
	return w, router
}

func bearerRequest(method, url, token string) *http.Request {
	req := httptest.NewRequest(method, url, strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer "+token)
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
// code-review finding that an admin-scoped API key could list (and, absent
// CSRF's incidental protection, create/revoke) every key in the system via
// Bearer auth. RequireCookieSession now closes this explicitly: even an
// admin-scoped key must be rejected from /api/api-keys, regardless of role.
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
// rejected on a mutating one. The write attempt is rejected for CSRF reasons
// before role is even evaluated (a pure Bearer client has no CSRF cookie) —
// which is a stricter outcome than the role check alone would give, and it
// means no key, regardless of role, can reach a write route in this phase.
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
