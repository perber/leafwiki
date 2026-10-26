package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/search"
)

func newHealthRouter(t *testing.T, setup func(*search.IndexingStatus), missingDataDir bool) http.Handler {
	t.Helper()

	dir := t.TempDir()
	index, err := search.NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	t.Cleanup(func() { _ = index.Close() })

	status := search.NewIndexingStatus()
	setup(status)

	storageDir := dir
	if missingDataDir {
		storageDir = filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-missing")
	}

	routes := NewRoutes(RoutesConfig{Index: index, Status: status, StorageDir: storageDir})
	return httpinternal.NewRouter(
		[]httpinternal.RouteRegistrar{routes},
		httpinternal.FrontendConfig{},
		httpinternal.RouterOptions{DisableRequestLog: true},
	)
}

func TestHealthRoutes_GetHealth_PreservesWireShape(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*search.IndexingStatus)
		missingData   bool
		wantCode      int
		wantStatus    string
		wantSearch    string
		wantDataCheck string
	}{
		{"ready", func(s *search.IndexingStatus) { s.Finish() }, false, http.StatusOK, "ok", "ok", "ok"},
		{"indexing", func(s *search.IndexingStatus) { s.Start() }, false, http.StatusOK, "ok", "indexing", "ok"},
		{"failed", func(s *search.IndexingStatus) { s.Start(); s.Fail(); s.Finish() }, false, http.StatusServiceUnavailable, "degraded", "failed", "ok"},
		{"missing", func(s *search.IndexingStatus) { s.Finish() }, true, http.StatusServiceUnavailable, "degraded", "ok", "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newHealthRouter(t, tt.setup, tt.missingData)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

			if rec.Code != tt.wantCode {
				t.Fatalf("status code = %d, want %d; body=%s", rec.Code, tt.wantCode, rec.Body.String())
			}

			var body map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if len(body) != 2 || body["status"] == nil || body["checks"] == nil {
				t.Fatalf("body keys = %v, want exactly status and checks", keys(body))
			}

			var status string
			if err := json.Unmarshal(body["status"], &status); err != nil {
				t.Fatalf("status is not a string: %v", err)
			}
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}

			var rawChecks map[string]json.RawMessage
			if err := json.Unmarshal(body["checks"], &rawChecks); err != nil {
				t.Fatalf("checks is not an object: %v", err)
			}
			if len(rawChecks) != 3 {
				t.Fatalf("checks keys = %v, want exactly sqlite, data_dir, search", keys(rawChecks))
			}
			checks := map[string]string{}
			for _, k := range []string{"sqlite", "data_dir", "search"} {
				raw, ok := rawChecks[k]
				if !ok {
					t.Fatalf("checks missing %q: %v", k, keys(rawChecks))
				}
				var v string
				if err := json.Unmarshal(raw, &v); err != nil {
					t.Fatalf("checks.%s is not a string: %v", k, err)
				}
				checks[k] = v
			}

			if checks["sqlite"] != "ok" {
				t.Errorf("checks.sqlite = %q, want ok", checks["sqlite"])
			}
			if checks["data_dir"] != tt.wantDataCheck {
				t.Errorf("checks.data_dir = %q, want %q", checks["data_dir"], tt.wantDataCheck)
			}
			if checks["search"] != tt.wantSearch {
				t.Errorf("checks.search = %q, want %q", checks["search"], tt.wantSearch)
			}
		})
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
