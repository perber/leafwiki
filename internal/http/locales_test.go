package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveLocalesDir_DefaultsNextToDataDir(t *testing.T) {
	t.Parallel()

	got := ResolveLocalesDir("/srv/wiki/data", "")
	want := filepath.Join("/srv/wiki", "locales")
	if got != want {
		t.Fatalf("ResolveLocalesDir() = %q, want %q", got, want)
	}
}

func TestRegisterLocalesRoutes_ServesNamespaceAndListsLanguages(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	localesDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(localesDir, "en"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localesDir, "languages.json"), []byte(`{"en":"English","ru":"Русский"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localesDir, "en", "common.json"), []byte(`{"hello":"Hello"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(localesDir, "ru"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localesDir, "ru", "common.json"), []byte(`{"hello":"Привет"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := gin.New()
	RegisterLocalesRoutes(engine.Group(""), localesDir)

	req := httptest.NewRequest(http.MethodGet, "/locales/en/common.json", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /locales/en/common.json status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/locales/../secret/common.json", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("path traversal status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/locales", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/locales status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload localesListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Languages) != 2 {
		t.Fatalf("languages count = %d, want 2", len(payload.Languages))
	}
}
