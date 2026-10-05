package assets

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newAssetServeRouter(t *testing.T, files map[string]string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	engine := gin.New()
	g := engine.Group("/assets")
	g.Use(assetServeHeaders())
	g.StaticFS("/", gin.Dir(dir, false))
	return engine
}

func getAsset(engine *gin.Engine, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestAssetServeHeaders_ActiveContent_IsForcedDownload(t *testing.T) {
	const payload = `<script>alert(document.domain)</script>`
	engine := newAssetServeRouter(t, map[string]string{
		"p1/poc.html":  payload,
		"p1/poc.HTML":  payload,
		"p1/poc.xhtml": payload,
		"p1/poc.xml":   payload,
		"p1/x.svg":     `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"p1/noext":     "<html>" + payload + "</html>", // Go sniffs this as text/html
	})

	for _, p := range []string{"poc.html", "poc.HTML", "poc.xhtml", "poc.xml", "x.svg", "noext"} {
		rec := getAsset(engine, "/assets/p1/"+p)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", p, rec.Code)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", p, got)
		}
		if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
			t.Errorf("%s: Content-Disposition = %q, want attachment", p, got)
		}
	}
}

func TestAssetServeHeaders_SafeMedia_StaysInline(t *testing.T) {
	engine := newAssetServeRouter(t, map[string]string{
		"p1/doc.pdf":  "%PDF-1.4",
		"p1/pic.png":  "\x89PNG",
		"p1/pic.JPG":  "x",
		"p1/vid.mp4":  "x",
		"p1/song.mp3": "x",
	})

	for _, p := range []string{"doc.pdf", "pic.png", "pic.JPG", "vid.mp4", "song.mp3"} {
		rec := getAsset(engine, "/assets/p1/"+p)
		if got := rec.Header().Get("Content-Disposition"); got != "" {
			t.Errorf("%s: Content-Disposition = %q, want none (inline)", p, got)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", p, got)
		}
	}
}
