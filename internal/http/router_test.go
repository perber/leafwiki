package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/wiki"
)

func TestCreatePageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	title := "Getting Started"
	expectedSlug := "getting-started"

	body := `{"title": "Getting Started"}`

	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	if resp["id"] == nil {
		t.Errorf("Expected id in response, got: %v", resp)
	}

	if resp["title"] != title {
		t.Errorf("Expected title in response, got: %v", resp)
	}

	if resp["slug"] != expectedSlug {
		t.Errorf("Expected slug in response, got: %v", resp)
	}
}

func TestGetTreeEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodGet, "/api/tree", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	if _, ok := resp["id"]; !ok {
		t.Errorf("Expected root node in response")
	}

	if resp["title"] != "root" {
		t.Errorf("Expected root node title to be 'Root', got: %v", resp)
	}

	if resp["slug"] != "root" {
		t.Errorf("Expected root node slug to be 'root', got: %v", resp)
	}

	if resp["id"] != "root" {
		t.Errorf("Expected root node id to be 'root', got: %v", resp)
	}
}

func TestSuggestSlugEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodGet, "/api/pages/slug-suggestion?title=NewPage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	if resp["slug"] == "" {
		t.Errorf("Expected a slug suggestion, got: %v", resp)
	}

	if resp["slug"] != "newpage" {
		t.Errorf("Expected 'newpage' as slug suggestion, got: %v", resp)
	}
}

func TestUpdatePageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Original Title")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := wikiInstance.GetTree().Children[0]

	payload := map[string]string{
		"title":   "Updated Title",
		"slug":    "updated-title",
		"content": "# Updated Content\nWith **Markdown** support.",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/pages/"+page.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	if resp["title"] != "Updated Title" {
		t.Errorf("Expected updated title, got %q", resp["title"])
	}
	if resp["slug"] != "updated-title" {
		t.Errorf("Expected updated slug, got %q", resp["slug"])
	}
	if resp["content"] != "# Updated Content\nWith **Markdown** support." {
		t.Errorf("Expected updated content, got %q", resp["content"])
	}
}

func TestUpdatePage_NotFound(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"title": "Updated", "slug": "updated", "content": "New content"}`
	req := httptest.NewRequest(http.MethodPut, "/api/pages/not-found-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// TODO: Should return a 404
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown page, got %d", rec.Code)
	}
}

func TestGetPageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Erstellt eine Page über Wiki (nicht direkt über HTTP)
	_, err := wikiInstance.CreatePage(nil, "Welcome")
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	page := wikiInstance.GetTree().Children[0]

	// Page abrufen
	req := httptest.NewRequest(http.MethodGet, "/api/pages/"+page.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if resp["id"] == nil {
		t.Errorf("Expected id in response, got: %v", resp)
	}

	if resp["title"] != "Welcome" {
		t.Errorf("Expected title in response, got: %v", resp)
	}

	if resp["slug"] != "welcome" {
		t.Errorf("Expected slug in response, got: %v", resp)
	}
}

func TestMovePageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Erstelle zwei Pages: root → a, root → b
	_, err := wikiInstance.CreatePage(nil, "Section A")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = wikiInstance.CreatePage(nil, "Section B")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	a := wikiInstance.GetTree().Children[0]
	b := wikiInstance.GetTree().Children[1]

	// Verschiebe a → unter b
	req := httptest.NewRequest(http.MethodPost, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"`+b.ID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	// Checke ob a jetzt Kind von b ist
	if len(b.Children) != 1 || b.Children[0].ID != a.ID {
		t.Errorf("Expected page to be moved under new parent")
	}
}
