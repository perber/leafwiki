package maintenance

import (
	"net/http"
	"net/http/httptest"
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
