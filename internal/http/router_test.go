package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/wiki"
)

func authenticatedRequest(t *testing.T, router http.Handler, method, url string, body *strings.Reader) *httptest.ResponseRecorder {
	// Login
	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Failed to login: %d - %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp map[string]string
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Invalid login response: %v", err)
	}
	token := loginResp["token"]

	// Perform authenticated request
	if body == nil {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreatePageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	title := "Getting Started"
	expectedSlug := "getting-started"

	body := `{"title": "Getting Started", "slug": "getting-started"}`

	rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

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
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing title, got %d", rec.Code)
	}
}

func TestCreatePageEndpoint_InvalidJSON(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `this is not valid json`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid JSON, got %d", rec.Code)
	}
}

func TestCreatePageEndpoint_PageAlreadyExists(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"title": "Page Exists", "slug": "page-exists"}`
	rec1 := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

	if rec1.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", rec1.Code)
	}

	rec2 := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec2.Code)
	}
}

func TestGetTreeEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)
	rec := authenticatedRequest(t, router, http.MethodGet, "/api/tree", nil)

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

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/slug-suggestion?title=NewPage", nil)

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

func TestSuggestSlugEndpoint_MissingTitle(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)
	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/slug-suggestion", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Delete Me", "delete-me")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := wikiInstance.GetTree().Children[0]
	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+page.ID, nil)

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
	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/not-found-id", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint_HasChildren(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	parent, err := wikiInstance.CreatePage(nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = wikiInstance.CreatePage(&parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+parent.ID, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint_Recursive(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	parent, err := wikiInstance.CreatePage(nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = wikiInstance.CreatePage(&parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+parent.ID+"?recursive=true", nil)
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

	_, err := wikiInstance.CreatePage(nil, "Original Title", "original-title")
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

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+page.ID, strings.NewReader(string(body)))

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
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/not-found-id", strings.NewReader(string(body)))
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown page, got %d", rec.Code)
	}
}

func TestUpdatePage_SlugRemainsIfUnchanged(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Create a page
	created, err := wikiInstance.CreatePage(nil, "Immutable Slug", "immutable-slug")
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Update title, but reuse slug
	payload := map[string]string{
		"title":   "Updated Title",
		"slug":    created.Slug,
		"content": "Updated content",
	}
	body, _ := json.Marshal(payload)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+created.ID, strings.NewReader(string(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var updated map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Invalid response JSON: %v", err)
	}

	if updated["slug"] != created.Slug {
		t.Errorf("Expected slug to remain unchanged, got: %v", updated["slug"])
	}
}

func TestUpdatePage_PageAlreadyExists(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Original Title", "original-title")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := wikiInstance.GetTree().Children[0]

	_, err = wikiInstance.CreatePage(nil, "Conflict Title", "conflict-title")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	payload := map[string]string{
		"title":   "Conflict Title",
		"slug":    "conflict-title",
		"content": "Updated content",
	}
	body, _ := json.Marshal(payload)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+page.ID, strings.NewReader(string(body)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdatePage_InvalidJSON(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `this is not valid json`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/invalid-id", strings.NewReader(string(body)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestUpdatePage_MissingTitle(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"slug": "updated", "content": "New content"}`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-title", strings.NewReader(string(body)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing title, got %d", rec.Code)
	}
}

func TestUpdatePage_MissingSlug(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"title": "Updated", "content": "New content"}`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-slug", strings.NewReader(string(body)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing slug, got %d", rec.Code)
	}
}

func TestGetPageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Erstellt eine Page über Wiki (nicht direkt über HTTP)
	_, err := wikiInstance.CreatePage(nil, "Welcome", "welcome")
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	page := wikiInstance.GetTree().Children[0]

	// Page abrufen
	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+page.ID, nil)

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

	if resp["title"] != "Welcome to Leaf Wiki" {
		t.Errorf("Expected title in response, got: %v", resp)
	}

	if resp["slug"] != "welcome-to-leaf-wiki" {
		t.Errorf("Expected slug in response, got: %v", resp)
	}
}

func TestGetPageEndpoint_NotFound(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/not-found-id", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestGetPageEndpoint_MissingID(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Erstelle zwei Pages: root → a, root → b
	_, err := wikiInstance.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = wikiInstance.CreatePage(nil, "Section B", "section-b")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	a := wikiInstance.GetTree().Children[0]
	b := wikiInstance.GetTree().Children[1]

	// Verschiebe a → unter b
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"`+b.ID+`"}`))

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

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/not-found-id/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_InvalidJSON(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/invalid-id/move", strings.NewReader(`this is not valid json`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_MissingParentID(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-parent/move", strings.NewReader(`{"parentId":""}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_ParentNotFound(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := wikiInstance.GetTree().Children[0]

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"not-found-id"}`))

	t.Logf("Response: %s", rec.Body.String())
	t.Logf("Response Code: %d", rec.Code)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_CircularReference(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := wikiInstance.GetTree().Children[0]

	_, err = wikiInstance.CreatePage(&a.ID, "Section B", "section-b")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	b := a.Children[0]

	// Verschiebe a → unter b
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+b.ID+"/move", strings.NewReader(`{"parentId":"`+a.ID+`"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePage_FailsIfTargetAlreadyHasPageWithSameSlug(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := wikiInstance.GetTree().Children[0]

	_, err = wikiInstance.CreatePage(nil, "Section B", "section-b")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Create Conflict Page in b
	conflictPage, err := wikiInstance.CreatePage(&a.ID, "Section B", "section-b")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Verschibe ConflictPage in root level
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+conflictPage.ID+"/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePage_InTheSamePlace(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	_, err := wikiInstance.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := wikiInstance.GetTree().Children[0]

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestSortPagesEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Create pages
	page1, err := wikiInstance.CreatePage(nil, "Page 1", "page-1")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page2, err := wikiInstance.CreatePage(nil, "Page 2", "page-2")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page3, err := wikiInstance.CreatePage(nil, "Page 3", "page-3")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Get Welcome Page by path
	welcomePage, err := wikiInstance.FindByPath("welcome-to-leaf-wiki")
	if err != nil {
		t.Fatalf("FindByPath failed: %v", err)
	}

	// Delete Welcome Page
	err = wikiInstance.DeletePage(welcomePage.ID, false)
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	// Sort pages
	payload := map[string]interface{}{
		"orderedIds": []string{page3.ID, page1.ID, page2.ID},
	}
	body, _ := json.Marshal(payload)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/root/sort", strings.NewReader(string(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if resp["message"] != "Pages sorted successfully" {
		t.Errorf("Expected success message, got: %v", resp["message"])
	}

	tree := wikiInstance.GetTree()
	if len(tree.Children) != 3 {
		t.Fatalf("Expected 3 children in root, got: %d", len(tree.Children))
	}

	if tree.Children[0].ID != page3.ID {
		t.Errorf("Expected first child to be page 3, got: %v", tree.Children[0].ID)
	}
	if tree.Children[1].ID != page1.ID {
		t.Errorf("Expected second child to be page 1, got: %v", tree.Children[1].ID)
	}
	if tree.Children[2].ID != page2.ID {
		t.Errorf("Expected third child to be page 2, got: %v", tree.Children[2].ID)
	}
}
