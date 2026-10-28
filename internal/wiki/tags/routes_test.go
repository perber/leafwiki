package tags

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	httpinternal "github.com/perber/wiki/internal/http"
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

type tagsPage struct {
	title, slug string
	tags        []string
}

// reactGuide is the single page of the wire-shape and error-body tests.
var reactGuide = tagsPage{"React Guide", "react-guide", []string{"react", "frontend"}}

// newTagsRouter builds the real router with the tags routes over the given
// pages, each indexed with the body "<title> body.".
func newTagsRouter(t *testing.T, opts httpinternal.RouterOptions, pages ...tagsPage) http.Handler {
	t.Helper()
	uc, svc, ts := setupUseCases(t)
	for _, p := range pages {
		createAndIndexPage(t, ts, svc, p.title, p.slug, p.tags, p.title+" body.")
	}

	routes := NewRoutes(RoutesConfig{GetTags: NewGetTagsUseCase(svc), GetPagesByTags: uc})
	opts.DisableRequestLog = true
	return httpinternal.NewRouter([]httpinternal.RouteRegistrar{routes}, httpinternal.FrontendConfig{}, opts)
}

// TestTagsRoutes_ListAndPages_PreservesWireShape pins the response bodies
// documented in docs/api: success arrays, the localized handler errors and
// the flat middleware error.
func TestTagsRoutes_ListAndPages_PreservesWireShape(t *testing.T) {
	tagCount := map[string]string{"tag": "string", "count": "number"}
	taggedPage := map[string]string{
		"id": "string", "title": "string", "path": "string", "excerpt": "string", "tags": "array",
		"createdAt": "string", "updatedAt": "string", "creatorId": "string", "lastAuthorId": "string",
	}
	editor := newTagsRouter(t, signedIn, reactGuide)
	anonymous := newTagsRouter(t, noSession, reactGuide)

	tests := []struct {
		name        string
		router      http.Handler
		url         string
		wantCode    int
		wantItem    map[string]string // shape of every array element on success
		wantErrCode string            // localized error code; empty means a flat error
	}{
		{"list tags", editor, "/api/tags", http.StatusOK, tagCount, ""},
		{"list tags for a selection", editor, "/api/tags?q=fr&limit=5&selected=react", http.StatusOK, tagCount, ""},
		{"list tags with a non-numeric limit", editor, "/api/tags?limit=abc", http.StatusBadRequest, nil, ErrCodeTagsInvalidLimit},
		{"pages by tags", editor, "/api/tags/pages?tags=react,frontend", http.StatusOK, taggedPage, ""},
		{"pages by repeated tags", editor, "/api/tags/pages?tags=react&tags=frontend", http.StatusOK, taggedPage, ""},
		{"pages by tags without tags", editor, "/api/tags/pages?tags=,", http.StatusBadRequest, nil, ErrCodeTagsMissingParam},
		{"list tags without a session", anonymous, "/api/tags", http.StatusUnauthorized, nil, ""},
		{"pages by tags without a session", anonymous, "/api/tags/pages?tags=react", http.StatusUnauthorized, nil, ""},
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

// queryPages are "React Guide" (react, frontend), "Vue Guide" (vue,
// frontend) and "Many Tags" (t000..t204), so frontend has count 2, every
// other tag count 1, and there are more than 200 tags.
func queryPages() []tagsPage {
	many := make([]string, 0, 205)
	for i := range 205 {
		many = append(many, fmt.Sprintf("t%03d", i))
	}
	return []tagsPage{
		reactGuide,
		{"Vue Guide", "vue-guide", []string{"vue", "frontend"}},
		{"Many Tags", "many-tags", many},
	}
}

// TestTagsRoutes_QueryParameters_FollowDocumentedSemantics checks limit
// default and clamping, q and selected normalisation, repeated and
// comma-separated lists, LIKE wildcards taken literally, and access with
// public access on.
func TestTagsRoutes_QueryParameters_FollowDocumentedSemantics(t *testing.T) {
	editor := newTagsRouter(t, signedIn, queryPages()...)
	public := newTagsRouter(t, publicRead, queryPages()...)

	tests := []struct {
		name     string
		router   http.Handler
		url      string
		wantCode int
		wantLen  int      // number of array items on 200
		wantHead []string // first tags (list) or page titles (pages), in order
		wantErr  string   // localized error code on 400
	}{
		{"default limit is 50", editor, "/api/tags", 200, 50, []string{"frontend", "react", "t000"}, ""},
		{"limit 0 means 50", editor, "/api/tags?limit=0", 200, 50, nil, ""},
		{"negative limit means 50", editor, "/api/tags?limit=-3", 200, 50, nil, ""},
		{"limit 1 orders by count first", editor, "/api/tags?limit=1", 200, 1, []string{"frontend"}, ""},
		{"limit 200 is honoured", editor, "/api/tags?limit=200", 200, 200, nil, ""},
		{"limit above 200 means 200", editor, "/api/tags?limit=500", 200, 200, nil, ""},
		{"empty limit is rejected", editor, "/api/tags?limit=", 400, 0, nil, ErrCodeTagsInvalidLimit},
		{"fractional limit is rejected", editor, "/api/tags?limit=1.5", 400, 0, nil, ErrCodeTagsInvalidLimit},
		{"overflowing limit is rejected", editor, "/api/tags?limit=99999999999999999999", 400, 0, nil, ErrCodeTagsInvalidLimit},
		{"q is trimmed and lowercased", editor, "/api/tags?q=%20FR%20", 200, 1, []string{"frontend"}, ""},
		{"q percent is literal", editor, "/api/tags?q=%25", 200, 0, nil, ""},
		{"q underscore is literal", editor, "/api/tags?q=_", 200, 0, nil, ""},
		{"selected leaves out itself", editor, "/api/tags?selected=react", 200, 1, []string{"frontend"}, ""},
		{"selected narrows to co-occurring tags", editor, "/api/tags?selected=frontend", 200, 2, []string{"react", "vue"}, ""},
		{"selected comma-separated and normalised", editor, "/api/tags?selected=REACT,%20frontend", 200, 0, nil, ""},
		{"selected repeated", editor, "/api/tags?selected=react&selected=frontend", 200, 0, nil, ""},
		{"blank selected is no selection", editor, "/api/tags?selected=,%20", 200, 50, []string{"frontend"}, ""},
		{"pages: tags lowercased", editor, "/api/tags/pages?tags=%20REACT", 200, 1, []string{"React Guide"}, ""},
		{"pages: one tag on two pages", editor, "/api/tags/pages?tags=frontend", 200, 2, nil, ""},
		{"pages: repeated tags must all match", editor, "/api/tags/pages?tags=frontend&tags=vue", 200, 1, []string{"Vue Guide"}, ""},
		{"pages: no page has both", editor, "/api/tags/pages?tags=react,vue", 200, 0, nil, ""},
		{"pages: unknown tag", editor, "/api/tags/pages?tags=nope", 200, 0, nil, ""},
		{"pages: tags missing", editor, "/api/tags/pages", 400, 0, nil, ErrCodeTagsMissingParam},
		{"pages: tags empty", editor, "/api/tags/pages?tags=", 400, 0, nil, ErrCodeTagsMissingParam},
		{"public access: list without session", public, "/api/tags?limit=1", 200, 1, []string{"frontend"}, ""},
		{"public access: pages without session", public, "/api/tags/pages?tags=react", 200, 1, []string{"React Guide"}, ""},
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
				Tag   string `json:"tag"`
				Title string `json:"title"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil || items == nil {
				t.Fatalf("body is not a JSON array: %s", rec.Body.String())
			}
			if len(items) != tt.wantLen {
				t.Fatalf("len = %d, want %d; body=%s", len(items), tt.wantLen, rec.Body.String())
			}
			for i, want := range tt.wantHead {
				if got := items[i].Tag + items[i].Title; got != want {
					t.Errorf("item %d = %q, want %q", i, got, want)
				}
			}
		})
	}
}

// TestTagsRoutes_InvalidQuery_ReturnsExactErrorBody pins the exact error body
// so the localized envelope stays wire-compatible (args omitted when empty).
func TestTagsRoutes_InvalidQuery_ReturnsExactErrorBody(t *testing.T) {
	router := newTagsRouter(t, signedIn, reactGuide)
	tests := []struct{ name, url, want string }{
		{"non-numeric limit", "/api/tags?limit=x", `{"error":{"code":"tags_invalid_limit","message":"Invalid limit value","template":"invalid limit value"}}`},
		{"missing tags", "/api/tags/pages", `{"error":{"code":"tags_missing_param","message":"Query parameter 'tags' is required","template":"query parameter tags is required"}}`},
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
