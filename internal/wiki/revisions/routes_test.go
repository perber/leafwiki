package revisions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/test_utils"
)

// Router options for the access modes the route tests use. The revision
// routes exist only with EnableRevision.
var (
	// signedIn injects the public editor, as with authentication disabled.
	signedIn = httpinternal.RouterOptions{AuthDisabled: true, EnableRevision: true}
	// noSession has no session and public access off.
	noSession = httpinternal.RouterOptions{EnableRevision: true}
	// publicRead has no session and public access on.
	publicRead = httpinternal.RouterOptions{PublicAccess: publicaccess.Fixed(true), EnableRevision: true}
)

const (
	rev1Content = "# Page\nFirst version."
	rev2Content = "# Page\nSecond version with a diagram."
	assetName   = "diagram.png"
	assetBytes  = "\x89PNG fake diagram"
)

// revisionsFixture is the real router over two pages: pageID has two
// revisions, rev1 (content only) and rev2 (new content plus diagram.png);
// emptyPageID has none.
type revisionsFixture struct {
	router              http.Handler
	pageID, emptyPageID string
	rev1, rev2          string
	viewerToken         string
}

func newRevisionsRouter(t *testing.T, opts httpinternal.RouterOptions) *revisionsFixture {
	t.Helper()
	dir := t.TempDir()

	ts := tree.NewTreeService(dir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	f := &revisionsFixture{}
	kind := tree.NodeKindPage
	for _, p := range []struct {
		id          *string
		title, slug string
	}{
		{&f.pageID, "Page", "page"},
		{&f.emptyPageID, "Empty", "empty"},
	} {
		id, err := ts.CreateNode("system", nil, p.title, p.slug, &kind)
		if err != nil {
			t.Fatalf("CreateNode %q: %v", p.slug, err)
		}
		*p.id = *id
	}
	setContent := func(content string) {
		t.Helper()
		if err := ts.UpdateNode(tree.UpdateNodeInput{UserID: "system", ID: f.pageID, Title: "Page", Slug: "page", Content: &content, ExpectedVersion: tree.VersionUnchecked}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	}

	// No coalesce window, so the second save is a revision of its own.
	revService := revision.NewService(dir, ts, nil, revision.ServiceOptions{})
	setContent(rev1Content)
	rev1, created, err := revService.RecordContentUpdate(f.pageID, "system", "")
	if err != nil || !created {
		t.Fatalf("RecordContentUpdate: created=%v err=%v", created, err)
	}
	// Revision files are named and sorted by their creation time; wait for
	// the clock to move on so rev2 is the newer one on every platform.
	for !time.Now().After(rev1.CreatedAt) {
		time.Sleep(time.Millisecond)
	}
	// The asset is written after rev1, so rev1 has no assets and rev2 adds one.
	setContent(rev2Content)
	assetDir := filepath.Join(dir, "assets", f.pageID)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, assetName), []byte(assetBytes), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	rev2, created, err := revService.RecordAssetChange(f.pageID, "system", "")
	if err != nil || !created {
		t.Fatalf("RecordAssetChange: created=%v err=%v", created, err)
	}
	f.rev1, f.rev2 = rev1.ID, rev2.ID

	userStore, err := coreauth.NewUserStore(dir)
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	sessionStore, err := coreauth.NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	sessions := coreauth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	userService := coreauth.NewUserService(userStore)
	authService := coreauth.NewAuthService(userService, sessions, nil)
	t.Cleanup(func() { _ = authService.Close() })

	// A real viewer session: the lowest role, so a 200 proves the reads need
	// no editor rights.
	f.viewerToken = createUserAndLogin(t, userService, authService, "viewer", coreauth.RoleViewer, "route-test-password-1")

	// Restore and integrity are never called by these tests.
	routes := NewRoutes(RoutesConfig{
		ListRevisions:    NewListRevisionsUseCase(revService),
		GetRevision:      NewGetRevisionUseCase(revService),
		CompareRevisions: NewCompareRevisionsUseCase(revService),
		GetRevisionAsset: NewGetRevisionAssetUseCase(revService),
		GetLatest:        NewGetLatestRevisionUseCase(revService),
		AuthService:      authService,
	})
	opts.DisableRequestLog = true
	// httptest requests are plain HTTP; without this the cookie is never read
	// and every 401 below would mean "HTTPS required", not "no session".
	opts.AllowInsecure = true
	f.router = httpinternal.NewRouter([]httpinternal.RouteRegistrar{routes}, httpinternal.FrontendConfig{}, opts)
	return f
}

func createUserAndLogin(t *testing.T, users *coreauth.UserService, auth *coreauth.AuthService, name, role, password string) string {
	t.Helper()
	if _, err := users.CreateUser(name, name+"@example.com", password, role); err != nil {
		t.Fatalf("CreateUser %q: %v", name, err)
	}
	token, err := auth.Login(name, password, "")
	if err != nil {
		t.Fatalf("Login %q: %v", name, err)
	}
	return token.Token
}

// get sends a GET request, with the access token as session cookie unless it
// is empty.
func (f *revisionsFixture) get(url, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "leafwiki_at", Value: token})
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f *revisionsFixture) revisionsURL(pageID string) string {
	return "/api/pages/" + pageID + "/revisions"
}

// assertResponse fails unless rec has the status code and exactly the body.
func assertResponse(t *testing.T, rec *httptest.ResponseRecorder, wantCode int, wantBody string) {
	t.Helper()
	if rec.Code != wantCode {
		t.Errorf("status code = %d, want %d; body=%s", rec.Code, wantCode, rec.Body.String())
	}
	if got := rec.Body.String(); got != wantBody {
		t.Errorf("body = %s, want %s", got, wantBody)
	}
}

// storeNotFoundBody is the 404 that respondWithRevisionError builds from the
// store's os.ErrNotExist (unknown revision): no args, unlike the
// handler-built "Revision not found".
const storeNotFoundBody = `{"error":{"code":"revision_not_found","message":"Revision resource not found","template":"revision resource not found"}}`

// revisionRequiredKeys are the RevisionResponse keys without omitempty;
// revisionAllowedKeys are all of its JSON tags.
var (
	revisionRequiredKeys = []string{"id", "pageId", "type", "authorId", "createdAt", "title", "slug", "kind", "path", "contentHash", "assetManifestHash"}
	revisionAllowedKeys  = map[string]bool{
		"id": true, "pageId": true, "parentId": true, "type": true, "authorId": true, "author": true,
		"createdAt": true, "title": true, "slug": true, "kind": true, "path": true, "contentHash": true,
		"assetManifestHash": true, "pageCreatedAt": true, "pageUpdatedAt": true, "creatorId": true,
		"lastAuthorId": true, "summary": true,
	}
)

// assertRevision fails unless raw is a revision object with every required
// key, no key outside RevisionResponse, and the given ID.
func assertRevision(t *testing.T, raw json.RawMessage, wantID string) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("not a JSON object: %s: %v", raw, err)
	}
	for _, key := range revisionRequiredKeys {
		if _, ok := obj[key]; !ok {
			t.Errorf("missing key %q in %s", key, raw)
		}
	}
	for key := range obj {
		if !revisionAllowedKeys[key] {
			t.Errorf("unexpected key %q in %s", key, raw)
		}
	}
	var rev struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &rev); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if rev.ID != wantID {
		t.Errorf("id = %q, want %q", rev.ID, wantID)
	}
}

// listBody decodes a 200 list response.
type listBody struct {
	NextCursor string            `json:"nextCursor"`
	Revisions  []json.RawMessage `json:"revisions"`
}

func decodeList(t *testing.T, rec *httptest.ResponseRecorder) listBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body listBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return body
}

// TestRevisionRoutes_List_WireShape pins GET /api/pages/{id}/revisions: the
// two top-level keys, newest first, and the revision keys.
func TestRevisionRoutes_List_WireShape(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)

	rec := f.get(f.revisionsURL(f.pageID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{"nextCursor": "string", "revisions": "array"})
	body := decodeList(t, rec)
	if len(body.Revisions) != 2 {
		t.Fatalf("revisions = %d, want 2; body=%s", len(body.Revisions), rec.Body.String())
	}
	assertRevision(t, body.Revisions[0], f.rev2)
	assertRevision(t, body.Revisions[1], f.rev1)
	if body.NextCursor != "" {
		t.Errorf("nextCursor = %q, want empty on the last page", body.NextCursor)
	}
}

// TestRevisionRoutes_List_Paging pins the cursor: limit cuts the page, the
// cursor continues after it, and an unknown cursor or a page without
// revisions is an empty list with 200, never null and never 404.
func TestRevisionRoutes_List_Paging(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)

	first := decodeList(t, f.get(f.revisionsURL(f.pageID)+"?limit=1", ""))
	if len(first.Revisions) != 1 || first.NextCursor == "" {
		t.Fatalf("first page = %d revisions, nextCursor %q; want 1 and a cursor", len(first.Revisions), first.NextCursor)
	}
	assertRevision(t, first.Revisions[0], f.rev2)

	second := decodeList(t, f.get(f.revisionsURL(f.pageID)+"?cursor="+url.QueryEscape(first.NextCursor), ""))
	if len(second.Revisions) != 1 || second.NextCursor != "" {
		t.Fatalf("second page = %d revisions, nextCursor %q; want 1 and none", len(second.Revisions), second.NextCursor)
	}
	assertRevision(t, second.Revisions[0], f.rev1)

	const empty = `{"nextCursor":"","revisions":[]}`
	assertResponse(t, f.get(f.revisionsURL(f.pageID)+"?cursor=unknown", ""), http.StatusOK, empty)
	assertResponse(t, f.get(f.revisionsURL(f.emptyPageID), ""), http.StatusOK, empty)
}

// TestRevisionRoutes_List_Limit pins the limit bounds: 1..200, blank means
// the default, anything else is a 400 naming the page.
func TestRevisionRoutes_List_Limit(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)
	invalid := `{"error":{"code":"revision_invalid_limit","message":"Revision list limit is invalid","template":"revision list limit for page %s is invalid","args":["` + f.pageID + `"]}}`

	tests := []struct {
		limit    string
		wantCode int
	}{
		{"0", http.StatusBadRequest},
		{"-1", http.StatusBadRequest},
		{"201", http.StatusBadRequest},
		{"abc", http.StatusBadRequest},
		{"", http.StatusOK},
		{"1", http.StatusOK},
		{"200", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run("limit="+tt.limit, func(t *testing.T) {
			rec := f.get(f.revisionsURL(f.pageID)+"?limit="+tt.limit, "")
			if tt.wantCode == http.StatusOK {
				decodeList(t, rec)
				return
			}
			assertResponse(t, rec, tt.wantCode, invalid)
		})
	}
}

// TestRevisionRoutes_Latest pins GET …/revisions/latest: the newest revision,
// and the handler-built 404 (with the page as arg) for a page without one.
func TestRevisionRoutes_Latest(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)

	rec := f.get(f.revisionsURL(f.pageID)+"/latest", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	assertRevision(t, rec.Body.Bytes(), f.rev2)

	assertResponse(t, f.get(f.revisionsURL(f.emptyPageID)+"/latest", ""), http.StatusNotFound,
		`{"error":{"code":"revision_not_found","message":"Revision not found","template":"revision for page %s not found","args":["`+f.emptyPageID+`"]}}`)
}

// TestRevisionRoutes_Get pins GET …/revisions/{revisionId}: the snapshot
// keys, assets as [] when there are none, the asset keys, and the
// store-built 404 for an unknown revision.
func TestRevisionRoutes_Get(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)

	snapshot := func(t *testing.T, revID string) map[string]json.RawMessage {
		t.Helper()
		rec := f.get(f.revisionsURL(f.pageID)+"/"+revID, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{"revision": "object", "content": "string", "assets": "array"})
		var body map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", rec.Body.String(), err)
		}
		assertRevision(t, body["revision"], revID)
		return body
	}

	t.Run("revision without assets", func(t *testing.T) {
		body := snapshot(t, f.rev1)
		if got := string(body["assets"]); got != "[]" {
			t.Errorf("assets = %s, want []", got)
		}
		var content string
		if err := json.Unmarshal(body["content"], &content); err != nil || content != rev1Content {
			t.Errorf("content = %q (err %v), want %q", content, err, rev1Content)
		}
	})
	t.Run("revision with an asset", func(t *testing.T) {
		body := snapshot(t, f.rev2)
		var assets []json.RawMessage
		if err := json.Unmarshal(body["assets"], &assets); err != nil {
			t.Fatalf("decode assets %s: %v", body["assets"], err)
		}
		if len(assets) != 1 {
			t.Fatalf("assets = %s, want one", body["assets"])
		}
		test_utils.AssertJSONObjectShape(t, assets[0], map[string]string{"name": "string", "sha256": "string", "sizeBytes": "number", "mimeType": "string"})
	})
	t.Run("unknown revision", func(t *testing.T) {
		assertResponse(t, f.get(f.revisionsURL(f.pageID)+"/unknown", ""), http.StatusNotFound, storeNotFoundBody)
	})
}

// TestRevisionRoutes_Compare pins GET …/revisions/compare: the comparison
// keys and values, the 400 for a missing side, and the store-built 404.
func TestRevisionRoutes_Compare(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)
	compareURL := func(base, target string) string {
		return f.revisionsURL(f.pageID) + "/compare?base=" + base + "&target=" + target
	}

	rec := f.get(compareURL(f.rev1, f.rev2), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{"base": "object", "target": "object", "contentChanged": "bool", "assetChanges": "array"})
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	if got := string(body["contentChanged"]); got != "true" {
		t.Errorf("contentChanged = %s, want true", got)
	}
	if got, want := string(body["assetChanges"]), `[{"name":"diagram.png","status":"added"}]`; got != want {
		t.Errorf("assetChanges = %s, want %s", got, want)
	}
	for _, side := range []string{"base", "target"} {
		test_utils.AssertJSONObjectShape(t, body[side], map[string]string{"revision": "object", "content": "string", "assets": "array"})
	}

	t.Run("missing target", func(t *testing.T) {
		assertResponse(t, f.get(f.revisionsURL(f.pageID)+"/compare?base="+f.rev1, ""), http.StatusBadRequest,
			`{"error":{"code":"revision_compare_invalid_request","message":"Revision compare request is invalid","template":"revision compare request for page %s is invalid","args":["`+f.pageID+`"]}}`)
	})
	t.Run("unknown base", func(t *testing.T) {
		assertResponse(t, f.get(compareURL("unknown", f.rev2), ""), http.StatusNotFound, storeNotFoundBody)
	})
}

// TestRevisionRoutes_Asset pins GET …/revisions/{revisionId}/assets/{name}:
// the file bytes, and the bodies of the three failures. The download headers
// are pinned in routes_asset_test.go.
func TestRevisionRoutes_Asset(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)
	assetsURL := func(revID string) string { return f.revisionsURL(f.pageID) + "/" + revID + "/assets/" }

	t.Run("asset", func(t *testing.T) {
		assertResponse(t, f.get(assetsURL(f.rev2)+assetName, ""), http.StatusOK, assetBytes)
	})
	t.Run("empty name", func(t *testing.T) {
		assertResponse(t, f.get(assetsURL(f.rev2), ""), http.StatusBadRequest,
			`{"error":{"code":"revision_preview_asset_invalid_name","message":"Revision asset name is invalid","template":"revision asset name for page %s revision %s is invalid","args":["`+f.pageID+`","`+f.rev2+`"]}}`)
	})
	t.Run("unknown name", func(t *testing.T) {
		assertResponse(t, f.get(assetsURL(f.rev2)+"missing.png", ""), http.StatusNotFound,
			`{"error":{"code":"revision_preview_asset_not_found","message":"Revision asset not found","template":"revision asset %s for page %s revision %s not found","args":["missing.png","`+f.pageID+`","`+f.rev2+`"]}}`)
	})
	t.Run("unknown revision", func(t *testing.T) {
		assertResponse(t, f.get(assetsURL("unknown")+assetName, ""), http.StatusNotFound, storeNotFoundBody)
	})
}

// readURLs are the five read routes of f, each answering 200 when allowed.
func (f *revisionsFixture) readURLs() map[string]string {
	base := f.revisionsURL(f.pageID)
	return map[string]string{
		"list":    base,
		"latest":  base + "/latest",
		"compare": base + "/compare?base=" + f.rev1 + "&target=" + f.rev2,
		"get":     base + "/" + f.rev1,
		"asset":   base + "/" + f.rev2 + "/assets/" + assetName,
	}
}

// TestRevisionRoutes_Access pins the gate of all five reads: any signed-in
// role, no anonymous read even with public access on (APIAuthGroup, not
// APIReadGroup).
func TestRevisionRoutes_Access(t *testing.T) {
	anonymous := newRevisionsRouter(t, noSession)
	public := newRevisionsRouter(t, publicRead)
	authDisabled := newRevisionsRouter(t, signedIn)

	tests := []struct {
		name     string
		fixture  *revisionsFixture
		viewer   bool
		wantCode int
	}{
		{"without a session", anonymous, false, http.StatusUnauthorized},
		{"without a session and with public access", public, false, http.StatusUnauthorized},
		{"as viewer", anonymous, true, http.StatusOK},
		{"with authentication disabled", authDisabled, false, http.StatusOK},
	}
	for _, tt := range tests {
		for route, u := range tt.fixture.readURLs() {
			t.Run(route+" "+tt.name, func(t *testing.T) {
				token := ""
				if tt.viewer {
					token = tt.fixture.viewerToken
				}
				rec := tt.fixture.get(u, token)
				if rec.Code != tt.wantCode {
					t.Fatalf("status code = %d, want %d; body=%s", rec.Code, tt.wantCode, rec.Body.String())
				}
				if tt.wantCode == http.StatusUnauthorized {
					// The auth middleware's flat {"error": string}, not the
					// localized envelope.
					test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{"error": "string"})
				}
			})
		}
	}
}

// TestRevisionRoutes_DisabledFlag pins that without EnableRevision the read
// routes are not registered at all.
func TestRevisionRoutes_DisabledFlag(t *testing.T) {
	f := newRevisionsRouter(t, httpinternal.RouterOptions{AuthDisabled: true})

	for route, u := range f.readURLs() {
		t.Run(route, func(t *testing.T) {
			if rec := f.get(u, ""); rec.Code != http.StatusNotFound {
				t.Errorf("status code = %d, want 404; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestRevisionRoutes_EmptyPathSegments pins what an empty or blank path
// segment answers today. The router (gin.New, default RedirectTrailingSlash)
// still matches an empty :id, so revision_invalid_page_id is reachable on all
// five reads. revision_invalid_revision_id is reachable on the asset route
// (empty or blank segment) and on get only with a blank segment: an empty one
// ("…/revisions//") is redirected before any handler runs.
func TestRevisionRoutes_EmptyPathSegments(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)
	const (
		pageIDRequired     = `{"error":{"code":"revision_invalid_page_id","message":"Page ID is required","template":"page id is required"}}`
		revisionIDRequired = `{"error":{"code":"revision_invalid_revision_id","message":"Revision ID is required","template":"revision id is required"}}`
	)
	base := f.revisionsURL(f.pageID)

	tests := []struct {
		name         string
		url          string
		wantCode     int
		wantBody     string
		wantLocation string
	}{
		{"list, empty page ID", "/api/pages//revisions", http.StatusBadRequest, pageIDRequired, ""},
		{"latest, empty page ID", "/api/pages//revisions/latest", http.StatusBadRequest, pageIDRequired, ""},
		{"compare, empty page ID", "/api/pages//revisions/compare?base=" + f.rev1 + "&target=" + f.rev2, http.StatusBadRequest, pageIDRequired, ""},
		{"get, empty page ID", "/api/pages//revisions/" + f.rev1, http.StatusBadRequest, pageIDRequired, ""},
		{"asset, empty page ID", "/api/pages//revisions/" + f.rev2 + "/assets/" + assetName, http.StatusBadRequest, pageIDRequired, ""},
		{"list, blank page ID", "/api/pages/%20/revisions", http.StatusBadRequest, pageIDRequired, ""},
		{"get, empty revision ID", base + "//", http.StatusMovedPermanently,
			`<a href="` + base + `/">Moved Permanently</a>.` + "\n\n", base + "/"},
		{"get, blank revision ID", base + "/%20", http.StatusBadRequest, revisionIDRequired, ""},
		{"asset, empty revision ID", base + "//assets/" + assetName, http.StatusBadRequest, revisionIDRequired, ""},
		{"asset, blank revision ID", base + "/%20/assets/" + assetName, http.StatusBadRequest, revisionIDRequired, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := f.get(tt.url, "")
			assertResponse(t, rec, tt.wantCode, tt.wantBody)
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}
}

// TestRevisionRoutes_MalformedPageID pins that a page ID the store rejects
// (path separator or dot component) answers 500, not 400: validateStorageID
// returns a plain error, which respondWithRevisionError maps to its default.
// Reported on #1661, not fixed here.
func TestRevisionRoutes_MalformedPageID(t *testing.T) {
	f := newRevisionsRouter(t, signedIn)
	const internalError = `{"error":{"code":"revision_internal_error","message":"Revision request failed","template":"revision request failed"}}`

	// gin.New matches on the decoded URL.Path, so %5C reaches :id as a backslash.
	for _, pageID := range []string{"a%5Cb", "."} {
		t.Run(pageID, func(t *testing.T) {
			assertResponse(t, f.get(f.revisionsURL(pageID), ""), http.StatusInternalServerError, internalError)
		})
	}
}
