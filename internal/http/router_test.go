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

func TestCreatePageEndpoint_MissingTitle(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"title": ""}`

	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing title, got %d", rec.Code)
	}
}

func TestCreatePageEndpoint_InvalidJSON(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `this is not valid json`

	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid JSON, got %d", rec.Code)
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

func TestSuggestSlugEndpoint_ExistingPage(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "NewPage")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

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

	if resp["slug"] != "newpage-1" {
		t.Errorf("Expected 'newpage-1' as slug suggestion, got %v", resp)
	}
}

func TestSuggestSlugEndpoint_MissingTitle(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodGet, "/api/pages/slug-suggestion", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Delete Me")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := wikiInstance.GetTree().Children[0]

	req := httptest.NewRequest(http.MethodDelete, "/api/pages/"+page.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	if _, err := wikiInstance.GetPage(page.ID); err == nil {
		t.Fatalf("Expected page to be deleted")
	}
}

func TestDeletePageEndpoint_NotFound(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodDelete, "/api/pages/not-found-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint_HasChildren(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	parent, err := wikiInstance.CreatePage(nil, "Parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = wikiInstance.CreatePage(&parent.ID, "Child")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/pages/"+parent.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint_Recursive(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	parent, err := wikiInstance.CreatePage(nil, "Parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = wikiInstance.CreatePage(&parent.ID, "Child")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/pages/"+parent.ID+"?recursive=true", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	if _, err := wikiInstance.GetPage(parent.ID); err == nil {
		t.Fatalf("Expected page to be deleted")
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

func TestUpdatePage_InvalidJSON(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `this is not valid json`
	req := httptest.NewRequest(http.MethodPut, "/api/pages/invalid-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestUpdatePage_MissingTitle(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"slug": "updated", "content": "New content"}`
	req := httptest.NewRequest(http.MethodPut, "/api/pages/missing-title", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing title, got %d", rec.Code)
	}
}

func TestUpdatePage_MissingSlug(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"title": "Updated", "content": "New content"}`
	req := httptest.NewRequest(http.MethodPut, "/api/pages/missing-slug", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing slug, got %d", rec.Code)
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

func TestGetPageEndpoint_NotFound(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodGet, "/api/pages/not-found-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestGetPageEndpoint_MissingID(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodGet, "/api/pages/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
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

func TestMovePageEndpoint_NotFound(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodPost, "/api/pages/not-found-id/move", strings.NewReader(`{"parentId":"root"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_InvalidJSON(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodPost, "/api/pages/invalid-id/move", strings.NewReader(`this is not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_MissingParentID(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodPost, "/api/pages/missing-parent/move", strings.NewReader(`{"parentId":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_ParentNotFound(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Section A")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := wikiInstance.GetTree().Children[0]

	req := httptest.NewRequest(http.MethodPost, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"not-found-id"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}
