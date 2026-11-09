package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
)

const (
	invalidCredentialsBody = `{"error":{"code":"auth_invalid_credentials","message":"Invalid credentials","template":"invalid credentials"}}`

	// freeLoginAttempts is the number of password attempts per (account,
	// client) that carry no wait (core/auth loginFreeAttempts).
	freeLoginAttempts = 5

	// freeCodeAttempts is the number of code attempts per account before the
	// code step locks (core/auth totpMaxAttempts).
	freeCodeAttempts = 5
)

type loginWireResponse struct {
	Status      int
	ContentType string
	RetryAfter  string
	Body        string // compacted JSON
	Headers     string // every response header, sorted, for comparing answers with each other
}

// postJSONFrom sends one JSON request from a TCP peer; headers come in name,
// value pairs.
func postJSONFrom(router http.Handler, path, remoteAddr string, payload map[string]string, headers ...string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func loginWire(t *testing.T, router http.Handler, remoteAddr, identifier, password string, headers ...string) loginWireResponse {
	t.Helper()
	rec := postJSONFrom(router, "/api/auth/login", remoteAddr,
		map[string]string{"identifier": identifier, "password": password}, headers...)

	var compact bytes.Buffer
	if err := json.Compact(&compact, rec.Body.Bytes()); err != nil {
		t.Fatalf("login response is not JSON: %v (%q)", err, rec.Body.String())
	}
	return loginWireResponse{
		Status:      rec.Code,
		ContentType: rec.Header().Get("Content-Type"),
		RetryAfter:  rec.Header().Get("Retry-After"),
		Body:        compact.String(),
		Headers:     sortedHeaders(rec.Header()),
	}
}

func sortedHeaders(h http.Header) string {
	lines := make([]string, 0, len(h))
	for name, values := range h {
		lines = append(lines, name+": "+strings.Join(values, ", "))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// A throttled attempt must not be told apart from a wrong identifier or
// password: a distinct answer would confirm that the account exists to anyone
// who can reach the login form. Each of the other requests comes from its own
// client address so the per-client rate limit never interferes.
func TestLogin_ThrottledAttemptRespondsLikeInvalidCredentials(t *testing.T) {
	w := createWikiTestInstance(t) // seeds admin "admin" / "adminpassword"
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	unknownUser := loginWire(t, router, "198.51.100.1:40000", "no-such-user", "whatever")
	wrongPassword := loginWire(t, router, "198.51.100.2:40000", "admin", "wrong-guess")
	// The free attempts of one client are used up; the correct password is
	// refused during the wait that follows.
	for i := 0; i < freeLoginAttempts; i++ {
		loginWire(t, router, "203.0.113.7:51000", "admin", "wrong-guess")
	}
	throttled := loginWire(t, router, "203.0.113.7:51000", "admin", "adminpassword")

	want := loginWireResponse{
		Status:      http.StatusUnauthorized,
		ContentType: "application/json; charset=utf-8",
		Body:        invalidCredentialsBody,
	}
	for name, got := range map[string]loginWireResponse{
		"unknown user":      unknownUser,
		"wrong password":    wrongPassword,
		"throttled attempt": throttled,
	} {
		// Headers are compared against the wrong-password answer rather than a
		// fixed list, so any header that differs (a cookie, a Retry-After)
		// shows up without pinning middleware details here.
		if got.Headers != wrongPassword.Headers {
			t.Errorf("%s headers:\n got  %s\n want %s", name, got.Headers, wrongPassword.Headers)
		}
		got.Headers = ""
		if got != want {
			t.Errorf("%s:\n got  %+v\n want %+v", name, got, want)
		}
	}
}

// The throttle is keyed on the client address the router resolves, so these
// scenarios fail if the trusted-proxy configuration is missing: with gin's
// default of trusting every peer, a forwarding header would pick the client.
func TestLogin_ThrottleFollowsTheResolvedClientAddress(t *testing.T) {
	tests := []struct {
		name    string
		trusted string // --trusted-proxy-ips
		// failing gives the peer address and extra headers of the i-th request
		// of the client that runs out of free attempts.
		failing func(i int) (peer string, headers []string)
		// The peer address and headers of a different client, which must still
		// get in.
		otherPeer    string
		otherHeaders []string
	}{
		{
			name: "forwarding headers from a direct client are ignored",
			failing: func(i int) (string, []string) {
				// One peer that claims a new address with every request.
				return "203.0.113.7:51000", []string{
					"X-Forwarded-For", fmt.Sprintf("192.0.2.%d", i+1),
					"X-Real-IP", fmt.Sprintf("198.18.0.%d", i+1),
				}
			},
			otherPeer: "203.0.113.8:51000",
		},
		{
			name:    "a listed proxy tells two clients apart",
			trusted: "10.0.0.0/8",
			failing: func(i int) (string, []string) {
				// A different left-hand entry does not make a different client.
				return "10.0.0.1:5000", []string{"X-Forwarded-For", fmt.Sprintf("192.0.2.%d, 203.0.113.7", i+1)}
			},
			otherPeer:    "10.0.0.1:5000",
			otherHeaders: []string{"X-Forwarded-For", "203.0.113.8"},
		},
		{
			name: "addresses of one IPv6 /64 are one client",
			failing: func(i int) (string, []string) {
				return fmt.Sprintf("[2001:db8:1:2::%x]:51000", i+1), nil
			},
			otherPeer: "[2001:db8:1:3::1]:51000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := createWikiTestInstance(t)
			defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
			trusted, err := authmw.ParseTrustedProxies(tt.trusted)
			if err != nil {
				t.Fatalf("ParseTrustedProxies(%q): %v", tt.trusted, err)
			}
			router := newTestRouter(w, func(o *httpinternal.RouterOptions) { o.TrustedProxies = trusted })

			for i := 0; i < freeLoginAttempts; i++ {
				peer, headers := tt.failing(i)
				if got := loginWire(t, router, peer, "admin", "wrong-guess", headers...); got.Status != http.StatusUnauthorized {
					t.Fatalf("wrong password %d: status %d, want 401", i+1, got.Status)
				}
			}

			peer, headers := tt.failing(freeLoginAttempts)
			if got := loginWire(t, router, peer, "admin", "adminpassword", headers...); got.Status != http.StatusUnauthorized || got.Body != invalidCredentialsBody {
				t.Errorf("the client past its free attempts got %d %s, want the invalid-credentials answer", got.Status, got.Body)
			}

			if got := loginWire(t, router, tt.otherPeer, "admin", "adminpassword", tt.otherHeaders...); got.Status != http.StatusOK {
				t.Errorf("another client was held back too: %d %s", got.Status, got.Body)
			}
		})
	}
}

// newTOTPLoginWiki returns a wiki whose "admin" account has TOTP enabled, and
// the secret that generates its codes.
func newTOTPLoginWiki(t *testing.T) (*wiki.Wiki, string) {
	t.Helper()
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		TOTPEncryptionKey:   "01234567890123456789012345678901",
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })

	admin, err := w.UserService().GetUserByIdentifier("admin")
	if err != nil {
		t.Fatalf("GetUserByIdentifier: %v", err)
	}
	generated, err := w.AuthService().StartTOTPSetup(admin.ID, "adminpassword")
	if err != nil {
		t.Fatalf("StartTOTPSetup: %v", err)
	}
	code, err := totp.GenerateCode(generated.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := w.AuthService().ConfirmTOTPSetup(admin.ID, code, ""); err != nil {
		t.Fatalf("ConfirmTOTPSetup: %v", err)
	}
	return w, generated.Secret
}

// The code step is locked per account, from every address together. A login
// with the right password answers 200 and clears the per-host login limiter,
// so interleaving it with wrong codes never reaches the limiter's ten: only
// the account's own lock can stop the guessing.
func TestLoginTOTP_WrongCodesAreLockedAcrossAddresses(t *testing.T) {
	w, secret := newTOTPLoginWiki(t)
	router := createRouterTestInstance(w, t)

	var challenge string
	for i := 0; i < freeCodeAttempts; i++ {
		peer := fmt.Sprintf("192.0.2.%d:40000", i+1)

		rec := postJSONFrom(router, "/api/auth/login", peer, map[string]string{"identifier": "admin", "password": "adminpassword"})
		var step1 struct {
			RequiresTOTP        bool   `json:"requiresTotp"`
			LoginChallengeToken string `json:"loginChallengeToken"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &step1); err != nil || rec.Code != http.StatusOK || !step1.RequiresTOTP {
			t.Fatalf("password step %d: %d %s", i+1, rec.Code, rec.Body.String())
		}
		challenge = step1.LoginChallengeToken

		rec = postJSONFrom(router, "/api/auth/login/totp", peer, map[string]string{"loginChallengeToken": challenge, "code": "000000"})
		if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "auth_invalid_credentials") {
			t.Fatalf("wrong code %d: expected the invalid-code answer, got %d %s", i+1, rec.Code, rec.Body.String())
		}
	}

	// A valid code from yet another address is refused like invalid credentials.
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	rec := postJSONFrom(router, "/api/auth/login/totp", "198.51.100.77:40000", map[string]string{"loginChallengeToken": challenge, "code": code})
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusUnauthorized || got != invalidCredentialsBody {
		t.Fatalf("expected the code lock to answer 401 %s, got %d %s", invalidCredentialsBody, rec.Code, got)
	}
}
