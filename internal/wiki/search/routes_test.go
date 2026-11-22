package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/publicaccess"
	coresearch "github.com/perber/wiki/internal/search"
	coretags "github.com/perber/wiki/internal/tags"
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

type searchPage struct {
	title, slug, body string
	tags              []string
}

// searchPages are two tagged pages and one without tags, all containing the
// word "components": react-guide (react, frontend), vue-guide (vue, frontend)
// and plain-page (no tags).
var searchPages = []searchPage{
	{"React Guide", "react-guide", "Hooks and components.", []string{"react", "frontend"}},
	{"Vue Guide", "vue-guide", "Composition and components.", []string{"vue", "frontend"}},
	{"Plain Page", "plain-page", "Plain components text.", nil},
}

// addSearchPage creates the page and writes it to the text index and the tag
// index, the way the page-save side effects do.
func addSearchPage(t *testing.T, ts *tree.TreeService, tagsSvc *coretags.TagsService, index *coresearch.SQLiteIndex, p searchPage) {
	t.Helper()

	kind := tree.NodeKindPage
	id, err := ts.CreateNode("system", nil, p.title, p.slug, &kind)
	if err != nil {
		t.Fatalf("CreateNode %q: %v", p.slug, err)
	}

	content := p.body
	if len(p.tags) > 0 {
		content = "---\ntags:\n"
		for _, tag := range p.tags {
			content += "  - " + tag + "\n"
		}
		content += "---\n\n" + p.body
	}
	if err := ts.UpdateNode(tree.UpdateNodeInput{UserID: "system", ID: *id, Title: p.title, Slug: p.slug, Content: &content, ExpectedVersion: tree.VersionUnchecked, PreserveFrontmatter: true}); err != nil {
		t.Fatalf("UpdateNode %q: %v", p.slug, err)
	}

	page, err := ts.GetPage(*id)
	if err != nil {
		t.Fatalf("GetPage %q: %v", p.slug, err)
	}
	if err := tagsSvc.IndexPageContent(page.ID, page.RawContent); err != nil {
		t.Fatalf("IndexPageContent %q: %v", p.slug, err)
	}
	path := strings.TrimPrefix(page.CalculatePath(), "/")
	if err := index.IndexPage(path, path+".md", page.ID, page.Title, page.Kind, page.RawContent); err != nil {
		t.Fatalf("IndexPage %q: %v", p.slug, err)
	}
}

// newSearchRouter builds the real router with the search routes over the
// given pages. It also returns the indexing status the status route reports.
func newSearchRouter(t *testing.T, opts httpinternal.RouterOptions, pages ...searchPage) (http.Handler, *coresearch.IndexingStatus) {
	t.Helper()
	uc, tagsSvc, ts := setupSearchUseCaseForTags(t)
	for _, p := range pages {
		addSearchPage(t, ts, tagsSvc, uc.index, p)
	}

	status := coresearch.NewIndexingStatus()
	routes := NewRoutes(RoutesConfig{Search: uc, GetIndexingStatus: NewGetIndexingStatusUseCase(status)})
	opts.DisableRequestLog = true
	return httpinternal.NewRouter([]httpinternal.RouteRegistrar{routes}, httpinternal.FrontendConfig{}, opts), status
}

func get(router http.Handler, url string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

// TestSearchRoutes_Search_PreservesWireShape pins the response body of
// GET /api/search for text, tag-only and combined queries: top-level keys,
// item keys (tags only for tagged pages), facet keys and the echoed paging.
func TestSearchRoutes_Search_PreservesWireShape(t *testing.T) {
	taggedItem := map[string]string{"page_id": "string", "title": "string", "path": "string", "kind": "string", "rank": "number", "excerpt": "string", "tags": "array"}
	untaggedItem := map[string]string{"page_id": "string", "title": "string", "path": "string", "kind": "string", "rank": "number", "excerpt": "string"}
	router, _ := newSearchRouter(t, signedIn, searchPages...)

	tests := []struct {
		name       string
		url        string
		wantCount  int
		wantItems  int
		wantOffset int
		wantLimit  int
	}{
		{"text query", "/api/search?q=components", 3, 3, 0, 20},
		{"text query with paging", "/api/search?q=components&offset=1&limit=1", 3, 1, 1, 1},
		{"text query narrowed by a tag", "/api/search?q=components&tags=frontend", 2, 2, 0, 20},
		{"tag only", "/api/search?tags=frontend", 2, 2, 0, 20},
		{"tag only with paging", "/api/search?tags=frontend&offset=1&limit=1", 2, 1, 1, 1},
		{"repeated tags must all match", "/api/search?tags=react&tags=frontend", 1, 1, 0, 20},
		{"comma-separated tags must all match", "/api/search?tags=react,frontend", 1, 1, 0, 20},
		{"tag that no page has", "/api/search?tags=nope", 0, 0, 0, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(router, tt.url)
			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}

			test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{
				"limit": "number", "offset": "number", "count": "number", "items": "array", "tag_facets": "array",
			})
			var body struct {
				Limit     int                          `json:"limit"`
				Offset    int                          `json:"offset"`
				Count     int                          `json:"count"`
				Items     []map[string]json.RawMessage `json:"items"`
				TagFacets []json.RawMessage            `json:"tag_facets"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %s: %v", rec.Body.String(), err)
			}
			if body.Count != tt.wantCount || len(body.Items) != tt.wantItems || body.Offset != tt.wantOffset || body.Limit != tt.wantLimit {
				t.Fatalf("count/items/offset/limit = %d/%d/%d/%d, want %d/%d/%d/%d; body=%s",
					body.Count, len(body.Items), body.Offset, body.Limit,
					tt.wantCount, tt.wantItems, tt.wantOffset, tt.wantLimit, rec.Body.String())
			}

			for _, item := range body.Items {
				raw, err := json.Marshal(item)
				if err != nil {
					t.Fatalf("encode item: %v", err)
				}
				want := taggedItem
				if string(item["title"]) == `"Plain Page"` {
					want = untaggedItem
				}
				test_utils.AssertJSONObjectShape(t, raw, want)
			}
			for _, facet := range body.TagFacets {
				test_utils.AssertJSONObjectShape(t, facet, map[string]string{"tag": "string", "count": "number"})
			}
		})
	}
}

// TestSearchRoutes_Search_TagFacetsListTagsOfTheMatches pins that the facets
// come with the hits: tags of the matching pages, most frequent first.
func TestSearchRoutes_Search_TagFacetsListTagsOfTheMatches(t *testing.T) {
	router, _ := newSearchRouter(t, signedIn, searchPages...)

	rec := get(router, "/api/search?q=components")
	var body struct {
		TagFacets []struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		} `json:"tag_facets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}

	got := make([]string, 0, len(body.TagFacets))
	for _, f := range body.TagFacets {
		got = append(got, f.Tag+":"+string(rune('0'+f.Count)))
	}
	if want := "frontend:2 react:1 vue:1"; strings.Join(got, " ") != want {
		t.Errorf("tag facets = %q, want %q", strings.Join(got, " "), want)
	}
}

// TestSearchRoutes_Search_TextQueryWithoutMatch pins that a text query that
// matches nothing sends empty lists, not null.
func TestSearchRoutes_Search_TextQueryWithoutMatch(t *testing.T) {
	router, _ := newSearchRouter(t, signedIn, searchPages...)

	rec := get(router, "/api/search?q=zebra")
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{
		"limit": "number", "offset": "number", "count": "number", "items": "array", "tag_facets": "array",
	})
	if body := rec.Body.String(); !strings.Contains(body, `"items":[]`) || !strings.Contains(body, `"count":0`) {
		t.Errorf(`body = %s, want "items":[] and "count":0`, body)
	}
}

// A q of only whitespace counts as missing, so with a tag the request is a
// tag-only search, not a 400.
func TestSearchRoutes_Search_WhitespaceQueryWithTag_IsTagOnlySearch(t *testing.T) {
	router, _ := newSearchRouter(t, signedIn, searchPages...)

	rec := get(router, "/api/search?q=%20%09&tags=react")
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"count":1`) || !strings.Contains(body, "React Guide") {
		t.Errorf(`body = %s, want the one page tagged react`, body)
	}
}

// TestSearchRoutes_Search_ErrorBodies pins the exact error bodies so the
// localized envelope stays wire-compatible (args omitted when empty).
func TestSearchRoutes_Search_ErrorBodies(t *testing.T) {
	router, _ := newSearchRouter(t, signedIn, searchPages...)
	tests := []struct{ name, url, want string }{
		{"missing query", "/api/search", `{"error":{"code":"search_missing_query","message":"Query parameter 'q' is required","template":"query parameter q is required"}}`},
		{"empty query and empty tags", "/api/search?q=&tags=,", `{"error":{"code":"search_missing_query","message":"Query parameter 'q' is required","template":"query parameter q is required"}}`},
		{"whitespace-only query", "/api/search?q=%20%09%20", `{"error":{"code":"search_missing_query","message":"Query parameter 'q' is required","template":"query parameter q is required"}}`},
		{"invalid offset", "/api/search?q=components&offset=x", `{"error":{"code":"search_invalid_offset","message":"Invalid offset value","template":"invalid offset value"}}`},
		{"invalid limit", "/api/search?q=components&limit=x", `{"error":{"code":"search_invalid_limit","message":"Invalid limit value","template":"invalid limit value"}}`},
		{"query with too many terms", "/api/search?q=" + strings.TrimSuffix(strings.Repeat("pi+", 13), "+"), `{"error":{"code":"search_query_too_long","message":"Search query is too long: at most 256 characters and 12 terms","template":"search query is too long: at most %s characters and %s terms","args":["256","12"]}}`},
		{"offset beyond the int64 range", "/api/search?q=components&offset=99999999999999999999", `{"error":{"code":"search_invalid_offset","message":"Invalid offset value","template":"invalid offset value"}}`},
		{"limit beyond the int64 range", "/api/search?q=components&limit=-99999999999999999999", `{"error":{"code":"search_invalid_limit","message":"Invalid limit value","template":"invalid limit value"}}`},
		{"fractional limit", "/api/search?q=components&limit=1.5", `{"error":{"code":"search_invalid_limit","message":"Invalid limit value","template":"invalid limit value"}}`},
		{"empty limit", "/api/search?q=components&limit=", `{"error":{"code":"search_invalid_limit","message":"Invalid limit value","template":"invalid limit value"}}`},
		{"tag-only search with invalid offset", "/api/search?tags=frontend&offset=x", `{"error":{"code":"search_invalid_offset","message":"Invalid offset value","template":"invalid offset value"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(router, tt.url)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status code = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			if got := rec.Body.String(); got != tt.want {
				t.Errorf("%s body = %s, want %s", tt.url, got, tt.want)
			}
		})
	}
}

// TestSearchRoutes_Search_ContextEnds pins the responses when a search's
// context ends before the index answers: the search timeout is a 503 with the
// search_unavailable body; a canceled request gets 499 and no body, which only
// the access log sees, because the client is gone.
func TestSearchRoutes_Search_ContextEnds(t *testing.T) {
	tests := []struct {
		name       string
		timeout    time.Duration
		cancel     bool
		wantStatus int
		wantBody   string
	}{
		{
			name:       "search timeout elapses",
			timeout:    0,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"error":{"code":"search_unavailable","message":"Search is currently unavailable","template":"search is currently unavailable"}}`,
		},
		{
			name:       "client canceled the request",
			timeout:    time.Minute,
			cancel:     true,
			wantStatus: statusClientClosedRequest,
			wantBody:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := newSearchRouter(t, signedIn, searchPages...)
			withSearchTimeout(t, tt.timeout)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancel {
				cancel()
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/search?q=components", nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status code = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %s, want %s", got, tt.wantBody)
			}
		})
	}
}

// TestSearchRoutes_Status_PreservesWireShape pins GET /api/search/status:
// the four keys, the zero finished_at before the first run and the values
// after a run.
func TestSearchRoutes_Status_PreservesWireShape(t *testing.T) {
	router, status := newSearchRouter(t, signedIn)
	shape := map[string]string{"active": "bool", "indexed": "number", "failed": "number", "finished_at": "string"}

	decode := func(t *testing.T) (body struct {
		Active     bool   `json:"active"`
		Indexed    int    `json:"indexed"`
		Failed     int    `json:"failed"`
		FinishedAt string `json:"finished_at"`
	}) {
		t.Helper()
		rec := get(router, "/api/search/status")
		if rec.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), shape)
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", rec.Body.String(), err)
		}
		return body
	}

	const zeroTime = "0001-01-01T00:00:00Z"

	before := decode(t)
	if before.Active || before.Indexed != 0 || before.Failed != 0 || before.FinishedAt != zeroTime {
		t.Errorf("before the first run = %+v, want idle with finished_at %s", before, zeroTime)
	}

	status.Start()
	if running := decode(t); !running.Active || running.FinishedAt != zeroTime {
		t.Errorf("while running = %+v, want active with finished_at %s", running, zeroTime)
	}

	status.Success()
	status.Fail()
	status.Finish()
	after := decode(t)
	if after.Active || after.Indexed != 1 || after.Failed != 1 || after.FinishedAt == zeroTime {
		t.Errorf("after the run = %+v, want idle, 1 indexed, 1 failed and a finish time", after)
	}
}

// TestSearchRoutes_ReadGroup_Access pins the read-group gate of both routes:
// 401 without a session, 200 with public access on or a session.
func TestSearchRoutes_ReadGroup_Access(t *testing.T) {
	anonymous, _ := newSearchRouter(t, noSession, searchPages...)
	public, _ := newSearchRouter(t, publicRead, searchPages...)
	editor, _ := newSearchRouter(t, signedIn, searchPages...)

	tests := []struct {
		name     string
		router   http.Handler
		url      string
		wantCode int
	}{
		{"search without a session", anonymous, "/api/search?q=components", http.StatusUnauthorized},
		{"status without a session", anonymous, "/api/search/status", http.StatusUnauthorized},
		{"search with public access", public, "/api/search?q=components", http.StatusOK},
		{"status with public access", public, "/api/search/status", http.StatusOK},
		{"search with a session", editor, "/api/search?q=components", http.StatusOK},
		{"status with a session", editor, "/api/search/status", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(tt.router, tt.url)
			if rec.Code != tt.wantCode {
				t.Fatalf("status code = %d, want %d; body=%s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantCode == http.StatusUnauthorized {
				test_utils.AssertJSONObjectShape(t, rec.Body.Bytes(), map[string]string{"error": "string"})
			}
		})
	}
}

// headerCountingRecorder counts WriteHeader calls, so a test can tell a
// response written once from one whose status was written twice.
type headerCountingRecorder struct {
	*httptest.ResponseRecorder
	writeHeaders int
}

func (r *headerCountingRecorder) WriteHeader(code int) {
	r.writeHeaders++
	r.ResponseRecorder.WriteHeader(code)
}

// TestSearchRoutes_Search_CanceledRequest_PassesThroughMiddleware pins that
// the 499 for a client that went away runs through the full middleware
// chain: the status is written once, the access log and the request metrics
// record 499, and nothing is logged as an error or a recovered panic.
func TestSearchRoutes_Search_CanceledRequest_PassesThroughMiddleware(t *testing.T) {
	uc, tagsSvc, ts := setupSearchUseCaseForTags(t)
	for _, p := range searchPages {
		addSearchPage(t, ts, tagsSvc, uc.index, p)
	}
	metrics := httpmetrics.NewHTTPMetrics("test")
	opts := httpinternal.RouterOptions{AuthDisabled: true, Metrics: metrics}
	routes := NewRoutes(RoutesConfig{Search: uc, GetIndexingStatus: NewGetIndexingStatusUseCase(coresearch.NewIndexingStatus())})
	router := httpinternal.NewRouter([]httpinternal.RouteRegistrar{routes}, httpinternal.FrontendConfig{}, opts)
	logs := captureLogs(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	rec := &headerCountingRecorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/search?q=components", nil))

	if rec.Code != statusClientClosedRequest {
		t.Fatalf("status code = %d, want %d; body=%s", rec.Code, statusClientClosedRequest, rec.Body.String())
	}
	if rec.writeHeaders != 1 {
		t.Errorf("WriteHeader called %d times, want 1", rec.writeHeaders)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
	got := logs.String()
	if !strings.Contains(got, "status=499") {
		t.Errorf("access log does not record status 499:\n%s", got)
	}
	if strings.Contains(got, "level=ERROR") || strings.Contains(got, "level=WARN") {
		t.Errorf("a canceled search logged a warning or an error:\n%s", got)
	}

	mrec := httptest.NewRecorder()
	metrics.HTTPHandler().ServeHTTP(mrec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(grepLines(mrec.Body.String(), "http_requests_total"), `"499"`) {
		t.Errorf("request metrics do not record status 499:\n%s", grepLines(mrec.Body.String(), "http_requests_total"))
	}
}

func grepLines(s, sub string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, sub) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// TestSearchRoutes_Search_BoundsLimitAndOffset pins the paging bounds for
// both the text path and the tag-only path: a limit below 1 means the default
// of 20, a limit above 100 means 100, a negative offset means 0 and an
// offset above 10 000 means 10 000. The
// response echoes the values that were actually applied.
func TestSearchRoutes_Search_BoundsLimitAndOffset(t *testing.T) {
	router, _ := newSearchRouter(t, signedIn, searchPages...)

	tests := []struct {
		name        string
		query       string
		wantLimit   int
		wantOffset  int
		wantNoItems bool
	}{
		{"text: limit -1 means the default", "q=components&limit=-1", 20, 0, false},
		{"text: limit 0 means the default", "q=components&limit=0", 20, 0, false},
		{"text: limit above the maximum is capped", "q=components&limit=100000", 100, 0, false},
		{"text: limit at the maximum is kept", "q=components&limit=100", 100, 0, false},
		{"text: limit 1 is kept", "q=components&limit=1", 1, 0, false},
		{"text: negative offset means 0", "q=components&offset=-5", 20, 0, false},
		{"tags: limit above the maximum is capped", "tags=frontend&limit=100000", 100, 0, false},
		{"tags: limit -1 means the default", "tags=frontend&limit=-1", 20, 0, false},
		{"tags: negative offset means 0", "tags=frontend&offset=-5", 20, 0, false},
		// Values up to the 32-bit int range, so the rows hold on 386 too.
		{"text: largest 32-bit limit is capped", "q=components&limit=2147483647", 100, 0, false},
		{"text: offset beyond the hit count is echoed, no items", "q=components&offset=50", 20, 50, true},
		{"text: offset above the maximum is capped, no items", "q=components&offset=2147483647", 20, 10000, true},
		{"text: offset at the maximum is kept", "q=components&offset=10000", 20, 10000, true},
		{"text: limit and offset both out of range", "q=components&limit=-7&offset=-7", 20, 0, false},
		{"tags: limit 0 means the default", "tags=frontend&limit=0", 20, 0, false},
		{"tags: offset beyond the hit count is clamped to the count", "tags=frontend&offset=50", 20, 2, true},
		{"tags: largest 32-bit offset is clamped to the count", "tags=frontend&offset=2147483647", 20, 2, true},
		{"tags: largest 32-bit limit is capped", "tags=frontend&limit=2147483647", 100, 0, false},
		{"text and tags: limit above the maximum is capped", "q=components&tags=frontend&limit=100000", 100, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(router, "/api/search?"+tt.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			var body struct {
				Limit  int               `json:"limit"`
				Offset int               `json:"offset"`
				Items  []json.RawMessage `json:"items"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v; body=%s", err, rec.Body.String())
			}
			if body.Limit != tt.wantLimit || body.Offset != tt.wantOffset {
				t.Fatalf("limit, offset = %d, %d; want %d, %d", body.Limit, body.Offset, tt.wantLimit, tt.wantOffset)
			}
			if len(body.Items) > tt.wantLimit {
				t.Fatalf("got %d items, want at most %d", len(body.Items), tt.wantLimit)
			}
			if tt.wantNoItems != (len(body.Items) == 0) {
				t.Fatalf("got %d items, wantNoItems %v", len(body.Items), tt.wantNoItems)
			}
		})
	}
}
