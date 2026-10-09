package revisions

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
)

// newRevisionAssetRouter records one revision for a page whose asset folder
// holds the given files and returns a router serving the revision-asset
// handler plus the page and revision IDs to request them under.
func newRevisionAssetRouter(t *testing.T, files map[string]string) (*gin.Engine, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	storageDir := t.TempDir()
	treeService := tree.NewTreeService(storageDir)
	if err := treeService.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	kind := tree.NodeKindPage
	pageID, err := treeService.CreateNode("system", nil, "Page", "page", &kind)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	assetDir := filepath.Join(storageDir, "assets", *pageID)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(assetDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) failed: %v", name, err)
		}
	}

	revService := revision.NewService(storageDir, treeService, nil, revision.ServiceOptions{})
	rev, _, err := revService.RecordAssetChange(*pageID, "system", "")
	if err != nil || rev == nil {
		t.Fatalf("RecordAssetChange failed: %#v %v", rev, err)
	}

	r := NewRoutes(RoutesConfig{GetRevisionAsset: NewGetRevisionAssetUseCase(revService)})
	engine := gin.New()
	engine.GET("/api/pages/:id/revisions/:revisionId/assets/*name", r.handleGetRevisionAsset)
	return engine, *pageID, rev.ID
}

func TestHandleGetRevisionAsset_ActiveContent_IsForcedDownload(t *testing.T) {
	const payload = `<script>alert(document.domain)</script>`
	engine, pageID, revID := newRevisionAssetRouter(t, map[string]string{
		"poc.html":  payload,
		"poc.HTML":  payload,
		"poc.xhtml": payload,
		"poc.xml":   payload,
		"x.svg":     `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"noext":     "<html>" + payload + "</html>",
	})

	for _, name := range []string{"poc.html", "poc.HTML", "poc.xhtml", "poc.xml", "x.svg", "noext"} {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pages/"+pageID+"/revisions/"+revID+"/assets/"+name, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", name, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", name, got)
		}
		if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
			t.Errorf("%s: Content-Disposition = %q, want attachment", name, got)
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != "sandbox" {
			t.Errorf("%s: Content-Security-Policy = %q, want sandbox", name, got)
		}
	}
}

func TestHandleGetRevisionAsset_SafeMedia_StaysInline(t *testing.T) {
	engine, pageID, revID := newRevisionAssetRouter(t, map[string]string{
		"doc.pdf":  "%PDF-1.4",
		"pic.png":  "\x89PNG",
		"vid.mp4":  "x",
		"song.mp3": "x",
	})

	for _, name := range []string{"doc.pdf", "pic.png", "vid.mp4", "song.mp3"} {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pages/"+pageID+"/revisions/"+revID+"/assets/"+name, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", name, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline") {
			t.Errorf("%s: Content-Disposition = %q, want inline", name, got)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", name, got)
		}
	}
}
