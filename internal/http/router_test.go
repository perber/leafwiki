package http

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func createWikiTestInstance(t *testing.T) *wiki.Wiki {
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	return w
}

func createRouterTestInstance(w *wiki.Wiki, t *testing.T) *gin.Engine {
	return NewRouter(w, RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,   // 15 minutes
		RefreshTokenTimeout:     7 * 24 * time.Hour, // 7 days
		HideLinkMetadataSection: false,
	})
}

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

	loginRes := loginRec.Result()
	defer loginRes.Body.Close()

	cookies := loginRes.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("Expected auth cookies on login response, got none")
	}

	csrfToken := loginRec.Header().Get("X-CSRF-Token")
	if csrfToken == "" {
		for _, c := range cookies {
			if c.Name == "leafwiki_csrf" || c.Name == "__Host-leafwiki_csrf" {
				csrfToken = c.Value
				break
			}
		}
	}

	if csrfToken == "" {
		t.Fatalf("Expected CSRF token after login, got none")
	}

	// Perform authenticated request
	if body == nil {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func authenticatedRequestAs(t *testing.T, router http.Handler, username, password, method, url string, body *strings.Reader) *httptest.ResponseRecorder {
	// Login with specific credentials
	loginBody := `{"identifier": "` + username + `", "password": "` + password + `"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Failed to login as %s: %d - %s", username, loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer loginRes.Body.Close()

	cookies := loginRes.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("Expected auth cookies on login response, got none")
	}

	csrfToken := loginRec.Header().Get("X-CSRF-Token")
	if csrfToken == "" {
		for _, c := range cookies {
			if c.Name == "leafwiki_csrf" || c.Name == "__Host-leafwiki_csrf" {
				csrfToken = c.Value
				break
			}
		}
	}

	if csrfToken == "" {
		t.Fatalf("Expected CSRF token after login, got none")
	}

	// Perform authenticated request
	if body == nil {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreatePageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `{"title": ""}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing title, got %d", rec.Code)
	}
}

func TestCreatePageEndpoint_InvalidJSON(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `this is not valid json`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid JSON, got %d", rec.Code)
	}
}

func TestCreatePageEndpoint_PageAlreadyExists(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/slug-suggestion", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	_, err := w.CreatePage(nil, "Delete Me", "delete-me")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := w.GetTree().Children[0]
	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+page.ID, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	if _, err := w.GetPage(page.ID); err == nil {
		t.Fatalf("Expected page to be deleted")
	}
}

func TestDeletePageEndpoint_NotFound(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/not-found-id", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint_HasChildren(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	parent, err := w.CreatePage(nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = w.CreatePage(&parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+parent.ID, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint_Recursive(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	parent, err := w.CreatePage(nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = w.CreatePage(&parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+parent.ID+"?recursive=true", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	if _, err := w.GetPage(parent.ID); err == nil {
		t.Fatalf("Expected page to be deleted")
	}
}

func TestUpdatePageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	_, err := w.CreatePage(nil, "Original Title", "original-title")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := w.GetTree().Children[0]

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `{"title": "Updated", "slug": "updated", "content": "New content"}`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/not-found-id", strings.NewReader(string(body)))
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown page, got %d", rec.Code)
	}
}

func TestUpdatePage_SlugRemainsIfUnchanged(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create a page
	created, err := w.CreatePage(nil, "Immutable Slug", "immutable-slug")
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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	_, err := w.CreatePage(nil, "Original Title", "original-title")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := w.GetTree().Children[0]

	_, err = w.CreatePage(nil, "Conflict Title", "conflict-title")
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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `this is not valid json`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/invalid-id", strings.NewReader(string(body)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestUpdatePage_MissingTitle(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `{"slug": "updated", "content": "New content"}`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-title", strings.NewReader(string(body)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing title, got %d", rec.Code)
	}
}

func TestUpdatePage_MissingSlug(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `{"title": "Updated", "content": "New content"}`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-slug", strings.NewReader(string(body)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing slug, got %d", rec.Code)
	}
}

func TestGetPageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create a page
	_, err := w.CreatePage(nil, "Welcome", "welcome")
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	page := w.GetTree().Children[0]

	// Get page
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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/not-found-id", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestGetPageEndpoint_MissingID(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create two pages a and b
	_, err := w.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	_, err = w.CreatePage(nil, "Section B", "section-b")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	a := w.GetTree().Children[0]
	b := w.GetTree().Children[1]

	// Move a under b
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"`+b.ID+`"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	// Check if a is now a child of b
	if len(b.Children) != 1 || b.Children[0].ID != a.ID {
		t.Errorf("Expected page to be moved under new parent")
	}
}

func TestMovePageEndpoint_NotFound(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/not-found-id/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_InvalidJSON(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/invalid-id/move", strings.NewReader(`this is not valid json`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_MissingParentID(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-parent/move", strings.NewReader(`{"parentId":""}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_ParentNotFound(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	_, err := w.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := w.GetTree().Children[0]

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"not-found-id"}`))

	t.Logf("Response: %s", rec.Body.String())
	t.Logf("Response Code: %d", rec.Code)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_CircularReference(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	_, err := w.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := w.GetTree().Children[0]

	_, err = w.CreatePage(&a.ID, "Section B", "section-b")
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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	_, err := w.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := w.GetTree().Children[0]

	_, err = w.CreatePage(nil, "Section B", "section-b")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Create Conflict Page in b
	conflictPage, err := w.CreatePage(&a.ID, "Section B", "section-b")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// move conflictPage under root (where section-b already exists)
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+conflictPage.ID+"/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePage_InTheSamePlace(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	_, err := w.CreatePage(nil, "Section A", "section-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	a := w.GetTree().Children[0]

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestSortPagesEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create pages
	page1, err := w.CreatePage(nil, "Page 1", "page-1")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page2, err := w.CreatePage(nil, "Page 2", "page-2")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page3, err := w.CreatePage(nil, "Page 3", "page-3")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Get Welcome Page by path
	welcomePage, err := w.FindByPath("welcome-to-leaf-wiki")
	if err != nil {
		t.Fatalf("FindByPath failed: %v", err)
	}

	// Delete Welcome Page
	err = w.DeletePage(welcomePage.ID, false)
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

	tree := w.GetTree()
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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `{"identifier": "admin", "password": "admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid login, got %d", rec.Code)
	}

	res := rec.Result()
	defer res.Body.Close()

	// Prüfen, ob Cookies gesetzt wurden
	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("Expected auth cookies to be set on login")
	}
}

func TestAuthLogin_InvalidCredentials(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// 1) Login
	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on login, got %d", loginRec.Code)
	}

	loginRes := loginRec.Result()
	defer loginRes.Body.Close()
	cookies := loginRes.Cookies()

	if len(cookies) == 0 {
		t.Fatalf("Expected auth cookies on login response, got none")
	}

	csrfToken := loginRec.Header().Get("X-CSRF-Token")
	if csrfToken == "" {
		for _, c := range cookies {
			if c.Name == "leafwiki_csrf" || c.Name == "__Host-leafwiki_csrf" {
				csrfToken = c.Value
				break
			}
		}
	}

	if csrfToken == "" {
		t.Fatalf("Expected CSRF token after login, got none")
	}

	// call refresh token endpoint with cookies from login
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh-token", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", csrfToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on refresh, got %d - %s", rec.Code, rec.Body.String())
	}

	// optional: check if new cookies are set
	refreshRes := rec.Result()
	defer refreshRes.Body.Close()
	newCookies := refreshRes.Cookies()
	if len(newCookies) == 0 {
		t.Fatalf("Expected new auth cookies on refresh")
	}
}

func TestCreateUserEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `{"username": "john", "email": "john@example.com", "password": "secret123", "role": "editor"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", rec.Code)
	}
}

func TestCreateUser_DuplicateEmailOrUsername(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create initial user
	payload := `{"username": "john", "email": "john@example.com", "password": "secret", "role": "editor"}`
	_ = authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(payload))

	// Attempt with duplicate username
	payloadDuplicate := `{"username": "john", "email": "john2@example.com", "password": "secret", "role": "editor"}`
	rec1 := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(payloadDuplicate))
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for duplicate username, got %d", rec1.Code)
	}

	// Attempt with duplicate email
	payloadDuplicateEmail := `{"username": "johnny", "email": "john@example.com", "password": "secret", "role": "editor"}`
	rec2 := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(payloadDuplicateEmail))
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for duplicate email, got %d", rec2.Code)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `{"username": "sam", "email": "sam@example.com", "password": "secret1234", "role": "undefined"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid role, got %d", rec.Code)
	}
}

func TestCreateUser_WithViewerRole(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	body := `{"username": "vieweruser", "email": "viewer@example.com", "password": "secret1234", "role": "viewer"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created for viewer role, got %d", rec.Code)
	}
}

func TestUpdateUser_RoleToViewer(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create user
	create := `{"username": "jane", "email": "jane@example.com", "password": "secretpassword", "role": "editor"}`
	resp := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(create))
	var user map[string]interface{}
	_ = json.Unmarshal(resp.Body.Bytes(), &user)

	updatePayload := map[string]string{
		"username": "jane-updated",
		"email":    "jane-updated@example.com",
		"password": "newpassword",
		"role":     "viewer",
	}
	data, _ := json.Marshal(updatePayload)
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/users/"+user["id"].(string), strings.NewReader(string(data)))

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for user update, got %d", rec.Code)
	}
}

func TestViewer_CannotCreatePage(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create a viewer user
	createUserBody := `{"username": "vieweruser", "email": "viewer@example.com", "password": "viewerpass", "role": "viewer"}`
	authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(createUserBody))

	// Try to create a page as viewer
	pageBody := `{"title": "Test Page", "slug": "test-page"}`
	rec := authenticatedRequestAs(t, router, "vieweruser", "viewerpass", http.MethodPost, "/api/pages", strings.NewReader(pageBody))

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for viewer creating page, got %d", rec.Code)
	}
}

func TestViewer_CannotUploadAsset(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create a viewer user
	createUserBody := `{"username": "vieweruser2", "email": "viewer2@example.com", "password": "viewerpass2", "role": "viewer"}`
	authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(createUserBody))

	// First create a page as admin to have a page ID
	pageBody := `{"title": "Test Page for Assets", "slug": "test-page-assets"}`
	pageResp := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(pageBody))
	var page map[string]interface{}
	_ = json.Unmarshal(pageResp.Body.Bytes(), &page)
	pageID := page["id"].(string)

	// Try to upload an asset as viewer
	rec := authenticatedRequestAs(t, router, "vieweruser2", "viewerpass2", http.MethodPost, "/api/pages/"+pageID+"/assets", strings.NewReader(""))

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for viewer uploading asset, got %d", rec.Code)
	}
}

func TestViewer_CannotUpdatePage(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create a viewer user
	createUserBody := `{"username": "vieweruser3", "email": "viewer3@example.com", "password": "viewerpass3", "role": "viewer"}`
	authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(createUserBody))

	// First create a page as admin
	pageBody := `{"title": "Test Page to Update", "slug": "test-page-update"}`
	pageResp := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(pageBody))
	var page map[string]interface{}
	_ = json.Unmarshal(pageResp.Body.Bytes(), &page)
	pageID := page["id"].(string)

	// Try to update the page as viewer
	updateBody := `{"title": "Updated Title", "slug": "updated-slug"}`
	rec := authenticatedRequestAs(t, router, "vieweruser3", "viewerpass3", http.MethodPut, "/api/pages/"+pageID, strings.NewReader(updateBody))

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for viewer updating page, got %d", rec.Code)
	}
}

func TestViewer_CannotDeletePage(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create a viewer user
	createUserBody := `{"username": "vieweruser4", "email": "viewer4@example.com", "password": "viewerpass4", "role": "viewer"}`
	authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(createUserBody))

	// First create a page as admin
	pageBody := `{"title": "Test Page to Delete", "slug": "test-page-delete"}`
	pageResp := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(pageBody))
	var page map[string]interface{}
	_ = json.Unmarshal(pageResp.Body.Bytes(), &page)
	pageID := page["id"].(string)

	// Try to delete the page as viewer
	rec := authenticatedRequestAs(t, router, "vieweruser4", "viewerpass4", http.MethodDelete, "/api/pages/"+pageID, nil)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for viewer deleting page, got %d", rec.Code)
	}
}

func TestGetUsersEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create user
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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Create user
	create := `{"username": "todelete", "email": "delete@example.com", "password": "secrepassword", "role": "editor"}`
	resp := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(create))
	var user map[string]interface{}
	_ = json.Unmarshal(resp.Body.Bytes(), &user)

	// Delete user
	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/users/"+user["id"].(string), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("Expected 204 OK on delete, got %d", rec.Code)
	}
}

func TestDeleteAdminUser_ShouldFail(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Get default admin
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

	// Attempt to delete the admin
	recDel := authenticatedRequest(t, router, http.MethodDelete, "/api/users/"+adminID, nil)
	if recDel.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 when deleting admin user, got %d", recDel.Code)
	}
}

func TestRequireAdminMiddleware(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Default Admin create user should succeed
	body := `{"username": "mod", "email": "mod@example.com", "password": "secretpassword", "role": "editor"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created by admin, got %d", rec.Code)
	}
}

func TestRequireAuthMiddleware_Unauthorized(t *testing.T) {
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

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
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Step 0: Login als Admin und Cookies holen
	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()

	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on login, got %d - %s", loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer loginRes.Body.Close()

	cookies := loginRes.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("Expected auth cookies after login, got none")
	}

	csrfToken := loginRec.Header().Get("X-CSRF-Token")
	if csrfToken == "" {
		for _, c := range cookies {
			if c.Name == "leafwiki_csrf" || c.Name == "__Host-leafwiki_csrf" {
				csrfToken = c.Value
				break
			}
		}
	}

	if csrfToken == "" {
		t.Fatalf("Expected CSRF token after login, got none")
	}

	addCookies := func(req *http.Request) {
		for _, c := range cookies {
			req.AddCookie(c)
		}

		if req.Method != http.MethodGet && req.Method != http.MethodHead && req.Method != http.MethodOptions {
			req.Header.Set("X-CSRF-Token", csrfToken)
		}
	}

	// Step 1: Create page direkt über Wiki-API
	page, err := w.CreatePage(nil, "Assets Page", "assets-page")
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Step 2: Upload file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "testfile.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := part.Write([]byte("Hello, asset!")); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/pages/"+page.ID+"/assets", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	addCookies(uploadReq)

	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created on upload, got %d - %s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploadResp map[string]string
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("Invalid upload JSON: %v", err)
	}
	if uploadResp["file"] == "" {
		t.Error("Expected file field in upload response")
	}

	// Step 3: List assets
	listReq := httptest.NewRequest(http.MethodGet, "/api/pages/"+page.ID+"/assets", nil)
	addCookies(listReq)

	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on listing, got %d - %s", listRec.Code, listRec.Body.String())
	}

	var listResp map[string][]string
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("Invalid listing JSON: %v", err)
	}
	if len(listResp["files"]) != 1 || listResp["files"][0] != "/assets/"+page.ID+"/testfile.txt" {
		t.Errorf("Expected file in listing, got: %v", listResp["files"])
	}

	// Step 4: Delete asset
	delReq := httptest.NewRequest(http.MethodDelete, "/api/pages/"+page.ID+"/assets/testfile.txt", nil)
	addCookies(delReq)

	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK on delete, got %d - %s", delRec.Code, delRec.Body.String())
	}

	// Step 5: Verify asset is gone
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/pages/"+page.ID+"/assets", nil)
	addCookies(listReq2)

	listRec2 := httptest.NewRecorder()
	router.ServeHTTP(listRec2, listReq2)

	if listRec2.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on listing after delete, got %d - %s", listRec2.Code, listRec2.Body.String())
	}

	var listResp2 map[string][]string
	if err := json.Unmarshal(listRec2.Body.Bytes(), &listResp2); err != nil {
		t.Fatalf("Invalid listing JSON: %v", err)
	}
	if len(listResp2["files"]) != 0 {
		t.Errorf("Expected asset to be deleted, got: %v", listResp2["files"])
	}
}

// Lets check the indexing status
func TestIndexingStatusEndpoint(t *testing.T) {
	// Lets call /api/search/status
	w := createWikiTestInstance(t)
	defer w.Close()
	router := createRouterTestInstance(w, t)

	// Default Admin holen
	rec := authenticatedRequest(t, router, http.MethodGet, "/api/search/status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	var status map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if status["active"] == nil {
		t.Errorf("Expected 'active' field in response, got: %v", status)
	}
}
