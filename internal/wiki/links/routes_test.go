package links

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	corelinks "github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/test_utils"
)

// Router options for the access modes the route tests use.
var (
	// signedIn injects the public editor, as with authentication disabled.
	signedIn = httpinternal.RouterOptions{AuthDisabled: true}
	// noSession has no session and public access off.
	noSession = httpinternal.RouterOptions{}
	// publicRead has no session and public access on.
	publicRead = httpinternal.RouterOptions{PublicAccess: publicaccess.Fixed(true)}
)

// linksFixture is the real router over three pages: A links to B and to a
// page that does not exist, B is linked from A, C has no links in or out.
type linksFixture struct {
	router                  http.Handler
	pageA, pageB, pageC     string
	adminToken, editorToken string

	tree  *tree.TreeService
	links *corelinks.LinkService
}

func newLinksRouter(t *testing.T, opts httpinternal.RouterOptions) *linksFixture {
	t.Helper()
	dir := t.TempDir()

	ts := tree.NewTreeService(dir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	store, err := corelinks.NewLinksStore(dir)
	if err != nil {
		t.Fatalf("NewLinksStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	linkSvc := corelinks.NewLinkService(dir, ts, store)

	f := &linksFixture{tree: ts, links: linkSvc}
	for _, p := range []struct {
		id          *string
		title, slug string
		content     string
	}{
		{&f.pageA, "Page A", "a", "Link to B: [Go to B](/b) and to [a missing page](/missing)."},
		{&f.pageB, "Page B", "b", "# Page B\nNo outgoing links."},
		{&f.pageC, "Page C", "c", "# Page C\nNo links at all."},
	} {
		kind := tree.NodeKindPage
		id, err := ts.CreateNode("system", nil, p.title, p.slug, &kind)
		if err != nil {
			t.Fatalf("CreateNode %q: %v", p.slug, err)
		}
		content := p.content
		if err := ts.UpdateNode(tree.UpdateNodeInput{UserID: "system", ID: *id, Title: p.title, Slug: p.slug, Content: &content, ExpectedVersion: tree.VersionUnchecked}); err != nil {
			t.Fatalf("UpdateNode %q: %v", p.slug, err)
		}
		*p.id = *id
	}
	if err := linkSvc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

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

	// Real users and sessions, so a 200 for the admin proves the role check
	// and not a disabled gate (RequireAdmin always answers 403 then).
	const password = "route-test-password-1"
	f.adminToken = createUserAndLogin(t, userService, authService, "admin", coreauth.RoleAdmin, password)
	f.editorToken = createUserAndLogin(t, userService, authService, "editor", coreauth.RoleEditor, password)

	routes := NewRoutes(RoutesConfig{
		GetLinkStatus:  NewGetLinkStatusUseCase(linkSvc, ts),
		GetBrokenLinks: NewGetBrokenLinksUseCase(linkSvc),
		AuthService:    authService,
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
	token, err := auth.Login(name, password)
	if err != nil {
		t.Fatalf("Login %q: %v", name, err)
	}
	return token.Token
}

// fixBrokenLink rewrites page A without the link to the missing page.
func (f *linksFixture) fixBrokenLink(t *testing.T) {
	t.Helper()
	content := "Link to B: [Go to B](/b)."
	if err := f.tree.UpdateNode(tree.UpdateNodeInput{UserID: "system", ID: f.pageA, Title: "Page A", Slug: "a", Content: &content, ExpectedVersion: tree.VersionUnchecked}); err != nil {
		t.Fatalf("UpdateNode a: %v", err)
	}
	if err := f.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}
}

// get sends a GET request, with the access token as session cookie unless it
// is empty.
func (f *linksFixture) get(url, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "leafwiki_at", Value: token})
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

// itemShapes of the link lists.
var (
	backlinkItem = map[string]string{"from_page_id": "string", "from_title": "string", "from_path": "string", "broken": "bool", "to_page_id": "string"}
	outgoingItem = map[string]string{"to_page_id": "string", "to_page_title": "string", "to_path": "string", "broken": "bool", "from_page_id": "string"}
)

// assertListKeys checks that each key of body holds the JSON kind in want and,
// for a non-empty array, that every element has the given item shape.
func assertListKeys(t *testing.T, body map[string]json.RawMessage, want map[string]string, itemShapes map[string]map[string]string) {
	t.Helper()
	for key, kind := range want {
		raw, ok := body[key]
		if !ok {
			t.Errorf("missing key %q", key)
			continue
		}
		if got := test_utils.JSONKind(t, raw); got != kind {
			t.Errorf("%s is a JSON %s, want %s", key, got, kind)
			continue
		}
		if kind != "array" {
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("decode %s: %v", key, err)
		}
		for _, item := range items {
			test_utils.AssertJSONObjectShape(t, item, itemShapes[key])
		}
	}
}

// TestLinksRoutes_LinkStatus_PreservesWireShape pins the body of
// GET /api/pages/{id}/links: the five keys, the item keys, and that an empty
// list is [] and never null.
func TestLinksRoutes_LinkStatus_PreservesWireShape(t *testing.T) {
	f := newLinksRouter(t, signedIn)
	counts := map[string]string{"backlinks": "number", "broken_incoming": "number", "outgoings": "number", "broken_outgoings": "number"}
	items := map[string]map[string]string{
		"backlinks": backlinkItem, "broken_incoming": backlinkItem,
		"outgoings": outgoingItem, "broken_outgoings": outgoingItem,
	}

	// Every list is an array, also when it is empty.
	allArrays := map[string]string{"backlinks": "array", "broken_incoming": "array", "outgoings": "array", "broken_outgoings": "array"}
	tests := []struct {
		name string
		page string
		want map[string]string
	}{
		{"page with a backlink", f.pageB, allArrays},
		{"page with a valid and a broken outgoing link", f.pageA, allArrays},
		{"page without links", f.pageC, allArrays},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := f.get("/api/pages/"+tt.page+"/links", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{
				"backlinks": tt.want["backlinks"], "broken_incoming": tt.want["broken_incoming"],
				"outgoings": "array", "broken_outgoings": "array", "counts": "object",
			})
			var body map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %s: %v", rec.Body.String(), err)
			}
			assertListKeys(t, body, tt.want, items)
			test_utils.AssertJSONObjectShape(t, body["counts"], counts)
		})
	}
}

// TestLinksRoutes_LinkStatus_ListsTheLinksOfThePage pins the list contents
// the shape test leaves open: which side of the link each list sits on.
func TestLinksRoutes_LinkStatus_ListsTheLinksOfThePage(t *testing.T) {
	f := newLinksRouter(t, signedIn)

	decode := func(t *testing.T, page string) corelinks.LinkStatusResult {
		t.Helper()
		rec := f.get("/api/pages/"+page+"/links", "")
		var body corelinks.LinkStatusResult
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", rec.Body.String(), err)
		}
		return body
	}

	b := decode(t, f.pageB)
	if len(b.Backlinks) != 1 || b.Backlinks[0].FromPageID != f.pageA || b.Backlinks[0].ToPageID != f.pageB || b.Counts.Backlinks != 1 {
		t.Errorf("page B backlinks = %+v (counts %+v), want one from A", b.Backlinks, b.Counts)
	}

	a := decode(t, f.pageA)
	if len(a.Outgoings) != 1 || a.Outgoings[0].ToPageID != f.pageB || a.Outgoings[0].Broken {
		t.Errorf("page A outgoings = %+v, want one valid link to B", a.Outgoings)
	}
	if len(a.BrokenOutgoings) != 1 || !a.BrokenOutgoings[0].Broken || a.BrokenOutgoings[0].ToPath != "/missing" {
		t.Errorf("page A broken outgoings = %+v, want one broken link to /missing", a.BrokenOutgoings)
	}
	if a.Counts.Outgoings != 1 || a.Counts.BrokenOutgoings != 1 {
		t.Errorf("page A counts = %+v, want 1 outgoing and 1 broken outgoing", a.Counts)
	}
}

// TestLinksRoutes_LinkStatus_UnknownPage_ReturnsExactErrorBody pins the
// not-found body so the localized envelope stays wire-compatible.
func TestLinksRoutes_LinkStatus_UnknownPage_ReturnsExactErrorBody(t *testing.T) {
	f := newLinksRouter(t, signedIn)

	rec := f.get("/api/pages/does-not-exist/links", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	want := `{"error":{"code":"link_page_not_found","message":"Page not found","template":"page not found"}}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestLinksRoutes_BrokenLinks_PreservesWireShape pins GET /api/links/broken
// for an admin: the {"links": [...]} wrapper and the item keys.
func TestLinksRoutes_BrokenLinks_PreservesWireShape(t *testing.T) {
	f := newLinksRouter(t, noSession)

	rec := f.get("/api/links/broken", f.adminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{"links": "array"})

	var body struct {
		Links []json.RawMessage `json:"links"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	if len(body.Links) != 1 {
		t.Fatalf("links = %d, want the one broken link of page A; body=%s", len(body.Links), rec.Body.String())
	}
	test_utils.AssertJSONObjectShape(t, body.Links[0], map[string]string{"from_page_id": "string", "from_title": "string", "from_path": "string", "to_path": "string"})

	var link corelinks.BrokenLink
	if err := json.Unmarshal(body.Links[0], &link); err != nil {
		t.Fatalf("decode %s: %v", body.Links[0], err)
	}
	if link.FromPageID != f.pageA || link.FromTitle != "Page A" || link.FromPath != "/a" || link.ToPath != "/missing" {
		t.Errorf("broken link = %+v, want A -> /missing", link)
	}
}

// TestLinksRoutes_BrokenLinks_WithoutBrokenLinks_ReturnsEmptyList pins that
// the list is [] and not null once the broken link is repaired.
func TestLinksRoutes_BrokenLinks_WithoutBrokenLinks_ReturnsEmptyList(t *testing.T) {
	f := newLinksRouter(t, noSession)
	f.fixBrokenLink(t)

	rec := f.get("/api/links/broken", f.adminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Body.String(), `{"links":[]}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestLinksRoutes_Access pins the gates: link status is a read-group route,
// the broken-link audit is admin only, also with public access on, and always
// forbidden while authentication is disabled.
func TestLinksRoutes_Access(t *testing.T) {
	anonymous := newLinksRouter(t, noSession)
	public := newLinksRouter(t, publicRead)
	authDisabled := newLinksRouter(t, signedIn)

	tests := []struct {
		name     string
		fixture  *linksFixture
		url      string
		token    string
		wantCode int
		wantFlat bool // 401 and 403 bodies are the flat {"error": string}
	}{
		{"link status without a session", anonymous, "/api/pages/" + anonymous.pageA + "/links", "", http.StatusUnauthorized, true},
		{"link status with public access", public, "/api/pages/" + public.pageA + "/links", "", http.StatusOK, false},
		{"link status as editor", anonymous, "/api/pages/" + anonymous.pageA + "/links", anonymous.editorToken, http.StatusOK, false},
		{"broken links without a session", anonymous, "/api/links/broken", "", http.StatusUnauthorized, true},
		{"broken links without a session and with public access", public, "/api/links/broken", "", http.StatusUnauthorized, true},
		{"broken links as editor", anonymous, "/api/links/broken", anonymous.editorToken, http.StatusForbidden, true},
		{"broken links as admin", anonymous, "/api/links/broken", anonymous.adminToken, http.StatusOK, false},
		{"broken links with authentication disabled", authDisabled, "/api/links/broken", "", http.StatusForbidden, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.fixture.get(tt.url, tt.token)
			if rec.Code != tt.wantCode {
				t.Fatalf("status code = %d, want %d; body=%s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantFlat {
				test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{"error": "string"})
			}
		})
	}
}
