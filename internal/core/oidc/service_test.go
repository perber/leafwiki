package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newFakeProvider spins up an httptest.Server serving a minimal OIDC
// discovery document at <issuer>/.well-known/openid-configuration (issuer
// path ends with a trailing slash, matching the real Authentik convention
// this is guarding against — see Config.Issuer's doc comment). Returns the
// issuer URL to pass to NewService.
func newFakeProvider(t *testing.T, endSessionEndpoint string) (issuer string, cleanup func()) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	issuerURL := server.URL + "/application/o/leafwiki/"

	mux.HandleFunc("/application/o/leafwiki/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 issuerURL,
			"authorization_endpoint": server.URL + "/application/o/leafwiki/authorize/",
			"token_endpoint":         server.URL + "/application/o/leafwiki/token/",
			"jwks_uri":               server.URL + "/application/o/leafwiki/jwks/",
			"end_session_endpoint":   endSessionEndpoint,
		})
	})

	return issuerURL, server.Close
}

func TestNewService_HandlesTrailingSlashIssuerWithoutDoublingIt(t *testing.T) {
	issuer, cleanup := newFakeProvider(t, "")
	defer cleanup()

	svc, err := NewService(context.Background(), Config{
		Issuer:       issuer,
		ClientID:     "leafwiki",
		ClientSecret: "secret",
		RedirectURL:  "https://wiki.example.com/login/oidc/callback",
	})
	if err != nil {
		t.Fatalf("NewService with a trailing-slash issuer: %v", err)
	}
	if svc == nil {
		t.Fatal("expected a non-nil service")
	}
}

func TestNewService_ExtractsEndSessionEndpointFromDiscovery(t *testing.T) {
	want := "https://auth.example.com/application/o/leafwiki/end-session/"
	issuer, cleanup := newFakeProvider(t, want)
	defer cleanup()

	svc, err := NewService(context.Background(), Config{
		Issuer:       issuer,
		ClientID:     "leafwiki",
		ClientSecret: "secret",
		RedirectURL:  "https://wiki.example.com/login/oidc/callback",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if svc.EndSessionURL != want {
		t.Fatalf("EndSessionURL = %q, want %q", svc.EndSessionURL, want)
	}
}

func TestNewService_NoEndSessionEndpoint_LeavesItEmpty(t *testing.T) {
	issuer, cleanup := newFakeProvider(t, "")
	defer cleanup()

	svc, err := NewService(context.Background(), Config{
		Issuer:       issuer,
		ClientID:     "leafwiki",
		ClientSecret: "secret",
		RedirectURL:  "https://wiki.example.com/login/oidc/callback",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if svc.EndSessionURL != "" {
		t.Fatalf("expected empty EndSessionURL, got %q", svc.EndSessionURL)
	}
}

func TestNewService_UnreachableIssuer_ReturnsError(t *testing.T) {
	_, err := NewService(context.Background(), Config{
		Issuer:       "http://127.0.0.1:1/does-not-exist/",
		ClientID:     "leafwiki",
		ClientSecret: "secret",
		RedirectURL:  "https://wiki.example.com/login/oidc/callback",
	})
	if err == nil {
		t.Fatal("expected an error for an unreachable issuer")
	}
}

func TestService_AuthCodeURL_IncludesStatePKCEAndNonce(t *testing.T) {
	issuer, cleanup := newFakeProvider(t, "")
	defer cleanup()

	svc, err := NewService(context.Background(), Config{
		Issuer:       issuer,
		ClientID:     "leafwiki",
		ClientSecret: "secret",
		RedirectURL:  "https://wiki.example.com/login/oidc/callback",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	url := svc.AuthCodeURL("the-state", "the-nonce", "the-verifier")
	for _, want := range []string{"state=the-state", "nonce=the-nonce", "code_challenge=", "code_challenge_method=S256"} {
		if !strings.Contains(url, want) {
			t.Fatalf("AuthCodeURL() = %q, expected it to contain %q", url, want)
		}
	}
}
