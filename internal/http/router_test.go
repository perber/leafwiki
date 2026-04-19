package http_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
)

func pageNodeKind() *tree.NodeKind {
	kind := tree.NodeKindPage
	return &kind
}

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
	return createRouterTestInstanceWithMaxAssetUploadSize(w, t, assets.DefaultMaxUploadSizeBytes)
}

func createRouterTestInstanceWithRevision(w *wiki.Wiki, t *testing.T) *gin.Engine {
	return httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		CustomStylesheet:        "",
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		EnableRevision:          true,
	})
}

func createRouterTestInstanceWithMaxAssetUploadSize(w *wiki.Wiki, t *testing.T, maxAssetUploadSizeBytes int64) *gin.Engine {
	return httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		CustomStylesheet:        "",
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,   // 15 minutes
		RefreshTokenTimeout:     7 * 24 * time.Hour, // 7 days
		HideLinkMetadataSection: false,
		MaxAssetUploadSizeBytes: maxAssetUploadSizeBytes,
	})
}

func createRouterTestInstanceWithAllowInsecure(w *wiki.Wiki, allowInsecure bool, t *testing.T) *gin.Engine {
	return httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		CustomStylesheet:        "",
		AllowInsecure:           allowInsecure,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
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
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

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
	loginData := map[string]string{
		"identifier": username,
		"password":   password,
	}
	loginBodyBytes, _ := json.Marshal(loginData)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBodyBytes))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Failed to login as %s: %d - %s", username, loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

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
	var reqBody io.Reader
	if body != nil {
		reqBody = body
	}
	req := httptest.NewRequest(method, url, reqBody)
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

type apiPage struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Slug     string        `json:"slug"`
	Content  string        `json:"content"`
	Path     string        `json:"path"`
	Kind     tree.NodeKind `json:"kind"`
	Children []*apiPage    `json:"children"`
}

func createPageViaAPI(t *testing.T, router http.Handler, title, slug string, parentID *string, kind *tree.NodeKind) *apiPage {
	t.Helper()

	payload := map[string]any{
		"title": title,
		"slug":  slug,
	}
	if parentID != nil {
		payload["parentId"] = *parentID
	}
	if kind != nil {
		payload["kind"] = string(*kind)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal(create page payload) failed: %v", err)
	}

	rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(string(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d - %s", rec.Code, rec.Body.String())
	}

	var page apiPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("Unmarshal(create page response) failed: %v", err)
	}

	return &page
}

func getPageByPathViaAPI(t *testing.T, router http.Handler, path string) *apiPage {
	t.Helper()

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/by-path?path="+path, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d - %s", rec.Code, rec.Body.String())
	}

	var page apiPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("Unmarshal(get page by path response) failed: %v", err)
	}

	return &page
}

func getTreeViaAPI(t *testing.T, router http.Handler) *apiPage {
	t.Helper()

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/tree", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d - %s", rec.Code, rec.Body.String())
	}

	var node apiPage
	if err := json.Unmarshal(rec.Body.Bytes(), &node); err != nil {
		t.Fatalf("Unmarshal(tree response) failed: %v", err)
	}

	return &node
}

func deletePageViaAPI(t *testing.T, router http.Handler, pageID string, recursive bool) {
	t.Helper()

	url := "/api/pages/" + pageID
	if recursive {
		url += "?recursive=true"
	}

	rec := authenticatedRequest(t, router, http.MethodDelete, url, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d - %s", rec.Code, rec.Body.String())
	}
}

func listAssetsViaAPI(t *testing.T, router http.Handler, pageID string) []string {
	t.Helper()

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+pageID+"/assets", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d - %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal(list assets response) failed: %v", err)
	}

	return resp.Files
}

func uploadAssetViaAPI(t *testing.T, router http.Handler, pageID, filename, content string) string {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("Write(asset payload) failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(writer) failed: %v", err)
	}

	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on login, got %d - %s", loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

	cookies := loginRes.Cookies()
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
		t.Fatal("Expected CSRF token after login, got none")
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/pages/"+pageID+"/assets", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		uploadReq.AddCookie(cookie)
	}

	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created on upload, got %d - %s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploadResp map[string]string
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("Unmarshal(upload asset response) failed: %v", err)
	}

	return uploadResp["file"]
}

func getLatestRevisionViaAPI(t *testing.T, router http.Handler, pageID string) map[string]any {
	t.Helper()

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+pageID+"/revisions/latest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d - %s", rec.Code, rec.Body.String())
	}

	var rev map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rev); err != nil {
		t.Fatalf("Unmarshal(latest revision response) failed: %v", err)
	}
	return rev
}

func getAdminUserIDViaAPI(t *testing.T, router http.Handler) string {
	t.Helper()

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/users", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d - %s", rec.Code, rec.Body.String())
	}

	var users []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("Unmarshal(users response) failed: %v", err)
	}
	for _, user := range users {
		if role, _ := user["role"].(string); role == "admin" {
			if id, _ := user["id"].(string); id != "" {
				return id
			}
		}
	}
	t.Fatal("admin user not found")
	return ""
}

func uploadBrandingLogoViaAPI(t *testing.T, router http.Handler, filename string, content []byte) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Write(logo payload) failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(writer) failed: %v", err)
	}

	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on login, got %d - %s", loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

	cookies := loginRes.Cookies()
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
		t.Fatal("Expected CSRF token after login, got none")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/branding/logo", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d - %s", rec.Code, rec.Body.String())
	}
}

func importerFixturePathForHTTPTests(t *testing.T, rel string) string {
	t.Helper()

	return test_utils.FixturePath(t, rel, "../importer/fixtures", "internal/importer/fixtures")
}

func createZipFromDir(t *testing.T, root string) []byte {
	t.Helper()

	var body bytes.Buffer
	zipWriter := zip.NewWriter(&body)

	err := filepath.Walk(root, func(sourcePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(root, sourcePath)
		if err != nil {
			return err
		}

		entry, err := zipWriter.Create(filepath.ToSlash(relativePath))
		if err != nil {
			return err
		}

		raw, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		_, err = entry.Write(raw)
		return err
	})
	if err != nil {
		t.Fatalf("create zip from dir: %v", err)
	}

	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return body.Bytes()
}

func TestCreatePageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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

func TestConfigEndpoint_ExplainsAllowInsecureRequirementOnHTTP(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstanceWithAllowInsecure(w, false, t)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "--allow-insecure") {
		t.Fatalf("expected response to explain allow-insecure requirement, got %s", rec.Body.String())
	}
}

func TestLoginEndpoint_ExplainsAllowInsecureRequirementOnHTTP(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstanceWithAllowInsecure(w, false, t)

	loginBody := `{"identifier": "admin", "password": "admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "--allow-insecure") {
		t.Fatalf("expected response to explain allow-insecure requirement, got %s", rec.Body.String())
	}
}

func TestCreatePageEndpoint_MissingTitle(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `{"title": ""}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing title, got %d", rec.Code)
	}
}

func TestCreatePageEndpoint_InvalidJSON(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `this is not valid json`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid JSON, got %d", rec.Code)
	}
}

func TestCreatePageEndpoint_PageAlreadyExists(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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

func TestConfigEndpoint_IncludesMaxAssetUploadSizeBytes(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	const maxAssetUploadSizeBytes int64 = 123456
	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            true,
		InjectCodeInHeader:      "",
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		MaxAssetUploadSizeBytes: maxAssetUploadSizeBytes,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	gotSize, ok := resp["maxAssetUploadSizeBytes"].(float64)
	if !ok {
		t.Fatalf("Expected maxAssetUploadSizeBytes in config response, got %v", resp)
	}

	if int64(gotSize) != maxAssetUploadSizeBytes {
		t.Fatalf("Expected maxAssetUploadSizeBytes=%d, got %v", maxAssetUploadSizeBytes, gotSize)
	}
}

func TestConfigEndpoint_IncludesEnableLinkRefactor(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            true,
		InjectCodeInHeader:      "",
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		EnableLinkRefactor:      true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	gotEnabled, ok := resp["enableLinkRefactor"].(bool)
	if !ok {
		t.Fatalf("Expected enableLinkRefactor in config response, got %v", resp)
	}

	if !gotEnabled {
		t.Fatalf("Expected enableLinkRefactor=true, got %v", gotEnabled)
	}
}

func TestRefactorPreviewEndpoint_UsesFrontendJSONShape(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	router := createRouterTestInstance(w, t)
	target := createPageViaAPI(t, router, "Target", "target", nil, pageNodeKind())
	ref := createPageViaAPI(t, router, "Ref", "ref", nil, pageNodeKind())

	updateBody := strings.NewReader(`{"title":"Ref","slug":"ref","content":"[Target](/target)"}`)
	updateRec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+ref.ID, updateBody)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on page update, got %d - %s", updateRec.Code, updateRec.Body.String())
	}

	previewBody := strings.NewReader(`{"kind":"rename","title":"Target","slug":"target-renamed"}`)
	previewRec := authenticatedRequest(t, router, http.MethodPost, "/api/pages/"+target.ID+"/refactor/preview", previewBody)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on refactor preview, got %d - %s", previewRec.Code, previewRec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(previewRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Invalid refactor preview JSON: %v", err)
	}

	if _, ok := resp["counts"]; !ok {
		t.Fatalf("Expected lowercase counts in response, got %v", resp)
	}
	if _, ok := resp["affectedPages"]; !ok {
		t.Fatalf("Expected lowercase affectedPages in response, got %v", resp)
	}
	if _, ok := resp["Counts"]; ok {
		t.Fatalf("Did not expect legacy Counts key in response, got %v", resp)
	}
	if _, ok := resp["AffectedPages"]; ok {
		t.Fatalf("Did not expect legacy AffectedPages key in response, got %v", resp)
	}

	counts, ok := resp["counts"].(map[string]any)
	if !ok {
		t.Fatalf("Expected counts object, got %T", resp["counts"])
	}
	if got := counts["affectedPages"]; got != float64(1) {
		t.Fatalf("Expected counts.affectedPages=1, got %v", got)
	}
	if _, ok := counts["matchedLinks"]; !ok {
		t.Fatalf("Expected counts.matchedLinks in response, got %v", counts)
	}
}

func TestUploadAssetEndpoint_RejectsFilesExceedingConfiguredLimit(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		MaxAssetUploadSizeBytes: 32,
	})

	page := createPageViaAPI(t, router, "Asset Limit Test", "asset-limit-test", nil, pageNodeKind())

	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Failed to login: %d - %s", loginRec.Code, loginRec.Body.String())
	}

	cookies := loginRec.Result().Cookies()
	csrfToken := loginRec.Header().Get("X-CSRF-Token")
	if csrfToken == "" {
		for _, c := range cookies {
			if c.Name == "leafwiki_csrf" || c.Name == "__Host-leafwiki_csrf" {
				csrfToken = c.Value
				break
			}
		}
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "large.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := part.Write([]byte(strings.Repeat("a", 128))); err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/pages/"+page.ID+"/assets", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		uploadReq.AddCookie(cookie)
	}

	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Expected 413 Request Entity Too Large, got %d - %s", uploadRec.Code, uploadRec.Body.String())
	}

	assetDir := filepath.Join(w.GetStorageDir(), "assets", page.ID)
	entries, err := os.ReadDir(assetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("Failed to read asset directory: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("Expected no files after rejected upload, got %d", len(entries))
	}
}

func TestSuggestSlugEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstanceWithRevision(w, t)

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

func TestCancelImportPlanEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	fileWriter, err := writer.CreateFormFile("file", "fixture-1.zip")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}

	zipFile, err := os.Open("../importer/fixtures/fixture-1.zip")
	if err != nil {
		t.Fatalf("Open fixture zip failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(zipFile.Close, t)

	if _, err := io.Copy(fileWriter, zipFile); err != nil {
		t.Fatalf("Copy zip fixture failed: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer failed: %v", err)
	}

	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Failed to login: %d - %s", loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

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

	createReq := httptest.NewRequest(http.MethodPost, "/api/import/plan", &body)
	createReq.Header.Set("Content-Type", writer.FormDataContentType())
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		createReq.AddCookie(cookie)
	}

	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 when creating import plan, got %d: %s", createRec.Code, createRec.Body.String())
	}

	cancelReq := httptest.NewRequest(http.MethodDelete, "/api/import/plan", nil)
	cancelReq.Header.Set("Content-Type", "application/json")
	cancelReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		cancelReq.AddCookie(cookie)
	}

	cancelRec := httptest.NewRecorder()
	router.ServeHTTP(cancelRec, cancelReq)

	if cancelRec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 when canceling import plan, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}
	if got := strings.TrimSpace(cancelRec.Body.String()); got != "null" {
		t.Fatalf("Expected null response body when clearing import plan, got %q", got)
	}

	getRec := authenticatedRequest(t, router, http.MethodGet, "/api/import/plan", nil)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404 when fetching canceled import plan, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	if resp["error"] == "" {
		t.Fatalf("Expected error message after canceling import plan, got: %v", resp)
	}
}

func TestImportExecuteEndpoint_WithZipUpload_ImportsPagesLinksAndAssets(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	fixtureDir := importerFixturePathForHTTPTests(t, "link-assets-package")
	zipBytes := createZipFromDir(t, fixtureDir)

	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Failed to login: %d - %s", loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

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

	var planBody bytes.Buffer
	planWriter := multipart.NewWriter(&planBody)
	fileWriter, err := planWriter.CreateFormFile("file", "link-assets-package.zip")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := fileWriter.Write(zipBytes); err != nil {
		t.Fatalf("Write zip bytes failed: %v", err)
	}
	if err := planWriter.Close(); err != nil {
		t.Fatalf("Close multipart writer failed: %v", err)
	}

	planReq := httptest.NewRequest(http.MethodPost, "/api/import/plan", &planBody)
	planReq.Header.Set("Content-Type", planWriter.FormDataContentType())
	planReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		planReq.AddCookie(cookie)
	}

	planRec := httptest.NewRecorder()
	router.ServeHTTP(planRec, planReq)

	if planRec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 when creating import plan, got %d: %s", planRec.Code, planRec.Body.String())
	}

	var planResp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(planRec.Body.Bytes(), &planResp); err != nil {
		t.Fatalf("Invalid import plan response JSON: %v", err)
	}
	if len(planResp.Items) != 5 {
		t.Fatalf("expected 5 plan items, got %d", len(planResp.Items))
	}

	execReq := httptest.NewRequest(http.MethodPost, "/api/import/execute", strings.NewReader(""))
	execReq.Header.Set("Content-Type", "application/json")
	execReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		execReq.AddCookie(cookie)
	}

	execRec := httptest.NewRecorder()
	router.ServeHTTP(execRec, execReq)

	if execRec.Code != http.StatusAccepted {
		t.Fatalf("Expected status 202 when starting import, got %d: %s", execRec.Code, execRec.Body.String())
	}

	var execResp struct {
		ImportedCount   int    `json:"imported_count"`
		SkippedCount    int    `json:"skipped_count"`
		ExecutionStatus string `json:"execution_status"`
		ExecutionResult *struct {
			ImportedCount int `json:"imported_count"`
			SkippedCount  int `json:"skipped_count"`
		} `json:"execution_result"`
	}
	if err := json.Unmarshal(execRec.Body.Bytes(), &execResp); err != nil {
		t.Fatalf("Invalid import execute response JSON: %v", err)
	}

	if execResp.ExecutionStatus != "running" {
		t.Fatalf("expected running execution status, got %q", execResp.ExecutionStatus)
	}

	var completedResp struct {
		ExecutionStatus string `json:"execution_status"`
		ExecutionResult *struct {
			ImportedCount int `json:"imported_count"`
			SkippedCount  int `json:"skipped_count"`
		} `json:"execution_result"`
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		statusReq := httptest.NewRequest(http.MethodGet, "/api/import/plan", nil)
		for _, cookie := range cookies {
			statusReq.AddCookie(cookie)
		}

		statusRec := httptest.NewRecorder()
		router.ServeHTTP(statusRec, statusReq)

		if statusRec.Code != http.StatusOK {
			t.Fatalf("Expected status 200 when fetching import plan, got %d: %s", statusRec.Code, statusRec.Body.String())
		}
		if err := json.Unmarshal(statusRec.Body.Bytes(), &completedResp); err != nil {
			t.Fatalf("Invalid import status response JSON: %v", err)
		}
		if completedResp.ExecutionStatus == "completed" {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	if completedResp.ExecutionStatus != "completed" || completedResp.ExecutionResult == nil {
		t.Fatalf("expected completed execution result, got %#v", completedResp)
	}
	if completedResp.ExecutionResult.ImportedCount != 5 || completedResp.ExecutionResult.SkippedCount != 0 {
		t.Fatalf(
			"unexpected execution result: imported=%d skipped=%d",
			completedResp.ExecutionResult.ImportedCount,
			completedResp.ExecutionResult.SkippedCount,
		)
	}

	setupPage := getPageByPathViaAPI(t, router, "guides/setup")
	for _, expected := range []string{
		"[Relative MD](/reference/endpoints)",
		"[Absolute MD](/reference/endpoints)",
		"[Container](/guides)",
		"[Endpoints](/reference/endpoints)",
		"[API Alias](/reference/endpoints)",
		"![Relative Image](/assets/" + setupPage.ID + "/logo.png)",
		"[Manual](/assets/" + setupPage.ID + "/manual.pdf)",
	} {
		if !strings.Contains(setupPage.Content, expected) {
			t.Fatalf("expected setup content to contain %q, got:\n%s", expected, setupPage.Content)
		}
	}

	assets := listAssetsViaAPI(t, router, setupPage.ID)
	if len(assets) != 2 {
		t.Fatalf("expected 2 uploaded assets, got %#v", assets)
	}
	_ = getPageByPathViaAPI(t, router, "reference/api-1")
}

func TestImportExecuteEndpoint_UsesConfiguredAssetUploadLimit(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstanceWithMaxAssetUploadSize(w, t, 1024)

	fixtureDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fixtureDir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "docs", "setup.md"), []byte("# Setup\n\n[Manual](./manual.pdf)\n"), 0o644); err != nil {
		t.Fatalf("write markdown fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "docs", "manual.pdf"), bytes.Repeat([]byte("a"), 2048), 0o644); err != nil {
		t.Fatalf("write oversized asset fixture: %v", err)
	}

	zipBytes := createZipFromDir(t, fixtureDir)

	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Failed to login: %d - %s", loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

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

	var planBody bytes.Buffer
	planWriter := multipart.NewWriter(&planBody)
	fileWriter, err := planWriter.CreateFormFile("file", "oversized-assets.zip")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := fileWriter.Write(zipBytes); err != nil {
		t.Fatalf("Write zip bytes failed: %v", err)
	}
	if err := planWriter.Close(); err != nil {
		t.Fatalf("Close multipart writer failed: %v", err)
	}

	planReq := httptest.NewRequest(http.MethodPost, "/api/import/plan", &planBody)
	planReq.Header.Set("Content-Type", planWriter.FormDataContentType())
	planReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		planReq.AddCookie(cookie)
	}

	planRec := httptest.NewRecorder()
	router.ServeHTTP(planRec, planReq)

	if planRec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 when creating import plan, got %d: %s", planRec.Code, planRec.Body.String())
	}

	execReq := httptest.NewRequest(http.MethodPost, "/api/import/execute", strings.NewReader(""))
	execReq.Header.Set("Content-Type", "application/json")
	execReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range cookies {
		execReq.AddCookie(cookie)
	}

	execRec := httptest.NewRecorder()
	router.ServeHTTP(execRec, execReq)

	if execRec.Code != http.StatusAccepted {
		t.Fatalf("Expected status 202 when starting import, got %d: %s", execRec.Code, execRec.Body.String())
	}

	var execResp struct {
		ExecutionStatus string `json:"execution_status"`
	}
	if err := json.Unmarshal(execRec.Body.Bytes(), &execResp); err != nil {
		t.Fatalf("Invalid import execute response JSON: %v", err)
	}
	if execResp.ExecutionStatus != "running" {
		t.Fatalf("expected running execution status, got %q", execResp.ExecutionStatus)
	}

	var completedResp struct {
		ExecutionStatus string `json:"execution_status"`
		ExecutionResult *struct {
			ImportedCount int `json:"imported_count"`
			SkippedCount  int `json:"skipped_count"`
			Items         []struct {
				Error *string `json:"error"`
			} `json:"items"`
		} `json:"execution_result"`
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		statusReq := httptest.NewRequest(http.MethodGet, "/api/import/plan", nil)
		for _, cookie := range cookies {
			statusReq.AddCookie(cookie)
		}

		statusRec := httptest.NewRecorder()
		router.ServeHTTP(statusRec, statusReq)

		if statusRec.Code != http.StatusOK {
			t.Fatalf("Expected status 200 when fetching import plan, got %d: %s", statusRec.Code, statusRec.Body.String())
		}
		if err := json.Unmarshal(statusRec.Body.Bytes(), &completedResp); err != nil {
			t.Fatalf("Invalid import status response JSON: %v", err)
		}
		if completedResp.ExecutionStatus == "completed" {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	if completedResp.ExecutionStatus != "completed" || completedResp.ExecutionResult == nil {
		t.Fatalf("expected completed execution result, got %#v", completedResp)
	}
	if completedResp.ExecutionResult.ImportedCount != 0 || completedResp.ExecutionResult.SkippedCount != 1 {
		t.Fatalf(
			"unexpected execution result: imported=%d skipped=%d",
			completedResp.ExecutionResult.ImportedCount,
			completedResp.ExecutionResult.SkippedCount,
		)
	}
	if len(completedResp.ExecutionResult.Items) != 1 || completedResp.ExecutionResult.Items[0].Error == nil || !strings.Contains(*completedResp.ExecutionResult.Items[0].Error, "file too large") {
		t.Fatalf("expected import error about configured asset limit, got %#v", completedResp.ExecutionResult.Items)
	}
}

func TestSuggestSlugEndpoint_MissingTitle(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/slug-suggestion", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	page := createPageViaAPI(t, router, "Delete Me", "delete-me", nil, pageNodeKind())
	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+page.ID, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	getRec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+page.ID, nil)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("Expected deleted page to return 404, got %d", getRec.Code)
	}
}

func TestDeletePageEndpoint_NotFound(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/not-found-id", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint_HasChildren(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	parent := createPageViaAPI(t, router, "Parent", "parent", nil, pageNodeKind())
	createPageViaAPI(t, router, "Child", "child", &parent.ID, pageNodeKind())

	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+parent.ID, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestDeletePageEndpoint_Recursive(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	parent := createPageViaAPI(t, router, "Parent", "parent", nil, pageNodeKind())
	createPageViaAPI(t, router, "Child", "child", &parent.ID, pageNodeKind())

	rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+parent.ID+"?recursive=true", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	getRec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+parent.ID, nil)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("Expected deleted page to return 404, got %d", getRec.Code)
	}
}

func TestUpdatePageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	page := createPageViaAPI(t, router, "Original Title", "original-title", nil, pageNodeKind())

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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `{"title": "Updated", "slug": "updated", "content": "New content"}`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/not-found-id", strings.NewReader(string(body)))
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown page, got %d", rec.Code)
	}
}

func TestUpdatePage_SlugRemainsIfUnchanged(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	// Create a page
	created := createPageViaAPI(t, router, "Immutable Slug", "immutable-slug", nil, pageNodeKind())

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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	page := createPageViaAPI(t, router, "Original Title", "original-title", nil, pageNodeKind())
	createPageViaAPI(t, router, "Conflict Title", "conflict-title", nil, pageNodeKind())

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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `this is not valid json`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/invalid-id", strings.NewReader(string(body)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestUpdatePage_MissingTitle(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `{"slug": "updated", "content": "New content"}`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-title", strings.NewReader(string(body)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing title, got %d", rec.Code)
	}
}

func TestUpdatePage_MissingSlug(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `{"title": "Updated", "content": "New content"}`
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-slug", strings.NewReader(string(body)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing slug, got %d", rec.Code)
	}
}

func TestGetPageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	// Create a page
	page := createPageViaAPI(t, router, "Welcome", "welcome", nil, pageNodeKind())

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

	if resp["title"] != page.Title {
		t.Errorf("Expected title in response, got: %v", resp)
	}

	if resp["slug"] != page.Slug {
		t.Errorf("Expected slug in response, got: %v", resp)
	}
}

func TestGetPageEndpoint_NotFound(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/not-found-id", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestGetPageEndpoint_MissingID(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestGetPageByPathEndpoint_MissingPath(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/by-path", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestGetPageByPathEndpoint_NotFound(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/by-path?path=does-not-exist", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestGetPageByPathEndpoint_PageReturnsNoChildren(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	// Create a standalone page (no children – adding children auto-converts it to a section)
	createPageViaAPI(t, router, "My Page", "my-page", nil, pageNodeKind())

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/by-path?path=my-page", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d - %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Page kind (depth=0): the node must be returned with children absent or null
	if resp["kind"] != "page" {
		t.Errorf("Expected kind 'page', got: %v", resp["kind"])
	}
	if children, ok := resp["children"]; ok && children != nil {
		t.Errorf("Expected no children for page kind (depth=0), got: %v", children)
	}
}

func TestGetPageByPathEndpoint_SectionReturnsDirectChildrenOnly(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	sectionKind := tree.NodeKindSection

	// Create a section with a child page that itself has a grandchild
	section := createPageViaAPI(t, router, "My Section", "my-section", nil, &sectionKind)
	child := createPageViaAPI(t, router, "Child Page", "child-page", &section.ID, pageNodeKind())
	createPageViaAPI(t, router, "Grandchild Page", "grandchild-page", &child.ID, pageNodeKind())

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/by-path?path=my-section", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d - %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Section kind (depth=1): direct children must be present
	children, ok := resp["children"].([]interface{})
	if !ok || len(children) == 0 {
		t.Fatalf("Expected direct children for section kind (depth=1), got: %v", resp["children"])
	}

	// Grandchildren must be absent or null (depth=1 means children's children are not included)
	firstChild, ok := children[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected child to be an object, got: %v", children[0])
	}
	if grandchildren, ok := firstChild["children"]; ok && grandchildren != nil {
		t.Errorf("Expected no grandchildren for section kind (depth=1), got: %v", grandchildren)
	}
}

func TestMovePageEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	// Create two pages a and b
	a := createPageViaAPI(t, router, "Section A", "section-a", nil, pageNodeKind())
	b := createPageViaAPI(t, router, "Section B", "section-b", nil, pageNodeKind())

	// Move a under b
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"`+b.ID+`"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	// Check if a is now a child of b
	movedParent := getPageByPathViaAPI(t, router, "section-b")
	if len(movedParent.Children) != 1 || movedParent.Children[0].ID != a.ID {
		t.Errorf("Expected page to be moved under new parent")
	}
}

func TestMovePageEndpoint_NotFound(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/not-found-id/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_InvalidJSON(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/invalid-id/move", strings.NewReader(`this is not valid json`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_MissingParentID(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/missing-parent/move", strings.NewReader(`{"parentId":""}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_ParentNotFound(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	a := createPageViaAPI(t, router, "Section A", "section-a", nil, pageNodeKind())

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"not-found-id"}`))

	t.Logf("Response: %s", rec.Body.String())
	t.Logf("Response Code: %d", rec.Code)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", rec.Code)
	}
}

func TestMovePageEndpoint_CircularReference(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	a := createPageViaAPI(t, router, "Section A", "section-a", nil, pageNodeKind())
	b := createPageViaAPI(t, router, "Section B", "section-b", &a.ID, pageNodeKind())

	// Verschiebe a → unter b
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+b.ID+"/move", strings.NewReader(`{"parentId":"`+a.ID+`"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePage_FailsIfTargetAlreadyHasPageWithSameSlug(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	a := createPageViaAPI(t, router, "Section A", "section-a", nil, pageNodeKind())
	createPageViaAPI(t, router, "Section B", "section-b", nil, pageNodeKind())

	// Create Conflict Page in b
	conflictPage := createPageViaAPI(t, router, "Section B", "section-b", &a.ID, pageNodeKind())

	// move conflictPage under root (where section-b already exists)
	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+conflictPage.ID+"/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestMovePage_InTheSamePlace(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	a := createPageViaAPI(t, router, "Section A", "section-a", nil, pageNodeKind())

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+a.ID+"/move", strings.NewReader(`{"parentId":"root"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", rec.Code)
	}
}

func TestSortPagesEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	// Create pages
	page1 := createPageViaAPI(t, router, "Page 1", "page-1", nil, pageNodeKind())
	page2 := createPageViaAPI(t, router, "Page 2", "page-2", nil, pageNodeKind())
	page3 := createPageViaAPI(t, router, "Page 3", "page-3", nil, pageNodeKind())
	welcomePage := getPageByPathViaAPI(t, router, "welcome-to-leafwiki")
	deletePageViaAPI(t, router, welcomePage.ID, false)

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

	root := getTreeViaAPI(t, router)
	if len(root.Children) != 3 {
		t.Fatalf("Expected 3 children in root, got: %d", len(root.Children))
	}

	if root.Children[0].ID != page3.ID {
		t.Errorf("Expected first child to be page 3, got: %v", root.Children[0].ID)
	}
	if root.Children[1].ID != page1.ID {
		t.Errorf("Expected second child to be page 1, got: %v", root.Children[1].ID)
	}
	if root.Children[2].ID != page2.ID {
		t.Errorf("Expected third child to be page 2, got: %v", root.Children[2].ID)
	}
}

func TestAuthLoginEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(res.Body.Close, t)

	// Prüfen, ob Cookies gesetzt wurden
	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("Expected auth cookies to be set on login")
	}
}

func TestAuthLogin_InvalidCredentials(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(refreshRes.Body.Close, t)
	newCookies := refreshRes.Cookies()
	if len(newCookies) == 0 {
		t.Fatalf("Expected new auth cookies on refresh")
	}
}

func TestCreateUserEndpoint(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `{"username": "john", "email": "john@example.com", "password": "secret123", "role": "editor"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", rec.Code)
	}
}

func TestCreateUser_DuplicateEmailOrUsername(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `{"username": "sam", "email": "sam@example.com", "password": "secret1234", "role": "undefined"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid role, got %d", rec.Code)
	}
}

func TestCreateUser_WithViewerRole(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	body := `{"username": "vieweruser", "email": "viewer@example.com", "password": "secret1234", "role": "viewer"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created for viewer role, got %d", rec.Code)
	}
}

func TestUpdateUser_RoleToViewer(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	// Default Admin create user should succeed
	body := `{"username": "mod", "email": "mod@example.com", "password": "secretpassword", "role": "editor"}`
	rec := authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created by admin, got %d", rec.Code)
	}
}

func TestRequireAdminMiddleware_BlockedWhenAuthDisabled(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Create router with auth disabled
	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		AuthDisabled:            true, // Auth is disabled
	})

	// Test POST /api/users (admin-only endpoint)
	createUserBody := `{"username": "testuser", "email": "test@example.com", "password": "password", "role": "editor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(createUserBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for POST /api/users when auth disabled, got %d - %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/users (admin-only endpoint)
	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for GET /api/users when auth disabled, got %d - %s", rec.Code, rec.Body.String())
	}

	// Test DELETE /api/users/:id (admin-only endpoint)
	req = httptest.NewRequest(http.MethodDelete, "/api/users/some-user-id", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for DELETE /api/users/:id when auth disabled, got %d - %s", rec.Code, rec.Body.String())
	}
}

func TestRequireAuthMiddleware_Unauthorized(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

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
	page := createPageViaAPI(t, router, "Assets Page", "assets-page", nil, pageNodeKind())

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

func TestAssetMutationRevisionsUseAuthenticatedUser(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstanceWithRevision(w, t)
	adminUserID := getAdminUserIDViaAPI(t, router)

	loginBody := `{"identifier": "admin", "password": "admin"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on login, got %d - %s", loginRec.Code, loginRec.Body.String())
	}

	loginRes := loginRec.Result()
	defer test_utils.WrapCloseWithErrorCheck(loginRes.Body.Close, t)

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

	writeAsset := func(t *testing.T, pageID, name string) {
		t.Helper()

		assetDir := filepath.Join(w.GetStorageDir(), "assets", pageID)
		if err := os.MkdirAll(assetDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(assetDir) failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(assetDir, name), []byte("payload"), 0o644); err != nil {
			t.Fatalf("WriteFile(asset) failed: %v", err)
		}
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T, pageID string)
		request func(t *testing.T, pageID string) *http.Request
	}{
		{
			name: "upload",
			request: func(t *testing.T, pageID string) *http.Request {
				t.Helper()

				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, err := writer.CreateFormFile("file", "upload.txt")
				if err != nil {
					t.Fatalf("CreateFormFile failed: %v", err)
				}
				if _, err := part.Write([]byte("payload")); err != nil {
					t.Fatalf("Write(asset payload) failed: %v", err)
				}
				if err := writer.Close(); err != nil {
					t.Fatalf("Close(writer) failed: %v", err)
				}

				req := httptest.NewRequest(http.MethodPost, "/api/pages/"+pageID+"/assets", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req
			},
		},
		{
			name: "rename",
			setup: func(t *testing.T, pageID string) {
				t.Helper()
				writeAsset(t, pageID, "old.txt")
			},
			request: func(t *testing.T, pageID string) *http.Request {
				t.Helper()
				req := httptest.NewRequest(http.MethodPut, "/api/pages/"+pageID+"/assets/rename", strings.NewReader(`{"old_filename":"old.txt","new_filename":"new.txt"}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
		},
		{
			name: "delete",
			setup: func(t *testing.T, pageID string) {
				t.Helper()
				// Upload through the HTTP API so the setup exercises the same path as production.
				uploadAssetViaAPI(t, router, pageID, "delete.txt", "payload")
			},
			request: func(t *testing.T, pageID string) *http.Request {
				t.Helper()
				return httptest.NewRequest(http.MethodDelete, "/api/pages/"+pageID+"/assets/delete.txt", nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page := createPageViaAPI(t, router, "Assets "+tc.name, "assets-"+tc.name, nil, pageNodeKind())

			if tc.setup != nil {
				tc.setup(t, page.ID)
			}

			req := tc.request(t, page.ID)
			addCookies(req)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code < http.StatusOK || rec.Code >= http.StatusMultipleChoices {
				t.Fatalf("Expected 2xx response, got %d - %s", rec.Code, rec.Body.String())
			}

			latest := getLatestRevisionViaAPI(t, router, page.ID)
			if latest["type"] != string(revision.RevisionTypeAssetUpdate) {
				t.Fatalf("latest type = %v, want %q", latest["type"], revision.RevisionTypeAssetUpdate)
			}
			if latest["authorId"] != adminUserID {
				t.Fatalf("latest author = %v, want %q", latest["authorId"], adminUserID)
			}
		})
	}
}

// Lets check the indexing status
func TestIndexingStatusEndpoint(t *testing.T) {
	// Lets call /api/search/status
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
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

// uploadTestAsset is a helper function that creates a page, uploads an asset, and returns the asset URL and auth cookies.
// If needsAuth is true, it will obtain authentication cookies; otherwise it will get CSRF token only (for AuthDisabled mode).
func uploadTestAsset(t *testing.T, router *gin.Engine, w *wiki.Wiki, content string, needsAuth bool) (assetURL string, cookies []*http.Cookie) {
	// Create a page
	pageID := ""
	if needsAuth {
		pageID = createPageViaAPI(t, router, "Test Page", "test-page", nil, pageNodeKind()).ID
	}

	// Prepare the file upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}

	var csrfToken string

	if needsAuth {
		// Login to get auth cookies
		loginBody := `{"identifier": "admin", "password": "admin"}`
		loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRec := httptest.NewRecorder()
		router.ServeHTTP(loginRec, loginReq)

		if loginRec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK on login, got %d", loginRec.Code)
		}

		cookies = loginRec.Result().Cookies()
		csrfToken = loginRec.Header().Get("X-CSRF-Token")
		if csrfToken == "" {
			for _, c := range cookies {
				if c.Name == "leafwiki_csrf" || c.Name == "__Host-leafwiki_csrf" {
					csrfToken = c.Value
					break
				}
			}
		}
	} else {
		createBody := `{"title":"Test Page","slug":"test-page"}`
		createReq := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(createBody))
		createReq.Header.Set("Content-Type", "application/json")

		// Get CSRF token only (for AuthDisabled mode)
		configReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		configRec := httptest.NewRecorder()
		router.ServeHTTP(configRec, configReq)

		cookies = configRec.Result().Cookies()
		csrfToken = configRec.Header().Get("X-CSRF-Token")
		if csrfToken == "" {
			for _, c := range cookies {
				if c.Name == "leafwiki_csrf" || c.Name == "__Host-leafwiki_csrf" {
					csrfToken = c.Value
					break
				}
			}
		}

		for _, cookie := range cookies {
			createReq.AddCookie(cookie)
		}
		createReq.Header.Set("X-CSRF-Token", csrfToken)

		createRec := httptest.NewRecorder()
		router.ServeHTTP(createRec, createReq)

		if createRec.Code != http.StatusCreated {
			t.Fatalf("Expected 201 Created on page creation, got %d - %s", createRec.Code, createRec.Body.String())
		}

		var pageResp apiPage
		if err := json.Unmarshal(createRec.Body.Bytes(), &pageResp); err != nil {
			t.Fatalf("Invalid page creation JSON: %v", err)
		}
		pageID = pageResp.ID
	}

	// Upload the asset
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/pages/"+pageID+"/assets", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	for _, cookie := range cookies {
		uploadReq.AddCookie(cookie)
	}
	uploadReq.Header.Set("X-CSRF-Token", csrfToken)

	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created on upload, got %d - %s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploadResp map[string]string
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("Invalid upload JSON: %v", err)
	}

	assetURL = uploadResp["file"]
	if assetURL == "" {
		t.Fatal("Expected file URL in upload response")
	}

	return assetURL, cookies
}

// TestAssetAccessControl tests the access control for static asset routes
func TestAssetAccessControl(t *testing.T) {
	t.Run("PrivateMode_UnauthenticatedAccess_Returns401", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		// Create router with PublicAccess=false and AuthDisabled=false
		router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
			PublicAccess:            false,
			InjectCodeInHeader:      "",
			CustomStylesheet:        "",
			AllowInsecure:           true,
			AccessTokenTimeout:      15 * time.Minute,
			RefreshTokenTimeout:     7 * 24 * time.Hour,
			HideLinkMetadataSection: false,
			AuthDisabled:            false,
		})

		// Upload an asset (with auth)
		assetURL, _ := uploadTestAsset(t, router, w, "test content", true)

		// Try to access the asset without authentication
		assetReq := httptest.NewRequest(http.MethodGet, assetURL, nil)
		assetRec := httptest.NewRecorder()
		router.ServeHTTP(assetRec, assetReq)

		// Should return 401 Unauthorized
		if assetRec.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized when accessing asset without auth in private mode, got %d", assetRec.Code)
		}
	})

	t.Run("PrivateMode_AuthenticatedAccess_Returns200", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		// Create router with PublicAccess=false and AuthDisabled=false
		router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
			PublicAccess:            false,
			InjectCodeInHeader:      "",
			CustomStylesheet:        "",
			AllowInsecure:           true,
			AccessTokenTimeout:      15 * time.Minute,
			RefreshTokenTimeout:     7 * 24 * time.Hour,
			HideLinkMetadataSection: false,
			AuthDisabled:            false,
		})

		// Upload an asset (with auth) and get cookies
		assetURL, cookies := uploadTestAsset(t, router, w, "test content", true)

		// Access the asset with authentication
		assetReq := httptest.NewRequest(http.MethodGet, assetURL, nil)
		for _, cookie := range cookies {
			assetReq.AddCookie(cookie)
		}
		assetRec := httptest.NewRecorder()
		router.ServeHTTP(assetRec, assetReq)

		// Should return 200 OK
		if assetRec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK when accessing asset with auth in private mode, got %d", assetRec.Code)
		}

		// Verify content
		content := assetRec.Body.String()
		if content != "test content" {
			t.Errorf("Expected 'test content', got '%s'", content)
		}
	})

	t.Run("PublicAccessMode_UnauthenticatedAccess_Returns200", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		// Create router with PublicAccess=true
		router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
			PublicAccess:            true,
			InjectCodeInHeader:      "",
			CustomStylesheet:        "",
			AllowInsecure:           true,
			AccessTokenTimeout:      15 * time.Minute,
			RefreshTokenTimeout:     7 * 24 * time.Hour,
			HideLinkMetadataSection: false,
			AuthDisabled:            false,
		})

		// Upload an asset (with auth)
		assetURL, _ := uploadTestAsset(t, router, w, "test content public", true)

		// Try to access the asset without authentication
		assetReq := httptest.NewRequest(http.MethodGet, assetURL, nil)
		assetRec := httptest.NewRecorder()
		router.ServeHTTP(assetRec, assetReq)

		// Should return 200 OK in public mode
		if assetRec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK when accessing asset without auth in public mode, got %d", assetRec.Code)
		}

		// Verify content
		content := assetRec.Body.String()
		if content != "test content public" {
			t.Errorf("Expected 'test content public', got '%s'", content)
		}
	})

	t.Run("AuthDisabledMode_UnauthenticatedAccess_Returns200", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		// Create router with AuthDisabled=true
		router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
			PublicAccess:            false,
			InjectCodeInHeader:      "",
			CustomStylesheet:        "",
			AllowInsecure:           true,
			AccessTokenTimeout:      15 * time.Minute,
			RefreshTokenTimeout:     7 * 24 * time.Hour,
			HideLinkMetadataSection: false,
			AuthDisabled:            true,
		})

		// Upload an asset (no auth needed, but CSRF token still required)
		assetURL, _ := uploadTestAsset(t, router, w, "test content no auth", false)

		// Try to access the asset without authentication
		assetReq := httptest.NewRequest(http.MethodGet, assetURL, nil)
		assetRec := httptest.NewRecorder()
		router.ServeHTTP(assetRec, assetReq)

		// Should return 200 OK when auth is disabled
		if assetRec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK when accessing asset without auth when AuthDisabled=true, got %d", assetRec.Code)
		}

		// Verify content
		content := assetRec.Body.String()
		if content != "test content no auth" {
			t.Errorf("Expected 'test content no auth', got '%s'", content)
		}
	})
}

func TestBuildCustomStylesheetTag(t *testing.T) {
	tag := httpinternal.BuildCustomStylesheetTag("/wiki", "/tmp/custom.css")

	expected := `<link rel="stylesheet" href="/wiki/custom.css">`
	if tag != expected {
		t.Fatalf("expected %q, got %q", expected, tag)
	}
}

func TestBuildCustomStylesheetTag_EmptyPath(t *testing.T) {
	tag := httpinternal.BuildCustomStylesheetTag("", "")
	if tag != "" {
		t.Fatalf("expected empty tag, got %q", tag)
	}
}

func TestInjectIntoHead(t *testing.T) {
	html := "<html><head></head><body></body></html>"
	got := httpinternal.InjectIntoHead(html, `<link rel="stylesheet" href="/custom.css">`)

	if !strings.Contains(got, `<link rel="stylesheet" href="/custom.css">`) {
		t.Fatalf("expected stylesheet link to be injected, got %q", got)
	}
}

func TestCustomStylesheetRoute(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	customCSSPath := filepath.Join(w.GetStorageDir(), "custom.css")
	if err := os.WriteFile(customCSSPath, []byte("body { color: red; }"), 0644); err != nil {
		t.Fatalf("failed to create custom stylesheet: %v", err)
	}

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		CustomStylesheet:        customCSSPath,
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		AuthDisabled:            false,
	})

	req := httptest.NewRequest(http.MethodGet, "/custom.css", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("expected css content-type, got %q", got)
	}

	if !strings.Contains(rec.Body.String(), "body { color: red; }") {
		t.Fatalf("expected CSS body, got %q", rec.Body.String())
	}
}

func TestCustomStylesheetRoute_RejectsPathOutsideStorageDir(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	outsideCSSPath := filepath.Join(t.TempDir(), "outside.css")
	if err := os.WriteFile(outsideCSSPath, []byte("body { color: blue; }"), 0644); err != nil {
		t.Fatalf("failed to create stylesheet outside storage dir: %v", err)
	}

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		CustomStylesheet:        outsideCSSPath,
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		AuthDisabled:            false,
	})

	req := httptest.NewRequest(http.MethodGet, "/custom.css", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when stylesheet path is outside storage dir, got %d", rec.Code)
	}
}

func TestCustomStylesheetRoute_RejectsNonCSSFile(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	textFilePath := filepath.Join(w.GetStorageDir(), "custom.txt")
	if err := os.WriteFile(textFilePath, []byte("not css"), 0644); err != nil {
		t.Fatalf("failed to create non-css file: %v", err)
	}

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		CustomStylesheet:        textFilePath,
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		AuthDisabled:            false,
	})

	req := httptest.NewRequest(http.MethodGet, "/custom.css", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when stylesheet path is not a css file, got %d", rec.Code)
	}
}

func TestBrandingAssetRoute_DisablesClientCache(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	router := createRouterTestInstance(w, t)
	uploadBrandingLogoViaAPI(t, router, "logo.png", []byte("logo"))

	req := httptest.NewRequest(http.MethodGet, "/branding/logo.png", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", got)
	}
}

func TestFaviconRoute_DisablesClientCache(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	EmbedFrontendOrig := httpinternal.EmbedFrontend
	httpinternal.EmbedFrontend = "true"
	defer func() {
		httpinternal.EmbedFrontend = EmbedFrontendOrig
	}()

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            false,
		InjectCodeInHeader:      "",
		CustomStylesheet:        "",
		AllowInsecure:           true,
		AccessTokenTimeout:      15 * time.Minute,
		RefreshTokenTimeout:     7 * 24 * time.Hour,
		HideLinkMetadataSection: false,
		AuthDisabled:            false,
	})

	req := httptest.NewRequest(http.MethodGet, "/favicon.svg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", got)
	}
}

func TestBuildCustomStylesheetTag_WhitespacePath(t *testing.T) {
	tag := httpinternal.BuildCustomStylesheetTag("/wiki", "   ")
	if tag != "" {
		t.Fatalf("expected empty tag for whitespace path, got %q", tag)
	}
}
