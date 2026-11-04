package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
)

const (
	// mcpInitializeBody is a spec-conformant initialize request; the
	// response is a plain JSON JSON-RPC reply (no SSE) because the client
	// does not opt into streaming.
	mcpInitializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"leafwiki-test","version":"0"}}}`
	mcpToolsListBody  = `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
)

// mcpJSONRPC is the JSON-RPC envelope of a /api/mcp response.
type mcpJSONRPC struct {
	Result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// newMCPRouterTest builds a Wiki+router pair with the experimental MCP
// server enabled, mirroring newAPIKeyRouterTest but without the snapshot/
// restore/backup routes (the MCP endpoint does not need them). With apiKeys,
// API-key management is on and Bearer auth is wired, so a key created through
// the admin session authenticates against /api/mcp.
func newMCPRouterTest(t *testing.T, apiKeys bool) (*wiki.Wiki, http.Handler) {
	t.Helper()
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:             t.TempDir(),
		AdminPassword:          "adminpassword",
		JWTSecret:              "secretkey",
		AccessTokenTimeout:     15 * time.Minute,
		RefreshTokenTimeout:    7 * 24 * time.Hour,
		EnableMCP:              true,
		EnableRevision:         true,
		EnableAPIKeyManagement: apiKeys,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })

	router := newTestRouter(w, func(o *httpinternal.RouterOptions) {
		if apiKeys {
			o.APIKeyService = w.APIKeyService()
			o.EnableAPIKeyManagement = true
		}
	})
	return w, router
}

// mcpPost sends a JSON-RPC body to the MCP endpoint with the session cookies
// (and CSRF token for a state-changing method), like authenticatedRequest but
// without re-logging in. sess nil = unauthenticated.
func mcpPost(t *testing.T, router http.Handler, body, sessionID string, sess *testSession) *httptest.ResponseRecorder {
	t.Helper()
	req := mcpPostRequest(body, sessionID, sess)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func mcpPostRequest(body, sessionID string, sess *testSession) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	if sess != nil {
		for _, c := range sess.cookies {
			req.AddCookie(c)
		}
		req.Header.Set("X-CSRF-Token", sess.csrf)
	}
	return req
}

// mcpPostBearer sends a JSON-RPC body authenticated with a Bearer API key.
// A resolved key skips CSRF in the middleware chain, so only the
// Authorization header is needed.
// mcpDelete terminates an MCP session via the transport's DELETE verb.
// Session auth is state-changing, so the CSRF token is required like for
// mcpPost.
func mcpDelete(t *testing.T, router http.Handler, sessionID string, sess *testSession) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/mcp", nil)
	req.Header.Set("Mcp-Session-Id", sessionID)
	if sess != nil {
		for _, c := range sess.cookies {
			req.AddCookie(c)
		}
		req.Header.Set("X-CSRF-Token", sess.csrf)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func mcpPostBearer(t *testing.T, router http.Handler, body, secret, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+secret)
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeMCP(t *testing.T, rec *httptest.ResponseRecorder) mcpJSONRPC {
	t.Helper()
	var resp mcpJSONRPC
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode MCP response %q: %v", rec.Body.String(), err)
	}
	return resp
}

// TestMCP_DisabledByDefault proves the endpoint does not exist at all when
// --enable-mcp is off: the request never reaches the MCP handler and the SPA
// fallback answers 404 for the unknown /api path.
func TestMCP_DisabledByDefault(t *testing.T) {
	w := createWikiTestInstance(t)
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	router := createRouterTestInstance(w, t)

	rec := mcpPost(t, router, mcpInitializeBody, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /api/mcp with the flag off, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMCP_SessionInitializeAndListsTools is the contract a cookie-authenticated
// session sees: a valid MCP initialize (serverInfo named leafwiki) and a
// tools/list carrying exactly the sixteen tools (four page reads, eight page
// writes, four revision tools) — the count and the set are both pinned, so a
// seventeenth tool fails this test until the expected list is extended.
func TestMCP_SessionInitializeAndListsTools(t *testing.T) {
	_, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	initResp := decodeMCP(t, initRec)
	if initResp.Error != nil {
		t.Fatalf("initialize returned a JSON-RPC error: %+v", initResp.Error)
	}
	if initResp.Result.ServerInfo.Name != "leafwiki" {
		t.Fatalf("expected serverInfo.name leafwiki, got %q", initResp.Result.ServerInfo.Name)
	}
	if initResp.Result.ProtocolVersion == "" {
		t.Fatalf("expected a protocolVersion in the initialize result")
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	listRec := mcpPost(t, router, mcpToolsListBody, sessionID, &sess)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tools/list, got %d: %s", listRec.Code, listRec.Body.String())
	}
	listResp := decodeMCP(t, listRec)
	if listResp.Error != nil {
		t.Fatalf("tools/list returned a JSON-RPC error: %+v", listResp.Error)
	}
	wantTools := []string{"get_page", "get_page_by_path", "search", "list_tree",
		"create_page", "update_page", "patch_page", "delete_page", "move_page",
		"ensure_path", "convert_node", "copy_page",
		"get_page_revisions", "get_page_revision", "compare_page_revisions", "restore_page_revision"}
	if len(listResp.Result.Tools) != len(wantTools) {
		t.Fatalf("expected exactly %d tools in tools/list, got %d: %+v", len(wantTools), len(listResp.Result.Tools), listResp.Result.Tools)
	}
	got := map[string]bool{}
	for _, tool := range listResp.Result.Tools {
		got[tool.Name] = true
	}
	for _, want := range wantTools {
		if !got[want] {
			t.Fatalf("expected tool %q in tools/list, got %v", want, listResp.Result.Tools)
		}
	}
}

// TestMCP_ReportsBuildVersion pins the version half of serverInfo: the
// initialize response must report the build version passed to the wiki
// (WikiOptions.Version, main.go passes the binary's version), not an empty
// string.
func TestMCP_ReportsBuildVersion(t *testing.T) {
	const version = "1.2.3-mcp-test"
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		EnableMCP:           true,
		Version:             version,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	router := newTestRouter(w, nil)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	initResp := decodeMCP(t, initRec)
	if initResp.Error != nil {
		t.Fatalf("initialize returned a JSON-RPC error: %+v", initResp.Error)
	}
	if got := initResp.Result.ServerInfo.Version; got != version {
		t.Fatalf("expected serverInfo.version %q, got %q", version, got)
	}
}

// TestMCP_DeleteTerminatesSession covers the transport's third verb:
// DELETE /api/mcp ends the session (200), and any request carrying the
// terminated session ID is refused afterwards.
func TestMCP_DeleteTerminatesSession(t *testing.T) {
	_, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	delRec := mcpDelete(t, router, sessionID, &sess)
	if delRec.Code != http.StatusOK {
		t.Fatalf("expected 200 terminating the session, got %d: %.200s", delRec.Code, delRec.Body.String())
	}

	rec := mcpPost(t, router, mcpToolsListBody, sessionID, &sess)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected the terminated session to be rejected with 404, got %d: %.200s", rec.Code, rec.Body.String())
	}
}

// TestMCP_GetOpensSSEStreamAndReturnsOnDisconnect proves the GET verb opens
// the SSE notification stream (200, text/event-stream) and — because the
// stream blocks by design — that the handler returns when the client
// disconnects (net/http cancels the request context), instead of leaking a
// goroutine per connection.
func TestMCP_GetOpensSSEStreamAndReturnsOnDisconnect(t *testing.T) {
	_, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	// Serve the router over a real connection: the client sees the stream
	// headers when the handler writes them, and done closes when the
	// handler goroutine returns.
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		router.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/mcp", nil)
	if err != nil {
		t.Fatalf("build GET request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)
	for _, c := range sess.cookies {
		req.AddCookie(c)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /api/mcp: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(resp.Body.Close, t)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 opening the SSE stream, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected content-type text/event-stream, got %q", ct)
	}

	// Disconnect the client; the handler must give up the stream.
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("GET handler did not return when the client disconnected")
	}
}

// TestMCP_RejectsOversizedRequestBody proves the body cap in
// internal/wiki/mcp routes.go (maxRequestBodyBytes, 500 MiB — the platform's
// bulk-content upload ceiling, strictly tighter than the uncapped REST
// page-update route) bounds mcp-go's unbounded io.ReadAll: a POST body above
// the cap is not slurped in full but answered with a JSON-RPC PARSE_ERROR
// (not a 500, not a hang). Every other test in this file exercises the
// under-cap path through the same handler.
func TestMCP_RejectsOversizedRequestBody(t *testing.T) {
	_, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	// A valid JSON-RPC initialize request padded just over the 500 MiB cap.
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"leafwiki-test","version":"0","note":"` +
		strings.Repeat("x", 500<<20+1) + `"}}}`

	rec := mcpPost(t, router, body, "", &sess)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an oversized body, got %d: %.200s", rec.Code, rec.Body.String())
	}
	resp := decodeMCP(t, rec)
	if resp.Error == nil {
		t.Fatalf("expected a JSON-RPC error for an oversized body, got result %+v", resp.Result)
	}
	if resp.Error.Code != -32700 {
		t.Fatalf("expected PARSE_ERROR (-32700), got %d: %s", resp.Error.Code, resp.Error.Message)
	}
}

// TestMCP_APIKeyAuthenticates proves the endpoint uses the existing API-key
// auth: a valid Bearer key passes the chain (CSRF skipped for resolved keys)
// and gets a session, while a revoked key is rejected with 401 on the very
// next request.
func TestMCP_APIKeyAuthenticates(t *testing.T) {
	_, router := newMCPRouterTest(t, true)

	adminID := getAdminUserIDViaAPI(t, router)
	createBody := `{"name":"mcp key","userId":"` + adminID + `","role":"admin"}`
	createRec := authenticatedRequest(t, router, http.MethodPost, "/api/api-keys", strings.NewReader(createBody))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating key, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Key struct {
			ID string `json:"id"`
		} `json:"key"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	initRec := mcpPostBearer(t, router, mcpInitializeBody, created.Secret, "")
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize with a Bearer key, got %d: %s", initRec.Code, initRec.Body.String())
	}
	initResp := decodeMCP(t, initRec)
	if initResp.Error != nil {
		t.Fatalf("initialize returned a JSON-RPC error: %+v", initResp.Error)
	}
	if initResp.Result.ServerInfo.Name != "leafwiki" {
		t.Fatalf("expected serverInfo.name leafwiki, got %q", initResp.Result.ServerInfo.Name)
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	listRec := mcpPostBearer(t, router, mcpToolsListBody, created.Secret, sessionID)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tools/list with a Bearer key, got %d: %s", listRec.Code, listRec.Body.String())
	}
	if got := decodeMCP(t, listRec); got.Error != nil {
		t.Fatalf("tools/list returned a JSON-RPC error: %+v", got.Error)
	}

	// A revoked key must stop working on the very next request.
	revokeRec := authenticatedRequest(t, router, http.MethodDelete, "/api/api-keys/"+created.Key.ID, nil)
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 revoking key, got %d: %s", revokeRec.Code, revokeRec.Body.String())
	}
	revokedRec := mcpPostBearer(t, router, mcpInitializeBody, created.Secret, "")
	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a revoked key, got %d: %s", revokedRec.Code, revokedRec.Body.String())
	}
}

// TestMCP_RegisteredUnderBasePath proves the endpoint lands under the
// configured base path like every other API route: under the prefix the
// request reaches the auth chain (401 for an unauthenticated caller), without
// it there is no route at all (404).
func TestMCP_RegisteredUnderBasePath(t *testing.T) {
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		EnableMCP:           true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) {
		o.BasePath = "/wiki"
	})

	req := httptest.NewRequest(http.MethodPost, "/wiki/api/mcp", strings.NewReader(mcpInitializeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from the auth chain under the base path, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(mcpInitializeBody)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without the base path, got %d: %s", rec.Code, rec.Body.String())
	}
}

// mcpToolCallResponse is the JSON-RPC envelope of a tools/call response: the
// tool's text content lives in result.content, and a failed call sets
// result.isError (a JSON-RPC error only covers transport-level failures).
type mcpToolCallResponse struct {
	Result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// mcpToolCallText posts a tools/call body and returns the tool's text result,
// failing the test on a non-200, a JSON-RPC error, or an errored tool result.
func mcpToolCallText(t *testing.T, router http.Handler, body, sessionID string, sess *testSession) string {
	t.Helper()
	rec := mcpPost(t, router, body, sessionID, sess)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tools/call, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp mcpToolCallResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode tools/call response %q: %v", rec.Body.String(), err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/call returned a JSON-RPC error: %+v", resp.Error)
	}
	if resp.Result.IsError {
		for _, c := range resp.Result.Content {
			t.Fatalf("tools/call returned an error result: %s", c.Text)
		}
	}
	for _, c := range resp.Result.Content {
		if c.Type == "text" {
			return c.Text
		}
	}
	t.Fatal("tools/call result carried no text content")
	return ""
}

// mcpPageID extracts the stable page ID from a rendered page result (the "ID: "
// line the page tools emit).
func mcpPageID(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if rest, ok := strings.CutPrefix(line, "ID: "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// TestMCP_ToolCallReadsPages is the end-to-end round trip: a cookie-authenticated
// session initializes, lists the tools, then drives the page-read tools against
// the live wiki. get_page_by_path discovers the welcome page (and its stable ID);
// get_page reads that page by ID; search proves a second, non-trivial delegation.
func TestMCP_ToolCallReadsPages(t *testing.T) {
	_, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	const welcomePath = "welcome-to-leafwiki"

	byPathBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_page_by_path","arguments":{"path":%q}}}`,
		welcomePath)
	byPathText := mcpToolCallText(t, router, byPathBody, sessionID, &sess)
	if !strings.Contains(byPathText, "Welcome to LeafWiki") {
		t.Fatalf("get_page_by_path did not return the welcome page:\n%s", byPathText)
	}
	pageID := mcpPageID(byPathText)
	if pageID == "" {
		t.Fatalf("could not extract a page ID from:\n%s", byPathText)
	}

	getPageBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_page","arguments":{"id":%q}}}`,
		pageID)
	getPageText := mcpToolCallText(t, router, getPageBody, sessionID, &sess)
	if !strings.Contains(getPageText, "Welcome to LeafWiki") {
		t.Fatalf("get_page did not return the page content:\n%s", getPageText)
	}

	searchBody := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"search","arguments":{"query":"Welcome"}}}`
	searchText := mcpToolCallText(t, router, searchBody, sessionID, &sess)
	// The title is FTS-highlighted, so assert on the stable route path.
	if !strings.Contains(searchText, "Path: "+welcomePath) {
		t.Fatalf("search did not find the welcome page:\n%s", searchText)
	}
}

// TestMCP_ToolCallMissingPageReportsPageNotFound proves the not-found error
// mapping end to end: a get_page_by_path call for a nonexistent route path
// answers HTTP 200 with an errored tool result (isError) carrying the stable
// page_not_found code, not the generic mcp_internal_error.
func TestMCP_ToolCallMissingPageReportsPageNotFound(t *testing.T) {
	_, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	body := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_page_by_path","arguments":{"path":"no/such/page"}}}`
	rec := mcpPost(t, router, body, sessionID, &sess)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tools/call, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp mcpToolCallResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode tools/call response %q: %v", rec.Body.String(), err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/call returned a JSON-RPC error: %+v", resp.Error)
	}
	if !resp.Result.IsError {
		for _, c := range resp.Result.Content {
			t.Fatalf("expected an errored tool result for a missing page, got success: %s", c.Text)
		}
	}
	var text string
	for _, c := range resp.Result.Content {
		if c.Type == "text" {
			text = c.Text
		}
	}
	if !strings.Contains(text, "page_not_found") {
		t.Fatalf("expected the page_not_found code in the error result, got %q", text)
	}
}

// TestMCP_ToolCallRefusesUnauthenticated proves the auth chain sits in front of
// the tools: a tools/call from a caller with no session is refused with 401 before
// any tool runs (no tool result is produced).
func TestMCP_ToolCallRefusesUnauthenticated(t *testing.T) {
	_, router := newMCPRouterTest(t, false)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_page","arguments":{"id":"nope"}}}`
	rec := mcpPost(t, router, body, "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unauthenticated tools/call, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMCP_ToolCallWritesPages is the write-tool end-to-end round trip. An
// admin (an editor) creates a page through create_page and reads it back
// through get_page; a viewer — the highest role below the write tools' editor
// minimum — is refused with mcp_forbidden and the page is not created; an
// unauthenticated caller is refused with 401 before any tool runs.
func TestMCP_ToolCallWritesPages(t *testing.T) {
	w, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	createBody := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"create_page","arguments":{"title":"MCP E2E Page","content":"written over mcp"}}}`
	createdText := mcpToolCallText(t, router, createBody, sessionID, &sess)
	pageID := mcpPageID(createdText)
	if pageID == "" {
		t.Fatalf("could not extract a page ID from:\n%s", createdText)
	}

	getPageBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_page","arguments":{"id":%q}}}`,
		pageID)
	gotText := mcpToolCallText(t, router, getPageBody, sessionID, &sess)
	if !strings.Contains(gotText, "written over mcp") {
		t.Fatalf("get_page did not return the created content:\n%s", gotText)
	}

	t.Run("a viewer is refused with mcp_forbidden", func(t *testing.T) {
		if _, err := w.UserService().CreateUser("mcp-viewer", "mcp-viewer@example.com", "viewerpass1", coreauth.RoleViewer); err != nil {
			t.Fatalf("CreateUser (viewer) error: %v", err)
		}
		viewer := loginSession(t, router, "mcp-viewer", "viewerpass1")

		viewerInit := mcpPost(t, router, mcpInitializeBody, "", &viewer)
		if viewerInit.Code != http.StatusOK {
			t.Fatalf("expected 200 for the viewer's initialize, got %d: %s", viewerInit.Code, viewerInit.Body.String())
		}
		viewerSession := viewerInit.Header().Get("Mcp-Session-Id")
		if viewerSession == "" {
			t.Fatal("expected the viewer's initialize response to carry a Mcp-Session-Id header")
		}

		// A viewer-distinct title, so "nothing was created" is observable
		// against a path the admin's call above never touched.
		viewerCreateBody := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"create_page","arguments":{"title":"MCP E2E Viewer Page","content":"should not exist"}}}`
		rec := mcpPost(t, router, viewerCreateBody, viewerSession, &viewer)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for an errored tool result, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp mcpToolCallResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode tools/call response %q: %v", rec.Body.String(), err)
		}
		if resp.Error != nil {
			t.Fatalf("tools/call returned a JSON-RPC error: %+v", resp.Error)
		}
		if !resp.Result.IsError {
			t.Fatalf("expected the viewer's create_page to be an errored tool result")
		}
		for _, c := range resp.Result.Content {
			if c.Type == "text" && !strings.Contains(c.Text, "mcp_forbidden") {
				t.Fatalf("expected mcp_forbidden in the error result, got %q", c.Text)
			}
		}
		// The refusal happens before the delegate: the page does not exist.
		byPath := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_page_by_path","arguments":{"path":"mcp-e2e-viewer-page"}}}`
		rec = mcpPost(t, router, byPath, sessionID, &sess)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for the admin's read-back, got %d: %s", rec.Code, rec.Body.String())
		}
		var absent mcpToolCallResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &absent); err != nil {
			t.Fatalf("decode read-back response %q: %v", rec.Body.String(), err)
		}
		if !absent.Result.IsError {
			t.Fatal("expected the viewer's create_page to leave no page behind")
		}
	})

	t.Run("an unauthenticated caller is refused with 401", func(t *testing.T) {
		rec := mcpPost(t, router, createBody, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for an unauthenticated write, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// TestMCP_ToolCallEnsurePath is the tree-tools end-to-end round trip: an admin
// ensures a nested route path over HTTP (missing segments become sections),
// then reads the deepest segment back by path; a viewer is refused with
// mcp_forbidden and nothing is created; an unauthenticated caller is refused
// with 401 before any tool runs.
func TestMCP_ToolCallEnsurePath(t *testing.T) {
	w, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	const ensurePathBody = `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"ensure_path","arguments":{"path":"mcp-e2e/notes/meeting","title":"MCP E2E Meeting"}}}`
	ensuredText := mcpToolCallText(t, router, ensurePathBody, sessionID, &sess)
	if !strings.Contains(ensuredText, "MCP E2E Meeting") {
		t.Fatalf("ensure_path did not return the final node:\n%s", ensuredText)
	}

	// The deepest segment is then readable by its full route path.
	byPathBody := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_page_by_path","arguments":{"path":"mcp-e2e/notes/meeting"}}}`
	byPathText := mcpToolCallText(t, router, byPathBody, sessionID, &sess)
	if !strings.Contains(byPathText, "MCP E2E Meeting") {
		t.Fatalf("get_page_by_path did not return the ensured node:\n%s", byPathText)
	}

	t.Run("a viewer is refused with mcp_forbidden", func(t *testing.T) {
		if _, err := w.UserService().CreateUser("mcp-tree-viewer", "mcp-tree-viewer@example.com", "viewerpass1", coreauth.RoleViewer); err != nil {
			t.Fatalf("CreateUser (viewer) error: %v", err)
		}
		viewer := loginSession(t, router, "mcp-tree-viewer", "viewerpass1")

		viewerInit := mcpPost(t, router, mcpInitializeBody, "", &viewer)
		if viewerInit.Code != http.StatusOK {
			t.Fatalf("expected 200 for the viewer's initialize, got %d: %s", viewerInit.Code, viewerInit.Body.String())
		}
		viewerSession := viewerInit.Header().Get("Mcp-Session-Id")
		if viewerSession == "" {
			t.Fatal("expected the viewer's initialize response to carry a Mcp-Session-Id header")
		}

		viewerBody := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"ensure_path","arguments":{"path":"mcp-e2e/denied","title":"MCP E2E Denied"}}}`
		rec := mcpPost(t, router, viewerBody, viewerSession, &viewer)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for an errored tool result, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp mcpToolCallResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode tools/call response %q: %v", rec.Body.String(), err)
		}
		if resp.Error != nil {
			t.Fatalf("tools/call returned a JSON-RPC error: %+v", resp.Error)
		}
		if !resp.Result.IsError {
			t.Fatalf("expected the viewer's ensure_path to be an errored tool result")
		}
		for _, c := range resp.Result.Content {
			if c.Type == "text" && !strings.Contains(c.Text, "mcp_forbidden") {
				t.Fatalf("expected mcp_forbidden in the error result, got %q", c.Text)
			}
		}
		// The refusal happens before the delegate: the path does not exist.
		absentBody := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_page_by_path","arguments":{"path":"mcp-e2e/denied"}}}`
		rec = mcpPost(t, router, absentBody, sessionID, &sess)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for the admin's read-back, got %d: %s", rec.Code, rec.Body.String())
		}
		var absent mcpToolCallResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &absent); err != nil {
			t.Fatalf("decode read-back response %q: %v", rec.Body.String(), err)
		}
		if !absent.Result.IsError {
			t.Fatal("expected the viewer's ensure_path to leave no path behind")
		}
	})

	t.Run("an unauthenticated caller is refused with 401", func(t *testing.T) {
		rec := mcpPost(t, router, ensurePathBody, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for an unauthenticated ensure_path, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// mcpRevisionIDs extracts the revision IDs from a get_page_revisions result:
// each entry line starts with "- <ID> …" (the rest carries type, author and
// time, which the test does not need).
func mcpRevisionIDs(text string) []string {
	var ids []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "- "))
		if len(fields) > 0 {
			ids = append(ids, fields[0])
		}
	}
	return ids
}

// TestMCP_ToolCallRevisions is the revision-tools end-to-end round trip: an
// admin creates a page and saves a second version, the history lists the
// revisions newest first, and restoring an older revision reverts the page to
// that content — the restore itself is recorded, so the history grows by one
// with a new "restore"-type entry. A viewer — the highest role below the
// restore tool's editor minimum — is refused with mcp_forbidden and the
// refusal is a no-op: the page is unchanged and no restore revision is
// recorded.
func TestMCP_ToolCallRevisions(t *testing.T) {
	w, router := newMCPRouterTest(t, false)
	sess := loginSession(t, router, "admin", "adminpassword")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sess)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	const (
		versionOne = "revisions e2e version 1"
		versionTwo = "revisions e2e version 2"
	)
	createBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"create_page","arguments":{"title":"MCP Revisions E2E","content":%q}}}`,
		versionOne)
	createdText := mcpToolCallText(t, router, createBody, sessionID, &sess)
	pageID := mcpPageID(createdText)
	if pageID == "" {
		t.Fatalf("could not extract a page ID from:\n%s", createdText)
	}

	updateBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"update_page","arguments":{"id":%q,"content":%q}}}`,
		pageID, versionTwo)
	mcpToolCallText(t, router, updateBody, sessionID, &sess)

	listBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_page_revisions","arguments":{"id":%q}}}`,
		pageID)
	listText := mcpToolCallText(t, router, listBody, sessionID, &sess)
	ids := mcpRevisionIDs(listText)
	if len(ids) < 2 {
		t.Fatalf("expected at least two revisions after a create and an update, got %d:\n%s", len(ids), listText)
	}

	// The list lines do not carry content, so read each entry back (newest
	// first) to find the revision holding version 1.
	restoreID := ""
	for _, id := range ids {
		getBody := fmt.Sprintf(
			`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_page_revision","arguments":{"id":%q,"revision_id":%q}}}`,
			pageID, id)
		revText := mcpToolCallText(t, router, getBody, sessionID, &sess)
		if strings.Contains(revText, versionOne) {
			restoreID = id
			break
		}
	}
	if restoreID == "" {
		t.Fatalf("no listed revision holds the first content:\n%s", listText)
	}

	restoreBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"restore_page_revision","arguments":{"id":%q,"revision_id":%q}}}`,
		pageID, restoreID)
	mcpToolCallText(t, router, restoreBody, sessionID, &sess)

	getPageBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"get_page","arguments":{"id":%q}}}`,
		pageID)
	restoredText := mcpToolCallText(t, router, getPageBody, sessionID, &sess)
	if !strings.Contains(restoredText, versionOne) || strings.Contains(restoredText, versionTwo) {
		t.Fatalf("the restored page must carry the first content only:\n%s", restoredText)
	}

	// The restore itself is recorded: one more entry, a restore-type one.
	listText = mcpToolCallText(t, router, listBody, sessionID, &sess)
	newIDs := mcpRevisionIDs(listText)
	if len(newIDs) != len(ids)+1 {
		t.Fatalf("expected the restore to add one revision (%d + 1), got %d:\n%s", len(ids), len(newIDs), listText)
	}
	if !strings.Contains(listText, "(restore)") {
		t.Fatalf("expected the recorded restore revision in the history:\n%s", listText)
	}

	t.Run("a viewer is refused with mcp_forbidden", func(t *testing.T) {
		if _, err := w.UserService().CreateUser("mcp-rev-viewer", "mcp-rev-viewer@example.com", "viewerpass1", coreauth.RoleViewer); err != nil {
			t.Fatalf("CreateUser (viewer) error: %v", err)
		}
		viewer := loginSession(t, router, "mcp-rev-viewer", "viewerpass1")

		viewerInit := mcpPost(t, router, mcpInitializeBody, "", &viewer)
		if viewerInit.Code != http.StatusOK {
			t.Fatalf("expected 200 for the viewer's initialize, got %d: %s", viewerInit.Code, viewerInit.Body.String())
		}
		viewerSession := viewerInit.Header().Get("Mcp-Session-Id")
		if viewerSession == "" {
			t.Fatal("expected the viewer's initialize response to carry a Mcp-Session-Id header")
		}

		viewerRestoreBody := fmt.Sprintf(
			`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"restore_page_revision","arguments":{"id":%q,"revision_id":%q}}}`,
			pageID, restoreID)
		rec := mcpPost(t, router, viewerRestoreBody, viewerSession, &viewer)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for an errored tool result, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp mcpToolCallResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode tools/call response %q: %v", rec.Body.String(), err)
		}
		if resp.Error != nil {
			t.Fatalf("tools/call returned a JSON-RPC error: %+v", resp.Error)
		}
		if !resp.Result.IsError {
			t.Fatal("expected the viewer's restore_page_revision to be an errored tool result")
		}
		for _, c := range resp.Result.Content {
			if c.Type == "text" && !strings.Contains(c.Text, "mcp_forbidden") {
				t.Fatalf("expected mcp_forbidden in the error result, got %q", c.Text)
			}
		}
		// The refusal happens before the delegate: no restore side effect.
		// The page keeps the restored content…
		gotText := mcpToolCallText(t, router, getPageBody, sessionID, &sess)
		if strings.Contains(gotText, versionTwo) {
			t.Fatal("the viewer's refused restore must not change the page")
		}
		// …and no restore entry was recorded — a successful (but refused
		// gate-bypassing) restore would grow the history by one.
		afterRefusal := mcpToolCallText(t, router, listBody, sessionID, &sess)
		if got := mcpRevisionIDs(afterRefusal); len(got) != len(newIDs) {
			t.Fatalf("the refused restore must not record a revision: history went from %d to %d:\n%s",
				len(newIDs), len(got), afterRefusal)
		}
	})
}
