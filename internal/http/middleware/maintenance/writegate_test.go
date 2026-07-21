package maintenance

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/restore"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestRouter(gate *restore.WriteGate) *gin.Engine {
	r := gin.New()
	r.Use(WriteGateMiddleware(gate))
	r.Any("/*path", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func TestWriteGateMiddleware_AllowsWritesWhenDisengaged(t *testing.T) {
	router := newTestRouter(restore.NewWriteGate())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/pages", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWriteGateMiddleware_BlocksWritesWhenEngaged(t *testing.T) {
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/pages", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestWriteGateMiddleware_ExemptsGetHeadOptionsEvenWhenEngaged(t *testing.T) {
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(method, "/api/pages", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200 even while engaged, got %d", method, w.Code)
		}
	}
}

func TestWriteGateMiddleware_ExemptsRestoreAdminPathEvenWhenEngaged(t *testing.T) {
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	for _, path := range []string{"/api/admin/restore/snapshot-1", "/api/admin/restore/self-restart"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, path, nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: expected the restore admin path to be exempt from the gate, got %d", path, w.Code)
		}
	}
}

func TestWriteGateMiddleware_StillBlocksOtherAdminPathsWhenEngaged(t *testing.T) {
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/admin/snapshot", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected non-restore admin writes to still be gated, got %d", w.Code)
	}
}

// TestWriteGateMiddleware_DoesNotMatchUnrelatedRouteWithSimilarName is the
// regression test for a real bug found in review: the exemption used to be a
// raw strings.Contains(path, "/api/admin/restore") check, which would also
// match an unrelated route whose name merely contains "restore" as a
// substring of a different segment (e.g. a hypothetical
// /api/admin/restore-policy), silently bypassing the gate for it. The fix
// matches whole path segments, so a distinct segment like "restore-policy"
// must not match a check for "restore".
func TestWriteGateMiddleware_DoesNotMatchUnrelatedRouteWithSimilarName(t *testing.T) {
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/admin/restore-policy", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected a route that merely contains \"restore\" as a substring (not the exact segment) to still be gated, got %d", w.Code)
	}
}

// TestWriteGateMiddleware_ExemptionSurvivesBasePathPrefix is the regression
// test for the second half of the same bug: with a raw substring check, an
// operator-configured --base-path that itself embeds "/api/admin/restore"
// (e.g. "/api/admin/restore-portal") would make *every* route in the app
// match the exemption, turning the gate into a permanent no-op. The
// segment-based fix must still correctly exempt only the real restore admin
// path even when preceded by an arbitrary base-path-like prefix, and must
// not exempt unrelated routes just because that prefix is present.
func TestWriteGateMiddleware_ExemptionSurvivesBasePathPrefix(t *testing.T) {
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/wiki/api/admin/restore/snapshot-1", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected the restore path to stay exempt behind a --base-path prefix, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/wiki/api/pages", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected an unrelated route behind a --base-path prefix to still be gated, got %d", w.Code)
	}
}

func TestWriteGateMiddleware_ExemptsSafeAuthEndpointsEvenWhenEngaged(t *testing.T) {
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	for _, path := range []string{"/api/auth/login", "/api/auth/login/totp", "/api/auth/refresh-token", "/api/auth/logout"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, path, nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: expected this auth endpoint to be exempt from the gate, got %d", path, w.Code)
		}
	}
}

func TestWriteGateMiddleware_StillBlocksTOTPMutationEndpointsWhenEngaged(t *testing.T) {
	// Unlike login/refresh/logout, these write to users.db itself and must
	// stay gated.
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	for _, path := range []string{"/api/users/me/totp/setup/start", "/api/users/me/totp/setup/confirm", "/api/users/me/totp/disable"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, path, nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: expected this TOTP-mutating endpoint to still be gated, got %d", path, w.Code)
		}
	}
}

func TestWriteGateMiddleware_RespondsWithLocalizedErrorShape(t *testing.T) {
	gate := restore.NewWriteGate()
	gate.Engage()
	router := newTestRouter(gate)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/pages", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{`"code":"restore_writes_disabled"`, `"message":`, `"template":`} {
		if !strings.Contains(body, want) {
			t.Errorf("expected response body to contain %s (LocalizedError shape), got %s", want, body)
		}
	}
}
