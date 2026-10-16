package contentfiles

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	httpinternal "github.com/perber/wiki/internal/http"
)

func newTestRouter(t *testing.T, rootDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	ctx := httpinternal.RouterContext{
		Engine: engine,
		Base:   &engine.RouterGroup,
		Opts:   httpinternal.RouterOptions{AuthDisabled: true},
	}
	NewRoutes(RoutesConfig{RootDir: rootDir, Log: slog.Default()}).RegisterRoutes(ctx)
	return engine
}

func TestContentFilesServesImageFromContentTree(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "guildselect", "product", "user-journeys", "img")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run2-06-reset-1-signin.png"), []byte("fake-png"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := newTestRouter(t, root)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/files/guildselect/product/user-journeys/img/run2-06-reset-1-signin.png", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("expected image/png content type, got %q", got)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected untrusted-file hardening headers")
	}
	if w.Body.String() != "fake-png" {
		t.Errorf("unexpected body: %q", w.Body.String())
	}
}

func TestContentFilesRejections(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "page.md"), []byte("# secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "img.png"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		url  string
	}{
		{"markdown pages are never served as files", "/files/page.md"},
		{"unknown extension", "/files/notes.txt"},
		{"missing file", "/files/nope.png"},
		{"directory root", "/files/"},
		{"traversal stays inside root (resolves to missing)", "/files/../../etc/passwd.png"},
	}
	engine := newTestRouter(t, root)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, c.url, nil)
			engine.ServeHTTP(w, req)
			if w.Code != http.StatusNotFound {
				t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestContentFilesNoRootDirNoRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	ctx := httpinternal.RouterContext{
		Engine: engine,
		Base:   &engine.RouterGroup,
		Opts:   httpinternal.RouterOptions{AuthDisabled: true},
	}
	NewRoutes(RoutesConfig{RootDir: "", Log: slog.Default()}).RegisterRoutes(ctx)
	if len(engine.Routes()) != 0 {
		t.Errorf("expected no routes without a root dir, got %d", len(engine.Routes()))
	}
}
