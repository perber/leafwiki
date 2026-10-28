package properties

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	coreprop "github.com/perber/wiki/internal/properties"
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

type propertiesPage struct {
	title, slug string
	props       map[string]coreprop.PropertyEntry
}

func text(v string) coreprop.PropertyEntry { return coreprop.PropertyEntry{Value: v, Type: "text"} }

// draftPage is the single page of the wire-shape and error-body tests.
var draftPage = propertiesPage{"Draft Page", "draft-page", map[string]coreprop.PropertyEntry{"status": text("draft")}}

// newPropertiesRouter builds the real router with the properties routes over
// the given pages and their stored properties.
func newPropertiesRouter(t *testing.T, opts httpinternal.RouterOptions, pages ...propertiesPage) http.Handler {
	t.Helper()

	dir := t.TempDir()
	ts := tree.NewTreeService(dir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	store, err := coreprop.NewPropertiesStore(dir)
	if err != nil {
		t.Fatalf("NewPropertiesStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	svc := coreprop.NewPropertiesService(store)

	kind := tree.NodeKindPage
	for _, p := range pages {
		id, err := ts.CreateNode("system", nil, p.title, p.slug, &kind)
		if err != nil {
			t.Fatalf("CreateNode %q: %v", p.slug, err)
		}
		if err := svc.SetPropertiesForPage(*id, p.props); err != nil {
			t.Fatalf("SetPropertiesForPage %q: %v", p.slug, err)
		}
	}

	routes := NewRoutes(RoutesConfig{
		GetPropertyKeys:    NewGetPropertyKeysUseCase(svc),
		GetPagesByProperty: NewGetPagesByPropertyUseCase(svc, ts, nil),
	})
	opts.DisableRequestLog = true
	return httpinternal.NewRouter([]httpinternal.RouteRegistrar{routes}, httpinternal.FrontendConfig{}, opts)
}

// TestPropertiesRoutes_ListAndPages_PreservesWireShape pins the response
// bodies documented in docs/api: success arrays, the localized handler errors
// and the flat middleware error.
func TestPropertiesRoutes_ListAndPages_PreservesWireShape(t *testing.T) {
	keyCount := map[string]string{"key": "string", "count": "number"}
	propertyPage := map[string]string{
		"id": "string", "title": "string", "path": "string", "properties": "object",
		"createdAt": "string", "updatedAt": "string", "creatorId": "string", "lastAuthorId": "string",
	}
	editor := newPropertiesRouter(t, signedIn, draftPage)
	anonymous := newPropertiesRouter(t, noSession, draftPage)

	tests := []struct {
		name        string
		router      http.Handler
		url         string
		wantCode    int
		wantItem    map[string]string // shape of every array element on success
		wantErrCode string            // localized error code; empty means a flat error
	}{
		{"list keys", editor, "/api/properties?q=st&limit=5", http.StatusOK, keyCount, ""},
		{"list keys with a non-numeric limit", editor, "/api/properties?limit=abc", http.StatusBadRequest, nil, ErrCodePropertiesInvalidLimit},
		{"pages by property", editor, "/api/properties/pages?key=status&value=draft", http.StatusOK, propertyPage, ""},
		{"pages by property without key", editor, "/api/properties/pages?value=draft", http.StatusBadRequest, nil, ErrCodePropertiesMissingKey},
		{"pages by property without value", editor, "/api/properties/pages?key=status&value=%20", http.StatusBadRequest, nil, ErrCodePropertiesMissingValue},
		{"list keys without a session", anonymous, "/api/properties", http.StatusUnauthorized, nil, ""},
		{"pages by property without a session", anonymous, "/api/properties/pages?key=status&value=draft", http.StatusUnauthorized, nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.url, nil))

			if rec.Code != tt.wantCode {
				t.Fatalf("status code = %d, want %d; body=%s", rec.Code, tt.wantCode, rec.Body.String())
			}
			switch {
			case tt.wantItem != nil:
				var items []json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
					t.Fatalf("body is not an array: %s", rec.Body.String())
				}
				if len(items) == 0 {
					t.Fatal("empty array, want at least one item")
				}
				for _, item := range items {
					test_utils.AssertJSONObjectShape(t, item, tt.wantItem)
				}
			case tt.wantErrCode != "":
				test_utils.AssertLocalizedError(t, rec.Body.Bytes(), tt.wantErrCode)
			default:
				test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{"error": "string"})
			}
		})
	}
}

func TestPropertiesRoutes_PagesByProperty_PreservesPropertyEntryShape(t *testing.T) {
	router := newPropertiesRouter(t, signedIn, draftPage)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/properties/pages?key=status&value=draft", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var pages []struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pages); err != nil || len(pages) != 1 {
		t.Fatalf("want one page, got %s (err %v)", rec.Body.String(), err)
	}
	entry, ok := pages[0].Properties["status"]
	if !ok {
		t.Fatalf("properties = %v, want status", pages[0].Properties)
	}
	test_utils.AssertJSONObjectShape(t, entry, map[string]string{"value": "string", "type": "string"})
}

// queryPages are "Draft A" (status=draft, owner=alice), "Draft B"
// (status=Draft) and "Many Keys" (status=draft, k000..k204=x), so status has
// count 3 and there are more than 200 keys.
func queryPages() []propertiesPage {
	many := map[string]coreprop.PropertyEntry{"status": text("draft")}
	for i := range 205 {
		many[fmt.Sprintf("k%03d", i)] = text("x")
	}
	return []propertiesPage{
		{"Draft A", "draft-a", map[string]coreprop.PropertyEntry{"status": text("draft"), "owner": text("alice")}},
		{"Draft B", "draft-b", map[string]coreprop.PropertyEntry{"status": text("Draft")}},
		{"Many Keys", "many-keys", many},
	}
}

// TestPropertiesRoutes_QueryParameters_FollowDocumentedSemantics checks limit
// default and clamping, q normalisation, exact (untrimmed, case-sensitive)
// key/value matching, the required-parameter errors and access with public
// access on.
func TestPropertiesRoutes_QueryParameters_FollowDocumentedSemantics(t *testing.T) {
	editor := newPropertiesRouter(t, signedIn, queryPages()...)
	public := newPropertiesRouter(t, publicRead, queryPages()...)

	tests := []struct {
		name     string
		router   http.Handler
		url      string
		wantCode int
		wantLen  int      // number of array items on 200
		wantHead []string // first keys (list) or page titles (pages), in order
		wantErr  string   // localized error code on 400
	}{
		{"default limit is 50", editor, "/api/properties", 200, 50, []string{"status", "k000"}, ""},
		{"limit 0 means 50", editor, "/api/properties?limit=0", 200, 50, nil, ""},
		{"negative limit means 50", editor, "/api/properties?limit=-1", 200, 50, nil, ""},
		{"limit above 200 means 200", editor, "/api/properties?limit=1000", 200, 200, nil, ""},
		{"empty limit is rejected", editor, "/api/properties?limit=", 400, 0, nil, ErrCodePropertiesInvalidLimit},
		{"overflowing limit is rejected", editor, "/api/properties?limit=99999999999999999999", 400, 0, nil, ErrCodePropertiesInvalidLimit},
		{"q is trimmed and lowercased", editor, "/api/properties?q=%20OW", 200, 1, []string{"owner"}, ""},
		{"q percent is literal", editor, "/api/properties?q=%25", 200, 0, nil, ""},
		{"pages: exact value", editor, "/api/properties/pages?key=status&value=draft", 200, 2, nil, ""},
		{"pages: value is case-sensitive", editor, "/api/properties/pages?key=status&value=Draft", 200, 1, []string{"Draft B"}, ""},
		{"pages: value is not trimmed", editor, "/api/properties/pages?key=status&value=%20draft", 200, 0, nil, ""},
		{"pages: key is case-sensitive", editor, "/api/properties/pages?key=STATUS&value=draft", 200, 0, nil, ""},
		{"pages: key is not trimmed", editor, "/api/properties/pages?key=status%20&value=draft", 200, 0, nil, ""},
		{"pages: no parameters reports the key first", editor, "/api/properties/pages", 400, 0, nil, ErrCodePropertiesMissingKey},
		{"pages: blank key", editor, "/api/properties/pages?key=%20&value=draft", 400, 0, nil, ErrCodePropertiesMissingKey},
		{"pages: value missing", editor, "/api/properties/pages?key=status", 400, 0, nil, ErrCodePropertiesMissingValue},
		{"public access: list without session", public, "/api/properties?limit=1", 200, 1, []string{"status"}, ""},
		{"public access: pages without session", public, "/api/properties/pages?key=owner&value=alice", 200, 1, []string{"Draft A"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.url, nil))

			if rec.Code != tt.wantCode {
				t.Fatalf("status code = %d, want %d; body=%s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantErr != "" {
				test_utils.AssertLocalizedError(t, rec.Body.Bytes(), tt.wantErr)
				return
			}
			var items []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil || items == nil {
				t.Fatalf("body is not a JSON array: %s", rec.Body.String())
			}
			if len(items) != tt.wantLen {
				t.Fatalf("len = %d, want %d; body=%s", len(items), tt.wantLen, rec.Body.String())
			}
			for i, want := range tt.wantHead {
				if got := items[i].Key + items[i].Title; got != want {
					t.Errorf("item %d = %q, want %q", i, got, want)
				}
			}
		})
	}
}

// TestPropertiesRoutes_InvalidQuery_ReturnsExactErrorBody pins the exact
// error body so the localized envelope stays wire-compatible (args omitted
// when empty).
func TestPropertiesRoutes_InvalidQuery_ReturnsExactErrorBody(t *testing.T) {
	router := newPropertiesRouter(t, signedIn, draftPage)
	tests := []struct{ name, url, want string }{
		{"non-numeric limit", "/api/properties?limit=x", `{"error":{"code":"properties_invalid_limit","message":"Invalid limit value","template":"invalid limit value"}}`},
		{"missing key", "/api/properties/pages?value=x", `{"error":{"code":"properties_missing_key","message":"Query parameter 'key' is required","template":"query parameter key is required"}}`},
		{"missing value", "/api/properties/pages?key=x", `{"error":{"code":"properties_missing_value","message":"Query parameter 'value' is required","template":"query parameter value is required"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.url, nil))
			if got := rec.Body.String(); got != tt.want {
				t.Errorf("%s body = %s, want %s", tt.url, got, tt.want)
			}
		})
	}
}
