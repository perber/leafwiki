package http

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
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

	var loginResp map[string]interface{}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Invalid login response: %v", err)
	}
	token := loginResp["token"].(string)

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

func TestAuthLoginEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"identifier": "admin", "password": "admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid login, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if _, ok := resp["token"]; !ok {
		t.Errorf("Expected token in response, got: %v", resp)
	}
}

func TestAuthLogin_InvalidCredentials(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"identifier": "admin", "password": "wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for wrong credentials, got %d", rec.Code)
	}
}

func TestAuthRefreshToken(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Login
	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	var loginResp map[string]string
	_ = json.Unmarshal(loginRec.Body.Bytes(), &loginResp)
	originalToken := loginResp["token"]
	refreshToken := loginResp["refresh_token"]

	// Refresh
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh-token", strings.NewReader(`{"token":"`+refreshToken+`"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on refresh, got %d", rec.Code)
	}

	var refreshResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &refreshResp)

	if refreshResp["token"] == originalToken {
		t.Errorf("Expected new token, got same one")
	}
}

func TestCreateUserEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"username": "john", "email": "john@example.com", "password": "secret", "role": "editor"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", rec.Code)
	}
}

func TestCreateUser_DuplicateEmailOrUsername(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Erstellt initialen Benutzer
	payload := `{"username": "john", "email": "john@example.com", "password": "secret", "role": "editor"}`
	_ = authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(payload))

	// Versuch mit gleichem Benutzernamen
	payloadDuplicate := `{"username": "john", "email": "john2@example.com", "password": "secret", "role": "editor"}`
	rec1 := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(payloadDuplicate))
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for duplicate username, got %d", rec1.Code)
	}

	// Versuch mit gleicher Email
	payloadDuplicateEmail := `{"username": "johnny", "email": "john@example.com", "password": "secret", "role": "editor"}`
	rec2 := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(payloadDuplicateEmail))
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for duplicate email, got %d", rec2.Code)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	body := `{"username": "sam", "email": "sam@example.com", "password": "secret", "role": "viewer"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid role, got %d", rec.Code)
	}
}

func TestGetUsersEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/users", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	var users []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(users) == 0 {
		t.Errorf("Expected at least one user (admin), got none")
	}
}

func TestUpdateUserEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Benutzer anlegen
	create := `{"username": "jane", "email": "jane@example.com", "password": "secretpassword", "role": "editor"}`
	resp := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(create))
	var user map[string]interface{}
	_ = json.Unmarshal(resp.Body.Bytes(), &user)

	updatePayload := map[string]string{
		"username": "jane-updated",
		"email":    "jane-updated@example.com",
		"password": "newpassword",
		"role":     "editor",
	}
	data, _ := json.Marshal(updatePayload)
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/users/"+user["id"].(string), strings.NewReader(string(data)))

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for user update, got %d", rec.Code)
	}

}

func TestDeleteUserEndpoint(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Benutzer anlegen
	create := `{"username": "todelete", "email": "delete@example.com", "password": "secrepassword", "role": "editor"}`
	resp := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(create))
	var user map[string]interface{}
	_ = json.Unmarshal(resp.Body.Bytes(), &user)

	// Benutzer löschen
	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/users/"+user["id"].(string), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("Expected 204 OK on delete, got %d", rec.Code)
	}
}

func TestDeleteAdminUser_ShouldFail(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Default Admin holen
	rec := authenticatedRequest(t, router, http.MethodGet, "/api/users", nil)
	var users []map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &users)

	var adminID string
	for _, u := range users {
		if u["role"] == "admin" {
			adminID = u["id"].(string)
		}
	}

	if adminID == "" {
		t.Fatal("No admin user found")
	}

	// Versuch den Admin zu löschen
	recDel := authenticatedRequest(t, router, http.MethodDelete, "/api/users/"+adminID, nil)
	if recDel.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 when deleting admin user, got %d", recDel.Code)
	}
}

func TestRequireAdminMiddleware(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Default Admin versucht, Benutzer zu erstellen (sollte erlaubt sein)
	body := `{"username": "mod", "email": "mod@example.com", "password": "secretpassword", "role": "editor"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created by admin, got %d", rec.Code)
	}
}

func TestRequireAuthMiddleware_Unauthorized(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Request ohne Token
	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(`{"title": "Oops", "slug": "oops"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestRequireAuthMiddleware_InvalidToken(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(`{"title": "Bad", "slug": "bad"}`))
	req.Header.Set("Authorization", "Bearer invalidtoken")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for invalid token, got %d", rec.Code)
	}
}

func TestAssetEndpoints(t *testing.T) {
	wikiInstance, _ := wiki.NewWiki(t.TempDir())
	router := NewRouter(wikiInstance)

	// Step 1: Create page
	page, err := wikiInstance.CreatePage(nil, "Assets Page", "assets-page")
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Step 2: Upload file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("file", "testfile.txt")
	part.Write([]byte("Hello, asset!"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/pages/"+page.ID+"/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Auth
	login := authenticatedRequest(t, router, http.MethodPost, "/api/auth/login", strings.NewReader(`{"identifier": "admin", "password": "admin"}`))
	var loginResp map[string]string
	json.Unmarshal(login.Body.Bytes(), &loginResp)
	token := loginResp["token"]
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created on upload, got %d - %s", rec.Code, rec.Body.String())
	}

	var uploadResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("Invalid upload JSON: %v", err)
	}
	if uploadResp["file"] == "" {
		t.Error("Expected File in upload response")
	}

	// Step 3: List assets
	listRec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+page.ID+"/assets", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on listing, got %d", listRec.Code)
	}
	var listResp map[string][]string
	json.Unmarshal(listRec.Body.Bytes(), &listResp)
	if len(listResp["files"]) != 1 || listResp["files"][0] != "/assets/root/assets-page/assets/testfile.txt" {
		t.Errorf("Expected file in listing, got: %v", listResp["files"])
	}

	// Step 4: Delete asset
	delRec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+page.ID+"/assets/testfile.txt", nil)
	if delRec.Code != http.StatusOK {
		t.Errorf("Expected 200 No Content on delete, got %d", delRec.Code)
	}

	// Step 5: Verify asset is gone
	listRec2 := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+page.ID+"/assets", nil)
	var listResp2 map[string][]string
	json.Unmarshal(listRec2.Body.Bytes(), &listResp2)
	if len(listResp2["files"]) != 0 {
		t.Errorf("Expected asset to be deleted, got: %v", listResp2["files"])
	}
}
