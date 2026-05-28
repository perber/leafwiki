package mcp_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perber/wiki/internal/core/assets"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/wiki"
	wikiassets "github.com/perber/wiki/internal/wiki/assets"
	wikimcp "github.com/perber/wiki/internal/wiki/mcp"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
	wikitags "github.com/perber/wiki/internal/wiki/tags"
)

var baseToolNames = wikimcp.BaseToolNames()

var baseToolInputProperties = map[string][]string{
	"get_config":            {},
	"get_current_user":      {},
	"get_tree":              {"depth"},
	"get_page":              {"id", "pageId"},
	"get_page_by_path":      {"path"},
	"lookup_path":           {"path"},
	"resolve_permalink":     {"id", "pageId"},
	"suggest_slug":          {"parentId", "currentId", "title"},
	"create_page":           {"parentId", "title", "slug", "kind"},
	"update_page":           {"id", "version", "title", "slug", "content", "tags", "properties"},
	"delete_page":           {"id", "version", "recursive"},
	"move_page":             {"id", "version", "parentId"},
	"sort_pages":            {"parentId", "orderedIds"},
	"ensure_page":           {"path", "title", "kind"},
	"convert_page":          {"id", "version", "targetKind"},
	"copy_page":             {"id", "targetParentId", "title", "slug"},
	"search_pages":          {"q", "tags", "offset", "limit"},
	"get_search_status":     {},
	"list_tags":             {"q", "selected", "limit"},
	"get_pages_by_tags":     {"tags"},
	"list_property_keys":    {"q", "limit"},
	"get_pages_by_property": {"key", "value"},
	"get_link_status":       {"id", "pageId"},
	"upload_asset":          {"pageId", "filename", "contentBase64"},
	"get_asset":             {"pageId", "filename"},
	"list_assets":           {"id", "pageId"},
	"rename_asset":          {"pageId", "oldFilename", "newFilename"},
	"delete_asset":          {"pageId", "filename"},
}

var featureToolInputProperties = map[string][]string{
	"list_revisions":        {"id", "pageId", "cursor", "limit"},
	"get_latest_revision":   {"id", "pageId"},
	"get_revision":          {"id", "pageId", "revisionId"},
	"compare_revisions":     {"id", "pageId", "baseRevisionId", "targetRevisionId"},
	"get_revision_asset":    {"id", "pageId", "revisionId", "assetName"},
	"restore_revision":      {"id", "pageId", "revisionId"},
	"preview_page_refactor": {"id", "pageId", "kind", "title", "slug", "content", "parentId"},
	"apply_page_refactor":   {"id", "pageId", "version", "kind", "title", "slug", "content", "parentId", "rewriteLinks"},
}

type httpMCPParityCase struct {
	Tool      string
	HTTPRoute string
	Assertion string
}

var mcpHTTPParityCases = []httpMCPParityCase{
	{Tool: "get_config", HTTPRoute: "GET /api/config", Assertion: "selected config fields match"},
	{Tool: "get_current_user", HTTPRoute: "GET /api/auth/me", Assertion: "current user payloads match"},
	{Tool: "get_tree", HTTPRoute: "GET /api/tree", Assertion: "tree payloads match"},
	{Tool: "get_page", HTTPRoute: "GET /api/pages/:id", Assertion: "page payloads match"},
	{Tool: "get_page_by_path", HTTPRoute: "GET /api/pages/by-path", Assertion: "page payloads match"},
	{Tool: "lookup_path", HTTPRoute: "GET /api/pages/lookup", Assertion: "lookup payloads match"},
	{Tool: "resolve_permalink", HTTPRoute: "GET /api/pages/permalink/:id", Assertion: "target payloads match"},
	{Tool: "suggest_slug", HTTPRoute: "GET /api/pages/slug-suggestion", Assertion: "slug payloads match"},
	{Tool: "create_page", HTTPRoute: "POST /api/pages", Assertion: "created page is visible through HTTP with matching payload"},
	{Tool: "update_page", HTTPRoute: "PUT /api/pages/:id", Assertion: "updated page is visible through HTTP and stale page_version_conflict errors match"},
	{Tool: "delete_page", HTTPRoute: "DELETE /api/pages/:id", Assertion: "success payloads, deleted state, and optimistic page_version_conflict errors match"},
	{Tool: "move_page", HTTPRoute: "PUT /api/pages/:id/move", Assertion: "success payloads, final route lookup, parent placement, and stale page_version_conflict errors match"},
	{Tool: "sort_pages", HTTPRoute: "PUT /api/pages/:id/sort", Assertion: "message payloads match"},
	{Tool: "ensure_page", HTTPRoute: "POST /api/pages/ensure", Assertion: "ensured page payloads match"},
	{Tool: "convert_page", HTTPRoute: "POST /api/pages/convert/:id", Assertion: "HTTP no-content result, final page, and stale page_version_conflict errors match"},
	{Tool: "copy_page", HTTPRoute: "POST /api/pages/copy/:id", Assertion: "copied page fields, route lookup, and source preservation match"},
	{Tool: "search_pages", HTTPRoute: "GET /api/search", Assertion: "count, pagination, facets, hasMore, and items match"},
	{Tool: "get_search_status", HTTPRoute: "GET /api/search/status", Assertion: "status payloads match"},
	{Tool: "list_tags", HTTPRoute: "GET /api/tags", Assertion: "tag payloads match"},
	{Tool: "get_pages_by_tags", HTTPRoute: "GET /api/tags/pages", Assertion: "page payloads match"},
	{Tool: "list_property_keys", HTTPRoute: "GET /api/properties", Assertion: "key payloads match"},
	{Tool: "get_pages_by_property", HTTPRoute: "GET /api/properties/pages", Assertion: "page payloads match"},
	{Tool: "get_link_status", HTTPRoute: "GET /api/pages/:id/links", Assertion: "link status payloads match"},
	{Tool: "upload_asset", HTTPRoute: "POST /api/pages/:id/assets", Assertion: "upload result, exact bytes, MIME type, and list visibility match"},
	{Tool: "get_asset", HTTPRoute: "GET /assets/:pageId/:filename", Assertion: "asset content and type match"},
	{Tool: "list_assets", HTTPRoute: "GET /api/pages/:id/assets", Assertion: "asset lists match"},
	{Tool: "rename_asset", HTTPRoute: "PUT /api/pages/:id/assets/rename", Assertion: "rename result, exact bytes, old-name absence, and new-name presence match"},
	{Tool: "delete_asset", HTTPRoute: "DELETE /api/pages/:id/assets/:name", Assertion: "delete payloads and exact asset absence match"},
	{Tool: "list_revisions", HTTPRoute: "GET /api/pages/:id/revisions", Assertion: "revision list payloads match"},
	{Tool: "get_latest_revision", HTTPRoute: "GET /api/pages/:id/revisions/latest", Assertion: "revision payloads match"},
	{Tool: "get_revision", HTTPRoute: "GET /api/pages/:id/revisions/:revisionId", Assertion: "snapshot payloads match"},
	{Tool: "compare_revisions", HTTPRoute: "GET /api/pages/:id/revisions/compare", Assertion: "comparison payloads match"},
	{Tool: "get_revision_asset", HTTPRoute: "GET /api/pages/:id/revisions/:revisionId/assets/:name", Assertion: "asset content and type match"},
	{Tool: "restore_revision", HTTPRoute: "POST /api/pages/:id/revisions/:revisionId/restore", Assertion: "stable restored page fields match"},
	{Tool: "preview_page_refactor", HTTPRoute: "POST /api/pages/:id/refactor/preview", Assertion: "preview payloads match"},
	{Tool: "apply_page_refactor", HTTPRoute: "POST /api/pages/:id/refactor/apply", Assertion: "successful apply payloads, rewritten links, and stale page_version_conflict errors match"},
}

var parityCoverage = struct {
	sync.Mutex
	seen map[string]map[string]struct{}
}{seen: map[string]map[string]struct{}{}}

func resetHTTPMCPParityCoverage() {
	parityCoverage.Lock()
	defer parityCoverage.Unlock()
	parityCoverage.seen = map[string]map[string]struct{}{}
}

func hasHTTPMCPParityRecorded(t *testing.T) bool {
	t.Helper()

	seenCases, expected := expectedHTTPMCPParityCases(t)
	parityCoverage.Lock()
	defer parityCoverage.Unlock()
	for _, name := range expected {
		tc := seenCases[name]
		if _, exercised := parityCoverage.seen[name][tc.HTTPRoute]; !exercised {
			return false
		}
	}
	return true
}

func recordHTTPMCPParity(t *testing.T, tool, httpRoute string) {
	t.Helper()
	found := false
	for _, tc := range mcpHTTPParityCases {
		if tc.Tool == tool {
			found = true
			if tc.HTTPRoute != httpRoute {
				t.Fatalf("parity record for %s used route %q, want %q", tool, httpRoute, tc.HTTPRoute)
			}
			break
		}
	}
	if !found {
		t.Fatalf("parity record for unknown tool %q", tool)
	}

	parityCoverage.Lock()
	defer parityCoverage.Unlock()
	routes := parityCoverage.seen[tool]
	if routes == nil {
		routes = map[string]struct{}{}
		parityCoverage.seen[tool] = routes
	}
	routes[httpRoute] = struct{}{}
}

func assertHTTPMCPParityRecorded(t *testing.T) {
	t.Helper()

	seenCases, expected := expectedHTTPMCPParityCases(t)
	parityCoverage.Lock()
	defer parityCoverage.Unlock()
	for _, name := range expected {
		tc := seenCases[name]
		if _, exercised := parityCoverage.seen[name][tc.HTTPRoute]; !exercised {
			t.Fatalf("parity case for tool %q route %q was not recorded by an executable assertion", name, tc.HTTPRoute)
		}
	}
}

func expectedHTTPMCPParityCases(t *testing.T) (map[string]httpMCPParityCase, []string) {
	t.Helper()

	seenCases := make(map[string]httpMCPParityCase, len(mcpHTTPParityCases))
	for _, tc := range mcpHTTPParityCases {
		if tc.Tool == "" || tc.HTTPRoute == "" || tc.Assertion == "" {
			t.Fatalf("parity case must declare tool, HTTP route, and assertion text: %#v", tc)
		}
		if prior, exists := seenCases[tc.Tool]; exists {
			t.Fatalf("duplicate parity case for %s: %#v and %#v", tc.Tool, prior, tc)
		}
		seenCases[tc.Tool] = tc
	}

	expected := append([]string{}, baseToolNames...)
	expected = append(expected, wikimcp.RevisionToolNames()...)
	expected = append(expected, wikimcp.LinkRefactorToolNames()...)
	sort.Strings(expected)

	for _, name := range expected {
		_, exists := seenCases[name]
		if !exists {
			t.Fatalf("missing HTTP/MCP parity case for tool %q", name)
		}
	}
	return seenCases, expected
}

var baseToolInputRequiredProperties = map[string][]string{
	"get_config":            {},
	"get_current_user":      {},
	"get_tree":              {},
	"get_page":              {},
	"get_page_by_path":      {"path"},
	"lookup_path":           {"path"},
	"resolve_permalink":     {},
	"suggest_slug":          {"title"},
	"create_page":           {"title", "slug"},
	"update_page":           {"id", "version", "title", "slug"},
	"delete_page":           {"id", "version"},
	"move_page":             {"id", "version"},
	"sort_pages":            {"parentId", "orderedIds"},
	"ensure_page":           {"path", "title"},
	"convert_page":          {"id", "version", "targetKind"},
	"copy_page":             {"id", "title", "slug"},
	"search_pages":          {},
	"get_search_status":     {},
	"list_tags":             {},
	"get_pages_by_tags":     {"tags"},
	"list_property_keys":    {},
	"get_pages_by_property": {"key", "value"},
	"get_link_status":       {},
	"upload_asset":          {"pageId", "filename", "contentBase64"},
	"get_asset":             {"pageId", "filename"},
	"list_assets":           {},
	"rename_asset":          {"pageId", "oldFilename", "newFilename"},
	"delete_asset":          {"pageId", "filename"},
}

var featureToolInputRequiredProperties = map[string][]string{
	"list_revisions":        {},
	"get_latest_revision":   {},
	"get_revision":          {"revisionId"},
	"compare_revisions":     {"baseRevisionId", "targetRevisionId"},
	"get_revision_asset":    {"revisionId", "assetName"},
	"restore_revision":      {"revisionId"},
	"preview_page_refactor": {"kind"},
	"apply_page_refactor":   {"version", "kind"},
}

var baseToolInputAlternativeRequiredProperties = map[string][]string{
	"get_page":          {"id", "pageId"},
	"resolve_permalink": {"id", "pageId"},
	"get_link_status":   {"id", "pageId"},
	"list_assets":       {"id", "pageId"},
}

var featureToolInputAlternativeRequiredProperties = map[string][]string{
	"list_revisions":        {"id", "pageId"},
	"get_latest_revision":   {"id", "pageId"},
	"get_revision":          {"id", "pageId"},
	"compare_revisions":     {"id", "pageId"},
	"get_revision_asset":    {"id", "pageId"},
	"restore_revision":      {"id", "pageId"},
	"preview_page_refactor": {"id", "pageId"},
	"apply_page_refactor":   {"id", "pageId"},
}

var baseToolOutputProperties = map[string][]string{
	"get_config":            {"publicAccess", "hideLinkMetadataSection", "authDisabled", "basePath", "maxAssetUploadSizeBytes", "enableRevision", "enableLinkRefactor", "httpRemoteUserEnabled", "httpRemoteUserLogoutUrl"},
	"get_current_user":      {"user"},
	"get_tree":              {"tree"},
	"get_page":              {"page"},
	"get_page_by_path":      {"page"},
	"lookup_path":           {"lookup"},
	"resolve_permalink":     {"target"},
	"suggest_slug":          {"slug"},
	"create_page":           {"page"},
	"update_page":           {"page"},
	"delete_page":           {"message"},
	"move_page":             {"message"},
	"sort_pages":            {"message"},
	"ensure_page":           {"page"},
	"convert_page":          {"message"},
	"copy_page":             {"page"},
	"search_pages":          {"count", "items", "limit", "offset", "tagFacets", "hasMore"},
	"get_search_status":     {"status"},
	"list_tags":             {"tags"},
	"get_pages_by_tags":     {"pages"},
	"list_property_keys":    {"keys"},
	"get_pages_by_property": {"pages"},
	"get_link_status":       {"status"},
	"upload_asset":          {"file"},
	"get_asset":             {"filename", "mimeType", "contentBase64"},
	"list_assets":           {"files"},
	"rename_asset":          {"url"},
	"delete_asset":          {"message"},
}

var featureToolOutputProperties = map[string][]string{
	"list_revisions":        {"revisions", "nextCursor"},
	"get_latest_revision":   {"revision"},
	"get_revision":          {"revision", "content", "assets"},
	"compare_revisions":     {"base", "target", "contentChanged", "assetChanges"},
	"get_revision_asset":    {"filename", "mimeType", "contentBase64"},
	"restore_revision":      {"page"},
	"preview_page_refactor": {"kind", "pageId", "oldPath", "newPath", "affectedPages", "counts", "warnings"},
	"apply_page_refactor":   {"page"},
}

func TestLocalMCPRegistration_DisabledByDefaultAndToolListMatchesPlan(t *testing.T) {
	w := newLocalMCPTestWiki(t, false)

	embedFrontendOrig := httpinternal.EmbedFrontend
	httpinternal.EmbedFrontend = "true"
	t.Cleanup(func() {
		httpinternal.EmbedFrontend = embedFrontendOrig
	})

	disabledRouter := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
	})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		disabledRouter.ServeHTTP(rec, httptest.NewRequest(method, "/mcp", strings.NewReader("{}")))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s /mcp without MCP enabled = %d, want 404", method, rec.Code)
		}
	}
	for _, path := range []string{"/mcp/", "/mcp/anything"} {
		rec := httptest.NewRecorder()
		disabledRouter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s without MCP enabled = %d, want 404", path, rec.Code)
		}
	}

	authEnabledRouter := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            false,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
	})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		authEnabledRouter.ServeHTTP(rec, httptest.NewRequest(method, "/mcp", strings.NewReader("{}")))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s /mcp with auth enabled = %d, want 404", method, rec.Code)
		}
	}

	remoteUserRouter := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		HTTPRemoteUser:          httpinternal.HTTPRemoteUserConfig{Enabled: true},
	})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		remoteUserRouter.ServeHTTP(rec, httptest.NewRequest(method, "/mcp", strings.NewReader("{}")))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s /mcp with remote-user middleware = %d, want 404", method, rec.Code)
		}
	}

	missingHostRouter := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
	})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		missingHostRouter.ServeHTTP(rec, httptest.NewRequest(method, "/mcp", strings.NewReader("{}")))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s /mcp without validated loopback host = %d, want 404", method, rec.Code)
		}
	}
	nonLoopbackRouter := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPBindHost:             "0.0.0.0",
	})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		nonLoopbackRouter.ServeHTTP(rec, httptest.NewRequest(method, "/mcp", strings.NewReader("{}")))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s /mcp with non-loopback host = %d, want 404", method, rec.Code)
		}
	}

	enabledRouter := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPToolListPageSize:     5,
	})
	session := connectLocalMCP(t, enabledRouter, "/mcp")

	firstPage, err := session.ListTools(context.Background(), &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools first page failed: %v", err)
	}
	if len(firstPage.Tools) != 5 {
		t.Fatalf("first ListTools page length = %d, want 5", len(firstPage.Tools))
	}
	if firstPage.NextCursor == "" {
		t.Fatalf("first ListTools page did not include next cursor")
	}

	got := listAllToolNames(t, session)
	assertToolNames(t, got, baseToolNames)

	tools := listAllTools(t, session)
	assertInputSchemasMatch(t, tools, baseToolInputProperties, baseToolInputRequiredProperties, baseToolInputAlternativeRequiredProperties)
	assertOutputSchemasMatch(t, tools, baseToolOutputProperties)

	typeErr := callToolError(t, session, "get_page", map[string]any{"id": float64(12)})
	if !strings.Contains(strings.ToLower(typeErr), "validating") && !strings.Contains(strings.ToLower(typeErr), "string") {
		t.Fatalf("get_page invalid type error = %q, want schema validation detail", typeErr)
	}

	for _, name := range got {
		if strings.HasPrefix(name, "leafwiki_") {
			t.Fatalf("tool %q has forbidden leafwiki_ prefix", name)
		}
	}
	for _, forbidden := range []string{
		"create_import_plan",
		"get_import_plan",
		"execute_import_plan",
		"clear_import_plan",
		"get_branding",
		"get_branding_asset",
		"get_favicon",
		"login",
		"logout",
		"create_user",
	} {
		if contains(got, forbidden) {
			t.Fatalf("forbidden tool %q was registered in MCP tool list", forbidden)
		}
	}
}

func TestLocalMCPRegistration_FeatureGatedTools(t *testing.T) {
	revisionTools := wikimcp.RevisionToolNames()
	refactorTools := wikimcp.LinkRefactorToolNames()

	tests := []struct {
		name               string
		enableRevision     bool
		enableLinkRefactor bool
		wantFeatureTools   []string
	}{
		{name: "none"},
		{name: "revision only", enableRevision: true, wantFeatureTools: revisionTools},
		{name: "link refactor only", enableLinkRefactor: true, wantFeatureTools: refactorTools},
		{name: "both", enableRevision: true, enableLinkRefactor: true, wantFeatureTools: append(append([]string{}, revisionTools...), refactorTools...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newLocalMCPTestWiki(t, tt.enableRevision)
			router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
				AuthDisabled:            true,
				PublicAccess:            true,
				AllowInsecure:           true,
				MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
				EnableRevision:          tt.enableRevision,
				EnableLinkRefactor:      tt.enableLinkRefactor,
				MCPEnabled:              true,
				MCPToolListPageSize:     200,
			})
			session := connectLocalMCP(t, router, "/mcp")

			want := append([]string{}, baseToolNames...)
			want = append(want, tt.wantFeatureTools...)
			assertToolNames(t, listAllToolNames(t, session), want)

			tools := listAllTools(t, session)
			expectedSchemas := copyToolInputProperties(baseToolInputProperties)
			expectedRequired := copyToolInputProperties(baseToolInputRequiredProperties)
			expectedAlternatives := copyToolInputProperties(baseToolInputAlternativeRequiredProperties)
			expectedOutputs := copyToolInputProperties(baseToolOutputProperties)
			for _, name := range tt.wantFeatureTools {
				expectedSchemas[name] = featureToolInputProperties[name]
				expectedRequired[name] = featureToolInputRequiredProperties[name]
				if props, ok := featureToolInputAlternativeRequiredProperties[name]; ok {
					expectedAlternatives[name] = props
				}
				expectedOutputs[name] = featureToolOutputProperties[name]
			}
			assertInputSchemasMatch(t, tools, expectedSchemas, expectedRequired, expectedAlternatives)
			assertOutputSchemasMatch(t, tools, expectedOutputs)
		})
	}
}

func TestLocalMCPRegistration_RespectsBasePath(t *testing.T) {
	w := newLocalMCPTestWiki(t, false)
	router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		BasePath:                "/wiki",
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	})

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, "/mcp", strings.NewReader("{}")))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s /mcp without base path = %d, want 404", method, rec.Code)
		}
	}

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, "/wiki/mcp", strings.NewReader("{}")))
		if rec.Code == http.StatusNotFound {
			t.Fatalf("%s /wiki/mcp = 404, want route mounted", method)
		}
	}

	session := connectLocalMCP(t, router, "/wiki/mcp")
	assertToolNames(t, listAllToolNames(t, session), baseToolNames)
}

func TestLocalMCPProtocol_PageMutationParity(t *testing.T) {
	runLocalMCPProtocolPageMutationParity(t)
}

func runLocalMCPProtocolPageMutationParity(t *testing.T) {
	w, storageDir := newLocalMCPTestWikiWithStorage(t, false)
	router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	})
	session := connectLocalMCP(t, router, "/mcp")

	invalidCreateKindErr := callToolError(t, session, "create_page", map[string]any{
		"title": "Invalid Kind",
		"slug":  "invalid-kind",
		"kind":  "folder",
	})
	assertErrorContainsAny(t, "MCP create_page invalid kind", invalidCreateKindErr, "page_invalid_kind", "invalid kind")
	assertErrorDoesNotContainAny(t, "MCP create_page invalid kind", invalidCreateKindErr, "enum", "validating")
	paddedCreateKindErr := callToolError(t, session, "create_page", map[string]any{
		"title": "Padded Kind",
		"slug":  "padded-kind",
		"kind":  " page ",
	})
	assertErrorContainsAny(t, "MCP create_page padded kind", paddedCreateKindErr, "page_invalid_kind", "invalid kind")
	assertErrorDoesNotContainAny(t, "MCP create_page padded kind", paddedCreateKindErr, "enum", "validating")
	invalidCreateKindHTTP := postHTTPJSONBody(t, router, "/api/pages", map[string]any{
		"title": "Invalid Kind HTTP",
		"slug":  "invalid-kind-http",
		"kind":  "folder",
	}, http.StatusBadRequest)
	if !strings.Contains(invalidCreateKindHTTP, "page_invalid_kind") {
		t.Fatalf("HTTP create_page invalid kind error = %q, want page_invalid_kind", invalidCreateKindHTTP)
	}
	paddedCreateKindHTTP := postHTTPJSONBody(t, router, "/api/pages", map[string]any{
		"title": "Padded Kind HTTP",
		"slug":  "padded-kind-http",
		"kind":  " page ",
	}, http.StatusBadRequest)
	if !strings.Contains(paddedCreateKindHTTP, "page_invalid_kind") {
		t.Fatalf("HTTP create_page padded kind error = %q, want page_invalid_kind", paddedCreateKindHTTP)
	}
	nullKindCreated := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Null Kind",
		"slug":  "null-kind",
		"kind":  nil,
	}), "page")
	if got := nullKindCreated["kind"]; got != "page" {
		t.Fatalf("MCP create_page null kind = %v, want page", got)
	}

	created := callToolStructured(t, session, "create_page", map[string]any{
		"title": "MCP Draft",
		"slug":  "mcp-draft",
		"kind":  "page",
	})
	createdPage := nestedMap(t, created, "page")
	pageID := stringField(t, createdPage, "id")
	version := stringField(t, createdPage, "version")

	httpPage := getHTTPPageByPath(t, router, "mcp-draft")
	if httpPage["id"] != pageID {
		t.Fatalf("HTTP page id = %v, want MCP-created id %q", httpPage["id"], pageID)
	}
	assertJSONEqual(t, "create_page HTTP page", createdPage, getHTTPPageByID(t, router, pageID))

	mcpCreateParent := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "MCP Create Parent",
		"slug":  "mcp-create-parent",
		"kind":  "section",
	}), "page")
	httpCreateParent := postHTTPJSON(t, router, "/api/pages", map[string]any{
		"title": "HTTP Create Parent",
		"slug":  "http-create-parent",
		"kind":  "section",
	}, http.StatusCreated)
	whitespaceParentCreateErr := callToolError(t, session, "create_page", map[string]any{
		"parentId": " ",
		"title":    "Whitespace Parent",
		"slug":     "whitespace-parent",
		"kind":     "page",
	})
	assertErrorContainsAny(t, "MCP create_page whitespace parentId", whitespaceParentCreateErr, "page_invalid_parent_id", "parent")
	whitespaceParentCreateHTTP := postHTTPJSONBody(t, router, "/api/pages", map[string]any{
		"parentId": " ",
		"title":    "Whitespace Parent HTTP",
		"slug":     "whitespace-parent-http",
		"kind":     "page",
	}, http.StatusBadRequest)
	if !strings.Contains(whitespaceParentCreateHTTP, "page_invalid_parent_id") {
		t.Fatalf("HTTP create_page whitespace parentId error = %q, want page_invalid_parent_id", whitespaceParentCreateHTTP)
	}
	paddedParentCreateErr := callToolError(t, session, "create_page", map[string]any{
		"parentId": " " + stringField(t, mcpCreateParent, "id") + " ",
		"title":    "Padded Parent",
		"slug":     "padded-parent",
		"kind":     "page",
	})
	assertErrorContainsAny(t, "MCP create_page padded parentId", paddedParentCreateErr, "page_invalid_parent_id", "parent")
	paddedParentCreateHTTP := postHTTPJSONBody(t, router, "/api/pages", map[string]any{
		"parentId": " " + stringField(t, httpCreateParent, "id") + " ",
		"title":    "Padded Parent HTTP",
		"slug":     "padded-parent-http",
		"kind":     "page",
	}, http.StatusBadRequest)
	if !strings.Contains(paddedParentCreateHTTP, "page_invalid_parent_id") {
		t.Fatalf("HTTP create_page padded parentId error = %q, want page_invalid_parent_id", paddedParentCreateHTTP)
	}
	mcpCreatedChild := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"parentId": stringField(t, mcpCreateParent, "id"),
		"title":    "Created Child",
		"slug":     "created-child",
		"kind":     "section",
	}), "page")
	httpCreatedChild := postHTTPJSON(t, router, "/api/pages", map[string]any{
		"parentId": stringField(t, httpCreateParent, "id"),
		"title":    "Created Child",
		"slug":     "created-child",
		"kind":     "section",
	}, http.StatusCreated)
	assertPageState(t, "MCP create_page child", getHTTPPageByPath(t, router, "mcp-create-parent/created-child"), stringField(t, mcpCreatedChild, "id"), "Created Child", "created-child", "mcp-create-parent/created-child", "section", "")
	assertPageState(t, "HTTP create_page child", getHTTPPageByPath(t, router, "http-create-parent/created-child"), stringField(t, httpCreatedChild, "id"), "Created Child", "created-child", "http-create-parent/created-child", "section", "")
	recordHTTPMCPParity(t, "create_page", "POST /api/pages")

	content := "Hello from MCP\n"
	updated := callToolStructured(t, session, "update_page", map[string]any{
		"id":      pageID,
		"version": version,
		"title":   "MCP Draft Updated",
		"slug":    "mcp-draft",
		"content": content,
		"tags":    []any{"mcp", "Parity"},
		"properties": map[string]any{
			"status": "draft",
		},
	})
	updatedPage := nestedMap(t, updated, "page")
	if got := updatedPage["content"]; got != content {
		t.Fatalf("updated MCP content = %v, want %q", got, content)
	}

	httpPage = getHTTPPageByPath(t, router, "mcp-draft")
	if got := httpPage["title"]; got != "MCP Draft Updated" {
		t.Fatalf("HTTP title after MCP update = %v, want updated title", got)
	}
	if got := httpPage["content"]; got != content {
		t.Fatalf("HTTP content after MCP update = %v, want %q", got, content)
	}
	assertJSONEqual(t, "update_page HTTP page", updatedPage, getHTTPPageByID(t, router, pageID))
	if got := stringSliceField(t, httpPage, "tags"); strings.Join(got, ",") != "mcp,parity" {
		t.Fatalf("HTTP tags after MCP update = %v, want [mcp parity]", got)
	}
	props := nestedMap(t, httpPage, "properties")
	if got := props["status"]; got != "draft" {
		t.Fatalf("HTTP properties.status after MCP update = %v, want draft", got)
	}
	rawMCPMetadata := readPageMarkdownByRoutePath(t, storageDir, "mcp-draft")
	if !strings.Contains(rawMCPMetadata, "tags:") || !strings.Contains(rawMCPMetadata, "- mcp") || !strings.Contains(rawMCPMetadata, "status: draft") {
		t.Fatalf("MCP update raw markdown missing metadata frontmatter:\n%s", rawMCPMetadata)
	}
	httpMetadataPage := postHTTPJSON(t, router, "/api/pages", map[string]any{
		"title": "HTTP Metadata",
		"slug":  "http-metadata",
		"kind":  "page",
	}, http.StatusCreated)
	httpMetadataUpdated := updateHTTPPage(t, router, stringField(t, httpMetadataPage, "id"), map[string]any{
		"version": stringField(t, httpMetadataPage, "version"),
		"title":   "HTTP Metadata Updated",
		"slug":    "http-metadata",
		"content": "HTTP metadata content\n",
		"tags":    []string{"HTTP", "Metadata"},
		"properties": map[string]string{
			"status": "review",
		},
	})
	if got := strings.Join(stringSliceField(t, httpMetadataUpdated, "tags"), ","); got != "http,metadata" {
		t.Fatalf("HTTP update tags = %v, want http,metadata", got)
	}
	httpMetadataProps := nestedMap(t, httpMetadataUpdated, "properties")
	if got := httpMetadataProps["status"]; got != "review" {
		t.Fatalf("HTTP update properties.status = %v, want review", got)
	}
	rawHTTPMetadata := readPageMarkdownByRoutePath(t, storageDir, "http-metadata")
	if !strings.Contains(rawHTTPMetadata, "tags:") || !strings.Contains(rawHTTPMetadata, "- http") || !strings.Contains(rawHTTPMetadata, "status: review") {
		t.Fatalf("HTTP update raw markdown missing metadata frontmatter:\n%s", rawHTTPMetadata)
	}
	recordHTTPMCPParity(t, "update_page", "PUT /api/pages/:id")

	metadataErr := callToolError(t, session, "update_page", map[string]any{
		"id":      pageID,
		"version": stringField(t, updatedPage, "version"),
		"title":   "MCP Draft Updated",
		"slug":    "mcp-draft",
		"content": content,
		"tags":    []any{"mcp", "MCP"},
		"properties": map[string]any{
			"leafwiki_hidden": "forbidden",
		},
	})
	if !strings.Contains(strings.ToLower(metadataErr), "validation") && !strings.Contains(strings.ToLower(metadataErr), "reserved") {
		t.Fatalf("MCP metadata validation error = %q, want validation detail", metadataErr)
	}

	csrfToken, csrfCookies := issueHTTPCSRF(t, router)
	staleHTTPBody := strings.NewReader(`{"version":"` + version + `","title":"MCP Draft Stale","slug":"mcp-draft","content":"stale"}`)
	staleHTTPReq := httptest.NewRequest(http.MethodPut, "/api/pages/"+pageID, staleHTTPBody)
	staleHTTPReq.Header.Set("Content-Type", "application/json")
	staleHTTPReq.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range csrfCookies {
		staleHTTPReq.AddCookie(cookie)
	}
	staleHTTPRec := httptest.NewRecorder()
	router.ServeHTTP(staleHTTPRec, staleHTTPReq)
	if staleHTTPRec.Code != http.StatusConflict {
		t.Fatalf("HTTP stale update status = %d, want 409: %s", staleHTTPRec.Code, staleHTTPRec.Body.String())
	}

	mcpErr := callToolError(t, session, "update_page", map[string]any{
		"id":      pageID,
		"version": version,
		"title":   "MCP Draft Stale",
		"slug":    "mcp-draft",
		"content": "stale",
	})
	assertPageVersionConflictParity(t, "stale update_page", mcpErr, staleHTTPRec.Body.String())

	search := callToolStructured(t, session, "search_pages", map[string]any{
		"q":      "Hello",
		"offset": float64(0),
		"limit":  float64(10),
	})
	httpSearch := getHTTPSearch(t, router, url.Values{
		"q":      {"Hello"},
		"offset": {"0"},
		"limit":  {"10"},
	})
	assertSearchResultsMatch(t, search, httpSearch)
	if got := search["count"]; got != float64(1) {
		t.Fatalf("MCP search count = %v, want 1", got)
	}
	items, ok := search["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("MCP search items = %#v, want one item", search["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("MCP search item has type %T", items[0])
	}
	if got := item["page_id"]; got != pageID {
		t.Fatalf("MCP search page_id = %v, want %q", got, pageID)
	}
	tagSearch := callToolStructured(t, session, "search_pages", map[string]any{
		"tags":   []any{"mcp"},
		"offset": float64(0),
		"limit":  float64(10),
	})
	tagHTTPSearch := getHTTPSearch(t, router, url.Values{
		"tags":   {"mcp"},
		"offset": {"0"},
		"limit":  {"10"},
	})
	assertSearchResultsMatch(t, tagSearch, tagHTTPSearch)

	for i := 1; i <= 2; i++ {
		extra := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
			"title": "Hello Extra " + strconv.Itoa(i),
			"slug":  "hello-extra-" + strconv.Itoa(i),
			"kind":  "page",
		}), "page")
		extraContent := "Hello paginated search " + strconv.Itoa(i)
		callToolStructured(t, session, "update_page", map[string]any{
			"id":      stringField(t, extra, "id"),
			"version": stringField(t, extra, "version"),
			"title":   extra["title"],
			"slug":    extra["slug"],
			"content": extraContent,
			"tags":    []any{"mcp"},
		})
	}
	paginatedSearch := callToolStructured(t, session, "search_pages", map[string]any{
		"q":      "Hello",
		"offset": float64(0),
		"limit":  float64(1),
	})
	paginatedHTTP := getHTTPSearch(t, router, url.Values{
		"q":      {"Hello"},
		"offset": {"0"},
		"limit":  {"1"},
	})
	assertSearchResultsMatch(t, paginatedSearch, paginatedHTTP)
	if paginatedSearch["hasMore"] != true {
		t.Fatalf("paginated MCP search hasMore = %v, want true", paginatedSearch["hasMore"])
	}
	recordHTTPMCPParity(t, "search_pages", "GET /api/search")
}

func TestLocalMCPProtocol_PageOperationParity(t *testing.T) {
	runLocalMCPProtocolPageOperationParity(t)
}

func runLocalMCPProtocolPageOperationParity(t *testing.T) {
	w := newLocalMCPTestWiki(t, false)
	router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	})
	session := connectLocalMCP(t, router, "/mcp")

	current := callToolStructured(t, session, "get_current_user", nil)
	user := nestedMap(t, current, "user")
	if user["username"] != "public-editor" || user["role"] != "editor" {
		t.Fatalf("current user = %#v, want public-editor editor", user)
	}
	httpUser := getHTTPMap(t, router, "/api/auth/me")
	assertJSONEqual(t, "get_current_user", user, httpUser)
	recordHTTPMCPParity(t, "get_current_user", "GET /api/auth/me")
	config := callToolStructured(t, session, "get_config", nil)
	if config["authDisabled"] != true {
		t.Fatalf("config authDisabled = %v, want true", config["authDisabled"])
	}
	if config["maxAssetUploadSizeBytes"] != float64(assets.DefaultMaxUploadSizeBytes) {
		t.Fatalf("config maxAssetUploadSizeBytes = %v, want default", config["maxAssetUploadSizeBytes"])
	}
	httpConfig := getHTTPMap(t, router, "/api/config")
	assertMapFieldsEqual(t, "get_config", config, httpConfig, []string{
		"publicAccess",
		"hideLinkMetadataSection",
		"authDisabled",
		"basePath",
		"maxAssetUploadSizeBytes",
		"enableRevision",
		"enableLinkRefactor",
		"httpRemoteUserEnabled",
		"httpRemoteUserLogoutUrl",
	})
	recordHTTPMCPParity(t, "get_config", "GET /api/config")

	slug := callToolStructured(t, session, "suggest_slug", map[string]any{"title": "Parent Section"})
	httpSlug := getHTTPMap(t, router, "/api/pages/slug-suggestion?title=Parent+Section")
	assertJSONEqual(t, "suggest_slug", slug, httpSlug)
	recordHTTPMCPParity(t, "suggest_slug", "GET /api/pages/slug-suggestion")
	if got := slug["slug"]; got != "parent-section" {
		t.Fatalf("suggest_slug = %v, want parent-section", got)
	}
	blankSlugErr := callToolError(t, session, "suggest_slug", map[string]any{"title": "   "})
	if !strings.Contains(strings.ToLower(blankSlugErr), "title") {
		t.Fatalf("MCP blank suggest_slug error = %q, want title detail", blankSlugErr)
	}
	blankSlugHTTP := getHTTPStatus(t, router, "/api/pages/slug-suggestion?title=+++",
		http.StatusBadRequest)
	if !strings.Contains(blankSlugHTTP, wikipages.ErrCodePageMissingTitle) {
		t.Fatalf("HTTP blank suggest_slug error = %q, want %s", blankSlugHTTP, wikipages.ErrCodePageMissingTitle)
	}
	punctuationSlugErr := callToolError(t, session, "suggest_slug", map[string]any{"title": "!!!"})
	if !strings.Contains(strings.ToLower(punctuationSlugErr), "title") {
		t.Fatalf("MCP punctuation-only suggest_slug error = %q, want title detail", punctuationSlugErr)
	}
	punctuationSlugHTTP := getHTTPStatus(t, router, "/api/pages/slug-suggestion?title=%21%21%21",
		http.StatusBadRequest)
	if !strings.Contains(punctuationSlugHTTP, wikipages.ErrCodePageInvalidTitle) {
		t.Fatalf("HTTP punctuation-only suggest_slug error = %q, want %s", punctuationSlugHTTP, wikipages.ErrCodePageInvalidTitle)
	}

	parent := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Parent Section",
		"slug":  "parent-section",
		"kind":  "section",
	}), "page")
	parentID := stringField(t, parent, "id")
	parentViaGet := nestedMap(t, callToolStructured(t, session, "get_page", map[string]any{"id": parentID}), "page")
	assertJSONEqual(t, "get_page", parentViaGet, getHTTPPageByID(t, router, parentID))
	recordHTTPMCPParity(t, "get_page", "GET /api/pages/:id")
	treeResult := callToolStructured(t, session, "get_tree", map[string]any{"depth": float64(1)})
	if treeResult["tree"] == nil {
		t.Fatalf("get_tree returned no tree: %#v", treeResult)
	}
	httpTree := getHTTPMap(t, router, "/api/tree?depth=1")
	assertJSONEqual(t, "get_tree", treeResult["tree"], httpTree)
	recordHTTPMCPParity(t, "get_tree", "GET /api/tree")

	pageByPath := nestedMap(t, callToolStructured(t, session, "get_page_by_path", map[string]any{"path": "parent-section"}), "page")
	httpPageByPath := getHTTPPageByPath(t, router, "parent-section")
	assertJSONEqual(t, "get_page_by_path", pageByPath, httpPageByPath)
	recordHTTPMCPParity(t, "get_page_by_path", "GET /api/pages/by-path")
	blankPathErr := callToolError(t, session, "get_page_by_path", map[string]any{"path": "  "})
	if !strings.Contains(blankPathErr, wikipages.ErrCodePageMissingPath) && !strings.Contains(strings.ToLower(blankPathErr), "missing path") {
		t.Fatalf("MCP blank get_page_by_path error = %q, want %s", blankPathErr, wikipages.ErrCodePageMissingPath)
	}
	blankPathHTTP := getHTTPStatus(t, router, "/api/pages/by-path?path=++", http.StatusBadRequest)
	if !strings.Contains(blankPathHTTP, wikipages.ErrCodePageMissingPath) {
		t.Fatalf("HTTP blank get_page_by_path error = %q, want %s", blankPathHTTP, wikipages.ErrCodePageMissingPath)
	}
	for _, invalidPath := range []string{"docs//intro", "docs/.", "docs/..", `docs\..\secret`} {
		mcpPathErr := callToolError(t, session, "get_page_by_path", map[string]any{"path": invalidPath})
		if !strings.Contains(mcpPathErr, "page_invalid_path") && !strings.Contains(strings.ToLower(mcpPathErr), "invalid path") {
			t.Fatalf("MCP invalid get_page_by_path path %q error = %q, want invalid path detail", invalidPath, mcpPathErr)
		}
		httpPathErr := getHTTPStatus(t, router, "/api/pages/by-path?path="+url.QueryEscape(invalidPath), http.StatusBadRequest)
		if !strings.Contains(httpPathErr, "page_invalid_path") {
			t.Fatalf("HTTP invalid get_page_by_path path %q error = %q, want page_invalid_path", invalidPath, httpPathErr)
		}
	}

	childA := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"parentId": parentID,
		"title":    "Child A",
		"slug":     "child-a",
		"kind":     "page",
	}), "page")
	childB := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"parentId": parentID,
		"title":    "Child B",
		"slug":     "child-b",
		"kind":     "page",
	}), "page")

	callToolStructured(t, session, "sort_pages", map[string]any{
		"parentId":   parentID,
		"orderedIds": []any{stringField(t, childB, "id"), stringField(t, childA, "id")},
	})
	httpSort := putHTTPJSON(t, router, "/api/pages/"+parentID+"/sort", map[string]any{
		"orderedIds": []string{stringField(t, childB, "id"), stringField(t, childA, "id")},
	}, http.StatusOK)
	mcpSort := callToolStructured(t, session, "sort_pages", map[string]any{
		"parentId":   parentID,
		"orderedIds": []any{stringField(t, childB, "id"), stringField(t, childA, "id")},
	})
	assertJSONEqual(t, "sort_pages", mcpSort, httpSort)
	parentAfterSort := nestedMap(t, callToolStructured(t, session, "get_page", map[string]any{"id": parentID}), "page")
	assertChildOrder(t, "sort_pages shared parent", parentAfterSort, stringField(t, childB, "id"), stringField(t, childA, "id"))
	mcpSortParent := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "MCP Sort Parent",
		"slug":  "mcp-sort-parent",
		"kind":  "section",
	}), "page")
	mcpSortA := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"parentId": stringField(t, mcpSortParent, "id"),
		"title":    "MCP Sort A",
		"slug":     "mcp-sort-a",
		"kind":     "page",
	}), "page")
	mcpSortB := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"parentId": stringField(t, mcpSortParent, "id"),
		"title":    "MCP Sort B",
		"slug":     "mcp-sort-b",
		"kind":     "page",
	}), "page")
	httpSortParent := postHTTPJSON(t, router, "/api/pages", map[string]any{
		"title": "HTTP Sort Parent",
		"slug":  "http-sort-parent",
		"kind":  "section",
	}, http.StatusCreated)
	httpSortA := postHTTPJSON(t, router, "/api/pages", map[string]any{
		"parentId": stringField(t, httpSortParent, "id"),
		"title":    "HTTP Sort A",
		"slug":     "http-sort-a",
		"kind":     "page",
	}, http.StatusCreated)
	httpSortB := postHTTPJSON(t, router, "/api/pages", map[string]any{
		"parentId": stringField(t, httpSortParent, "id"),
		"title":    "HTTP Sort B",
		"slug":     "http-sort-b",
		"kind":     "page",
	}, http.StatusCreated)
	callToolStructured(t, session, "sort_pages", map[string]any{
		"parentId":   stringField(t, mcpSortParent, "id"),
		"orderedIds": []any{stringField(t, mcpSortB, "id"), stringField(t, mcpSortA, "id")},
	})
	putHTTPJSON(t, router, "/api/pages/"+stringField(t, httpSortParent, "id")+"/sort", map[string]any{
		"orderedIds": []string{stringField(t, httpSortB, "id"), stringField(t, httpSortA, "id")},
	}, http.StatusOK)
	assertChildOrder(t, "MCP sort_pages parent", getHTTPPageByPath(t, router, "mcp-sort-parent"), stringField(t, mcpSortB, "id"), stringField(t, mcpSortA, "id"))
	assertChildOrder(t, "HTTP sort_pages parent", getHTTPPageByPath(t, router, "http-sort-parent"), stringField(t, httpSortB, "id"), stringField(t, httpSortA, "id"))
	recordHTTPMCPParity(t, "sort_pages", "PUT /api/pages/:id/sort")
	parentByAlias := nestedMap(t, callToolStructured(t, session, "get_page", map[string]any{"pageId": parentID}), "page")
	if parentByAlias["id"] != parentID {
		t.Fatalf("get_page pageId alias returned id = %v, want %q", parentByAlias["id"], parentID)
	}

	ensured := nestedMap(t, callToolStructured(t, session, "ensure_page", map[string]any{
		"path":  "parent-section/ensured",
		"title": "Ensured Page",
		"kind":  "page",
	}), "page")
	ensuredID := stringField(t, ensured, "id")
	httpEnsured := postHTTPJSON(t, router, "/api/pages/ensure", map[string]any{
		"path":  "parent-section/ensured",
		"title": "Ensured Page",
		"kind":  "page",
	}, http.StatusOK)
	assertJSONEqual(t, "ensure_page", ensured, httpEnsured)
	mcpEnsuredIndependent := nestedMap(t, callToolStructured(t, session, "ensure_page", map[string]any{
		"path":  "parent-section/ensured-mcp",
		"title": "Ensured Independent",
		"kind":  "section",
	}), "page")
	httpEnsuredIndependent := postHTTPJSON(t, router, "/api/pages/ensure", map[string]any{
		"path":  "parent-section/ensured-http",
		"title": "Ensured Independent",
		"kind":  "section",
	}, http.StatusOK)
	assertPageState(t, "MCP ensure_page independent", getHTTPPageByPath(t, router, "parent-section/ensured-mcp"), stringField(t, mcpEnsuredIndependent, "id"), "Ensured Independent", "ensured-mcp", "parent-section/ensured-mcp", "section", "")
	assertPageState(t, "HTTP ensure_page independent", getHTTPPageByPath(t, router, "parent-section/ensured-http"), stringField(t, httpEnsuredIndependent, "id"), "Ensured Independent", "ensured-http", "parent-section/ensured-http", "section", "")
	nullKindEnsured := nestedMap(t, callToolStructured(t, session, "ensure_page", map[string]any{
		"path":  "parent-section/ensured-null-kind",
		"title": "Ensured Null Kind",
		"kind":  nil,
	}), "page")
	if got := nullKindEnsured["kind"]; got != "page" {
		t.Fatalf("MCP ensure_page null kind = %v, want page", got)
	}
	recordHTTPMCPParity(t, "ensure_page", "POST /api/pages/ensure")

	lookup := callToolStructured(t, session, "lookup_path", map[string]any{"path": "parent-section/ensured"})
	httpLookup := getHTTPMap(t, router, "/api/pages/lookup?path=parent-section%2Fensured")
	assertJSONEqual(t, "lookup_path", lookup["lookup"], httpLookup)
	recordHTTPMCPParity(t, "lookup_path", "GET /api/pages/lookup")
	if lookup["lookup"] == nil {
		t.Fatalf("lookup_path returned no lookup: %#v", lookup)
	}
	permalink := callToolStructured(t, session, "resolve_permalink", map[string]any{"id": ensuredID})
	httpPermalink := getHTTPMap(t, router, "/api/pages/permalink/"+ensuredID)
	assertJSONEqual(t, "resolve_permalink", permalink["target"], httpPermalink)
	recordHTTPMCPParity(t, "resolve_permalink", "GET /api/pages/permalink/:id")
	target := nestedMap(t, permalink, "target")
	if got := target["path"]; got != "parent-section/ensured" {
		t.Fatalf("resolve_permalink path = %v, want parent-section/ensured", got)
	}
	permalinkByAlias := callToolStructured(t, session, "resolve_permalink", map[string]any{"pageId": ensuredID})
	targetByAlias := nestedMap(t, permalinkByAlias, "target")
	if got := targetByAlias["path"]; got != "parent-section/ensured" {
		t.Fatalf("resolve_permalink pageId alias path = %v, want parent-section/ensured", got)
	}

	invalidEnsureKindErr := callToolError(t, session, "ensure_page", map[string]any{
		"path":  "parent-section/invalid-kind",
		"title": "Invalid Ensure Kind",
		"kind":  "folder",
	})
	assertErrorContainsAny(t, "MCP ensure_page invalid kind", invalidEnsureKindErr, "page_invalid_kind", "invalid kind", "enum")
	invalidEnsureKindHTTP := postHTTPJSONBody(t, router, "/api/pages/ensure", map[string]any{
		"path":  "parent-section/invalid-kind-http",
		"title": "Invalid Ensure Kind HTTP",
		"kind":  "folder",
	}, http.StatusBadRequest)
	if !strings.Contains(invalidEnsureKindHTTP, "page_invalid_kind") {
		t.Fatalf("HTTP ensure_page invalid kind error = %q, want page_invalid_kind", invalidEnsureKindHTTP)
	}

	mcpMove := callToolStructured(t, session, "move_page", map[string]any{
		"id":       stringField(t, childA, "id"),
		"version":  stringField(t, childA, "version"),
		"parentId": "",
	})
	httpMove := putHTTPJSON(t, router, "/api/pages/"+stringField(t, childB, "id")+"/move", map[string]any{
		"version":  stringField(t, childB, "version"),
		"parentId": "",
	}, http.StatusOK)
	assertJSONEqual(t, "move_page", mcpMove, httpMove)
	httpMoved := getHTTPPageByPath(t, router, "child-a")
	assertPageState(t, "MCP moved child A", httpMoved, stringField(t, childA, "id"), "Child A", "child-a", "child-a", "page", "")
	httpMovedB := getHTTPPageByPath(t, router, "child-b")
	assertPageState(t, "HTTP moved child B", httpMovedB, stringField(t, childB, "id"), "Child B", "child-b", "child-b", "page", "")
	parentAfterMove := getHTTPPageByPath(t, router, "parent-section")
	assertChildrenDoNotContain(t, "parent after move", parentAfterMove, stringField(t, childA, "id"), stringField(t, childB, "id"))
	staleMoveErr := callToolError(t, session, "move_page", map[string]any{
		"id":       stringField(t, childA, "id"),
		"version":  stringField(t, childA, "version"),
		"parentId": parentID,
	})
	staleMoveHTTP := putHTTPJSONBody(t, router, "/api/pages/"+stringField(t, childA, "id")+"/move", map[string]any{
		"version":  stringField(t, childA, "version"),
		"parentId": parentID,
	}, http.StatusConflict)
	assertPageVersionConflictParity(t, "stale move_page", staleMoveErr, staleMoveHTTP)
	mcpWhitespaceMove := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "MCP Whitespace Move",
		"slug":  "mcp-whitespace-move",
		"kind":  "page",
	}), "page")
	whitespaceMoveErr := callToolError(t, session, "move_page", map[string]any{
		"id":       stringField(t, mcpWhitespaceMove, "id"),
		"version":  stringField(t, mcpWhitespaceMove, "version"),
		"parentId": " ",
	})
	assertErrorContainsAny(t, "MCP whitespace move parentId", whitespaceMoveErr, "page_invalid_parent_id", "parent")
	httpWhitespaceMove := postHTTPJSON(t, router, "/api/pages", map[string]any{
		"title": "HTTP Whitespace Move",
		"slug":  "http-whitespace-move",
		"kind":  "page",
	}, http.StatusCreated)
	whitespaceMoveHTTP := putHTTPJSONBody(t, router, "/api/pages/"+stringField(t, httpWhitespaceMove, "id")+"/move", map[string]any{
		"version":  stringField(t, httpWhitespaceMove, "version"),
		"parentId": " ",
	}, http.StatusBadRequest)
	if !strings.Contains(whitespaceMoveHTTP, "page_invalid_parent_id") {
		t.Fatalf("HTTP whitespace move parentId error = %q, want page_invalid_parent_id", whitespaceMoveHTTP)
	}
	mcpMissingParentMove := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"parentId": parentID,
		"title":    "MCP Missing Parent Move",
		"slug":     "mcp-missing-parent-move",
		"kind":     "page",
	}), "page")
	missingParentMove := callToolStructured(t, session, "move_page", map[string]any{
		"id":      stringField(t, mcpMissingParentMove, "id"),
		"version": stringField(t, mcpMissingParentMove, "version"),
	})
	if missingParentMove["message"] != "Page moved" {
		t.Fatalf("MCP move_page missing parentId message = %v, want Page moved", missingParentMove["message"])
	}
	assertPageState(t, "MCP move_page missing parentId", getHTTPPageByPath(t, router, "mcp-missing-parent-move"), stringField(t, mcpMissingParentMove, "id"), "MCP Missing Parent Move", "mcp-missing-parent-move", "mcp-missing-parent-move", "page", "")
	recordHTTPMCPParity(t, "move_page", "PUT /api/pages/:id/move")

	convertMe := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Convert Me",
		"slug":  "convert-me",
		"kind":  "section",
	}), "page")
	mcpConvert := callToolStructured(t, session, "convert_page", map[string]any{
		"id":         stringField(t, convertMe, "id"),
		"version":    stringField(t, convertMe, "version"),
		"targetKind": "page",
	})
	if mcpConvert["message"] != "Page converted" {
		t.Fatalf("convert_page message = %v, want Page converted", mcpConvert["message"])
	}
	httpConverted := getHTTPPageByPath(t, router, "convert-me")
	if httpConverted["kind"] != "page" {
		t.Fatalf("converted kind = %v, want page", httpConverted["kind"])
	}
	assertPageState(t, "MCP converted page", httpConverted, stringField(t, convertMe, "id"), "Convert Me", "convert-me", "convert-me", "page", "")
	convertHTTP := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Convert HTTP",
		"slug":  "convert-http",
		"kind":  "section",
	}), "page")
	postHTTPJSONNoContent(t, router, "/api/pages/convert/"+stringField(t, convertHTTP, "id"), map[string]any{
		"version":    stringField(t, convertHTTP, "version"),
		"targetKind": "page",
	}, http.StatusNoContent)
	httpConvertedPeer := getHTTPPageByPath(t, router, "convert-http")
	assertPageState(t, "HTTP converted page", httpConvertedPeer, stringField(t, convertHTTP, "id"), "Convert HTTP", "convert-http", "convert-http", "page", "")
	staleConvertErr := callToolError(t, session, "convert_page", map[string]any{
		"id":         stringField(t, convertMe, "id"),
		"version":    stringField(t, convertMe, "version"),
		"targetKind": "section",
	})
	staleConvertHTTP := postHTTPJSONBody(t, router, "/api/pages/convert/"+stringField(t, convertMe, "id"), map[string]any{
		"version":    stringField(t, convertMe, "version"),
		"targetKind": "section",
	}, http.StatusConflict)
	assertPageVersionConflictParity(t, "stale convert_page", staleConvertErr, staleConvertHTTP)
	invalidConvertErr := callToolError(t, session, "convert_page", map[string]any{
		"id":         stringField(t, convertMe, "id"),
		"version":    stringField(t, httpConverted, "version"),
		"targetKind": "folder",
	})
	assertErrorContainsAny(t, "MCP invalid convert_page targetKind", invalidConvertErr, wikipages.ErrCodePageInvalidTargetKind, "invalid target kind", "targetkind")
	assertErrorDoesNotContainAny(t, "MCP invalid convert_page targetKind", invalidConvertErr, "enum", "validating")
	paddedConvertErr := callToolError(t, session, "convert_page", map[string]any{
		"id":         stringField(t, convertMe, "id"),
		"version":    stringField(t, httpConverted, "version"),
		"targetKind": " page ",
	})
	assertErrorContainsAny(t, "MCP padded convert_page targetKind", paddedConvertErr, wikipages.ErrCodePageInvalidTargetKind, "invalid target kind", "targetkind")
	assertErrorDoesNotContainAny(t, "MCP padded convert_page targetKind", paddedConvertErr, "enum", "validating")
	invalidConvertHTTP := postHTTPJSONBody(t, router, "/api/pages/convert/"+stringField(t, convertMe, "id"), map[string]any{
		"version":    stringField(t, httpConverted, "version"),
		"targetKind": "folder",
	}, http.StatusBadRequest)
	if !strings.Contains(invalidConvertHTTP, wikipages.ErrCodePageInvalidTargetKind) {
		t.Fatalf("HTTP invalid convert_page targetKind error = %q, want %s", invalidConvertHTTP, wikipages.ErrCodePageInvalidTargetKind)
	}
	paddedConvertHTTP := postHTTPJSONBody(t, router, "/api/pages/convert/"+stringField(t, convertMe, "id"), map[string]any{
		"version":    stringField(t, httpConverted, "version"),
		"targetKind": " page ",
	}, http.StatusBadRequest)
	if !strings.Contains(paddedConvertHTTP, wikipages.ErrCodePageInvalidTargetKind) {
		t.Fatalf("HTTP padded convert_page targetKind error = %q, want %s", paddedConvertHTTP, wikipages.ErrCodePageInvalidTargetKind)
	}
	recordHTTPMCPParity(t, "convert_page", "POST /api/pages/convert/:id")

	copied := nestedMap(t, callToolStructured(t, session, "copy_page", map[string]any{
		"id":    stringField(t, childA, "id"),
		"title": "Child A Copy",
		"slug":  "child-a-copy",
	}), "page")
	httpCopied := postHTTPJSON(t, router, "/api/pages/copy/"+stringField(t, childA, "id"), map[string]any{
		"title": "Child A Copy",
		"slug":  "child-a-http-copy",
	}, http.StatusCreated)
	whitespaceCopyErr := callToolError(t, session, "copy_page", map[string]any{
		"id":             stringField(t, childA, "id"),
		"targetParentId": " ",
		"title":          "Whitespace Copy",
		"slug":           "whitespace-copy",
	})
	assertErrorContainsAny(t, "MCP copy_page whitespace targetParentId", whitespaceCopyErr, "page_invalid_parent_id", "parent")
	whitespaceCopyHTTP := postHTTPJSONBody(t, router, "/api/pages/copy/"+stringField(t, childB, "id"), map[string]any{
		"targetParentId": " ",
		"title":          "Whitespace Copy HTTP",
		"slug":           "whitespace-copy-http",
	}, http.StatusBadRequest)
	if !strings.Contains(whitespaceCopyHTTP, "page_invalid_parent_id") {
		t.Fatalf("HTTP copy_page whitespace targetParentId error = %q, want page_invalid_parent_id", whitespaceCopyHTTP)
	}
	paddedCopyErr := callToolError(t, session, "copy_page", map[string]any{
		"id":             stringField(t, childA, "id"),
		"targetParentId": " " + parentID + " ",
		"title":          "Padded Copy",
		"slug":           "padded-copy",
	})
	assertErrorContainsAny(t, "MCP copy_page padded targetParentId", paddedCopyErr, "page_invalid_parent_id", "parent")
	paddedCopyHTTP := postHTTPJSONBody(t, router, "/api/pages/copy/"+stringField(t, childB, "id"), map[string]any{
		"targetParentId": " " + parentID + " ",
		"title":          "Padded Copy HTTP",
		"slug":           "padded-copy-http",
	}, http.StatusBadRequest)
	if !strings.Contains(paddedCopyHTTP, "page_invalid_parent_id") {
		t.Fatalf("HTTP copy_page padded targetParentId error = %q, want page_invalid_parent_id", paddedCopyHTTP)
	}
	assertMapFieldsEqual(t, "copy_page", copied, httpCopied, []string{"title", "kind", "content"})
	assertPageState(t, "MCP copied page", getHTTPPageByPath(t, router, "child-a-copy"), stringField(t, copied, "id"), "Child A Copy", "child-a-copy", "child-a-copy", "page", "")
	assertPageState(t, "HTTP copied page", getHTTPPageByPath(t, router, "child-a-http-copy"), stringField(t, httpCopied, "id"), "Child A Copy", "child-a-http-copy", "child-a-http-copy", "page", "")
	assertPageState(t, "copy_page source preserved", getHTTPPageByPath(t, router, "child-a"), stringField(t, childA, "id"), "Child A", "child-a", "child-a", "page", "")
	recordHTTPMCPParity(t, "copy_page", "POST /api/pages/copy/:id")

	missingDeleteVersionErr := callToolError(t, session, "delete_page", map[string]any{"id": stringField(t, copied, "id")})
	if !strings.Contains(strings.ToLower(missingDeleteVersionErr), "version") {
		t.Fatalf("MCP delete without version error = %q, want version detail", missingDeleteVersionErr)
	}
	missingDeleteHTTP := deleteHTTPStatus(t, router, "/api/pages/"+stringField(t, copied, "id"), http.StatusBadRequest)
	if !strings.Contains(strings.ToLower(missingDeleteHTTP), "version") {
		t.Fatalf("HTTP delete without version error = %q, want version detail", missingDeleteHTTP)
	}

	staleDelete := nestedMap(t, callToolStructured(t, session, "update_page", map[string]any{
		"id":      stringField(t, copied, "id"),
		"version": stringField(t, copied, "version"),
		"title":   "Child A Copy Updated",
		"slug":    "child-a-copy",
		"content": "updated",
	}), "page")
	staleDeleteErr := callToolError(t, session, "delete_page", map[string]any{
		"id":      stringField(t, copied, "id"),
		"version": stringField(t, copied, "version"),
	})
	staleDeleteHTTP := deleteHTTPStatus(t, router, "/api/pages/"+stringField(t, copied, "id")+"?version="+url.QueryEscape(stringField(t, copied, "version")), http.StatusConflict)
	assertPageVersionConflictParity(t, "stale delete_page", staleDeleteErr, staleDeleteHTTP)

	mcpDeletedPage := callToolStructured(t, session, "delete_page", map[string]any{
		"id":      stringField(t, copied, "id"),
		"version": stringField(t, staleDelete, "version"),
	})
	httpDeletedPageBody := deleteHTTPStatus(t, router, "/api/pages/"+stringField(t, httpCopied, "id")+"?version="+url.QueryEscape(stringField(t, httpCopied, "version")), http.StatusOK)
	assertJSONEqual(t, "delete_page", mcpDeletedPage, decodeJSONMap(t, "HTTP delete_page", []byte(httpDeletedPageBody)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pages/by-path?path=child-a-copy", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET deleted copy = %d, want 404", rec.Code)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pages/by-path?path=child-a-http-copy", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET HTTP-deleted copy = %d, want 404", rec.Code)
	}
	recordHTTPMCPParity(t, "delete_page", "DELETE /api/pages/:id")
}

func TestLocalMCPProtocol_IndexAndAssetParity(t *testing.T) {
	runLocalMCPProtocolIndexAndAssetParity(t)
}

func runLocalMCPProtocolIndexAndAssetParity(t *testing.T) {
	w := newLocalMCPTestWiki(t, false)
	router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	})
	session := connectLocalMCP(t, router, "/mcp")

	searchErr := callToolError(t, session, "search_pages", map[string]any{})
	if !strings.Contains(searchErr, "search_missing_query") && !strings.Contains(strings.ToLower(searchErr), "query") {
		t.Fatalf("empty search_pages error = %q, want missing query detail", searchErr)
	}
	blankTagSearchErr := callToolError(t, session, "search_pages", map[string]any{"tags": []any{" "}})
	if !strings.Contains(blankTagSearchErr, "search_missing_query") && !strings.Contains(strings.ToLower(blankTagSearchErr), "query") {
		t.Fatalf("blank-tag search_pages error = %q, want missing query detail", blankTagSearchErr)
	}

	target := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Target",
		"slug":  "target",
		"kind":  "page",
	}), "page")
	source := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Source",
		"slug":  "source",
		"kind":  "page",
	}), "page")
	sourceID := stringField(t, source, "id")
	content := "Tagged source with [Target](/target)"
	callToolStructured(t, session, "update_page", map[string]any{
		"id":      sourceID,
		"version": stringField(t, source, "version"),
		"title":   "Source",
		"slug":    "source",
		"content": content,
		"tags":    []any{"mcp", "assets"},
		"properties": map[string]any{
			"status": "draft",
		},
	})

	status := callToolStructured(t, session, "get_search_status", nil)
	httpStatus := getHTTPValue(t, router, "/api/search/status")
	assertJSONEqual(t, "get_search_status", status["status"], httpStatus)
	recordHTTPMCPParity(t, "get_search_status", "GET /api/search/status")
	if status["status"] == nil {
		t.Fatalf("get_search_status returned no status: %#v", status)
	}

	tags := callToolStructured(t, session, "list_tags", map[string]any{"q": "mc", "limit": float64(10)})
	httpTags := getHTTPValue(t, router, "/api/tags?q=mc&limit=10")
	assertJSONEqual(t, "list_tags", tags["tags"], httpTags)
	recordHTTPMCPParity(t, "list_tags", "GET /api/tags")
	if !arrayContainsObjectField(tags["tags"], "tag", "mcp") {
		t.Fatalf("list_tags = %#v, want mcp tag", tags["tags"])
	}
	tagPages := callToolStructured(t, session, "get_pages_by_tags", map[string]any{"tags": []any{"mcp"}})
	httpTagPages := getHTTPValue(t, router, "/api/tags/pages?tags=mcp")
	assertJSONEqual(t, "get_pages_by_tags", tagPages["pages"], httpTagPages)
	recordHTTPMCPParity(t, "get_pages_by_tags", "GET /api/tags/pages")
	if !arrayContainsObjectField(tagPages["pages"], "id", sourceID) {
		t.Fatalf("get_pages_by_tags = %#v, want source page", tagPages["pages"])
	}
	blankTagsErr := callToolError(t, session, "get_pages_by_tags", map[string]any{"tags": []any{" "}})
	if !strings.Contains(blankTagsErr, wikitags.ErrCodeTagsMissingParam) && !strings.Contains(strings.ToLower(blankTagsErr), "tags") {
		t.Fatalf("MCP blank get_pages_by_tags error = %q, want tags missing detail", blankTagsErr)
	}
	blankTagsHTTP := getHTTPStatus(t, router, "/api/tags/pages?tags=+", http.StatusBadRequest)
	if !strings.Contains(blankTagsHTTP, wikitags.ErrCodeTagsMissingParam) {
		t.Fatalf("HTTP blank get_pages_by_tags error = %q, want %s", blankTagsHTTP, wikitags.ErrCodeTagsMissingParam)
	}

	keys := callToolStructured(t, session, "list_property_keys", map[string]any{"q": "sta", "limit": float64(10)})
	httpKeys := getHTTPValue(t, router, "/api/properties?q=sta&limit=10")
	assertJSONEqual(t, "list_property_keys", keys["keys"], httpKeys)
	recordHTTPMCPParity(t, "list_property_keys", "GET /api/properties")
	if !arrayContainsObjectField(keys["keys"], "key", "status") {
		t.Fatalf("list_property_keys = %#v, want status key", keys["keys"])
	}
	propertyPages := callToolStructured(t, session, "get_pages_by_property", map[string]any{"key": "status", "value": "draft"})
	httpPropertyPages := getHTTPValue(t, router, "/api/properties/pages?key=status&value=draft")
	assertJSONEqual(t, "get_pages_by_property", propertyPages["pages"], httpPropertyPages)
	recordHTTPMCPParity(t, "get_pages_by_property", "GET /api/properties/pages")
	if !arrayContainsObjectField(propertyPages["pages"], "id", sourceID) {
		t.Fatalf("get_pages_by_property = %#v, want source page", propertyPages["pages"])
	}

	links := callToolStructured(t, session, "get_link_status", map[string]any{"pageId": sourceID})
	linkStatus := nestedMap(t, links, "status")
	httpLinkStatus := getHTTPMap(t, router, "/api/pages/"+sourceID+"/links")
	assertJSONEqual(t, "get_link_status", linkStatus, httpLinkStatus)
	recordHTTPMCPParity(t, "get_link_status", "GET /api/pages/:id/links")
	counts := nestedMap(t, linkStatus, "counts")
	if counts["outgoings"] != float64(1) {
		t.Fatalf("link outgoing count = %v, want 1; target=%s", counts["outgoings"], target["id"])
	}

	assetContent := []byte("asset content")
	cssContent := []byte("body { color: rebeccapurple; }\n")
	httpAssetContent := []byte("asset content from http")
	uploaded := callToolStructured(t, session, "upload_asset", map[string]any{
		"pageId":        sourceID,
		"filename":      "note.txt",
		"contentBase64": base64.StdEncoding.EncodeToString(assetContent),
	})
	if uploaded["file"] != "/assets/"+sourceID+"/note.txt" {
		t.Fatalf("upload_asset file = %v, want note asset URL", uploaded["file"])
	}
	httpUploaded := uploadHTTPAsset(t, router, sourceID, "http-note.txt", httpAssetContent, http.StatusCreated)
	assertAssetURLResult(t, "upload_asset", uploaded, "file", sourceID)
	assertAssetURLResult(t, "HTTP upload asset", httpUploaded, "file", sourceID)
	asset := callToolStructured(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "note.txt"})
	if asset["filename"] != "note.txt" {
		t.Fatalf("get_asset filename = %v, want note.txt", asset["filename"])
	}
	if got := asset["contentBase64"]; got != base64.StdEncoding.EncodeToString(assetContent) {
		t.Fatalf("get_asset contentBase64 = %v, want uploaded content", got)
	}
	httpNoteBody, httpNoteContentType := getHTTPAssetWithContentType(t, router, sourceID, "note.txt")
	if httpNoteBody != string(assetContent) {
		t.Fatalf("HTTP asset content = %q, want uploaded content", httpNoteBody)
	}
	if !strings.HasPrefix(httpNoteContentType, asset["mimeType"].(string)) {
		t.Fatalf("HTTP note content type = %q, want MCP mime type %q", httpNoteContentType, asset["mimeType"])
	}
	httpAsset := callToolStructured(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "http-note.txt"})
	if got := httpAsset["contentBase64"]; got != base64.StdEncoding.EncodeToString(httpAssetContent) {
		t.Fatalf("MCP read of HTTP-uploaded asset = %v, want HTTP uploaded content", got)
	}
	httpAssetBody, httpAssetContentType := getHTTPAssetWithContentType(t, router, sourceID, "http-note.txt")
	if httpAssetBody != string(httpAssetContent) {
		t.Fatalf("HTTP-uploaded asset content = %q, want HTTP uploaded content", httpAssetBody)
	}
	if !strings.HasPrefix(httpAssetContentType, httpAsset["mimeType"].(string)) {
		t.Fatalf("HTTP-uploaded content type = %q, want MCP mime type %q", httpAssetContentType, httpAsset["mimeType"])
	}
	listed := callToolStructured(t, session, "list_assets", map[string]any{"pageId": sourceID})
	if !arrayContainsString(listed["files"], "/assets/"+sourceID+"/note.txt") {
		t.Fatalf("list_assets = %#v, want uploaded note", listed["files"])
	}
	httpListed := getHTTPAssets(t, router, sourceID)
	if !arrayContainsString(httpListed["files"], "/assets/"+sourceID+"/note.txt") {
		t.Fatalf("HTTP asset list = %#v, want uploaded note", httpListed["files"])
	}
	assertJSONEqual(t, "list_assets", listed, httpListed)
	recordHTTPMCPParity(t, "upload_asset", "POST /api/pages/:id/assets")
	recordHTTPMCPParity(t, "list_assets", "GET /api/pages/:id/assets")
	recordHTTPMCPParity(t, "get_asset", "GET /assets/:pageId/:filename")
	callToolStructured(t, session, "upload_asset", map[string]any{
		"pageId":        sourceID,
		"filename":      "style.css",
		"contentBase64": base64.StdEncoding.EncodeToString(cssContent),
	})
	cssAsset := callToolStructured(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "style.css"})
	httpCSSBody, httpCSSContentType := getHTTPAssetWithContentType(t, router, sourceID, "style.css")
	if httpCSSBody != string(cssContent) {
		t.Fatalf("HTTP CSS asset content = %q, want uploaded CSS", httpCSSBody)
	}
	if !strings.HasPrefix(httpCSSContentType, cssAsset["mimeType"].(string)) {
		t.Fatalf("HTTP CSS asset content type = %q, want MCP mime type %q", httpCSSContentType, cssAsset["mimeType"])
	}

	renamed := callToolStructured(t, session, "rename_asset", map[string]any{
		"pageId":      sourceID,
		"oldFilename": "note.txt",
		"newFilename": "renamed.txt",
	})
	if renamed["url"] != "/assets/"+sourceID+"/renamed.txt" {
		t.Fatalf("rename_asset url = %v, want renamed URL", renamed["url"])
	}
	httpRenamed := putHTTPJSON(t, router, "/api/pages/"+sourceID+"/assets/rename", map[string]any{
		"old_filename": "http-note.txt",
		"new_filename": "http-renamed.txt",
	}, http.StatusOK)
	assertAssetURLResult(t, "rename_asset", renamed, "url", sourceID)
	assertAssetURLResult(t, "HTTP rename_asset", httpRenamed, "url", sourceID)
	renamedAsset := callToolStructured(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "renamed.txt"})
	if got := renamedAsset["contentBase64"]; got != base64.StdEncoding.EncodeToString(assetContent) {
		t.Fatalf("MCP renamed asset content = %v, want original MCP asset content", got)
	}
	httpRenamedBody, httpRenamedContentType := getHTTPAssetWithContentType(t, router, sourceID, "renamed.txt")
	if httpRenamedBody != string(assetContent) {
		t.Fatalf("HTTP renamed asset content = %q, want original MCP asset content", httpRenamedBody)
	}
	if !strings.HasPrefix(httpRenamedContentType, renamedAsset["mimeType"].(string)) {
		t.Fatalf("HTTP renamed content type = %q, want MCP mime type %q", httpRenamedContentType, renamedAsset["mimeType"])
	}
	assertMCPToolErrorContains(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "note.txt"}, "asset")
	getHTTPStatus(t, router, "/assets/"+sourceID+"/note.txt", http.StatusNotFound)
	httpRenamedAsset := callToolStructured(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "http-renamed.txt"})
	if got := httpRenamedAsset["contentBase64"]; got != base64.StdEncoding.EncodeToString(httpAssetContent) {
		t.Fatalf("MCP read of HTTP-renamed asset content = %v, want HTTP asset content", got)
	}
	if got := getHTTPAsset(t, router, sourceID, "http-renamed.txt"); got != string(httpAssetContent) {
		t.Fatalf("HTTP-renamed asset content = %q, want HTTP asset content", got)
	}
	assertMCPToolErrorContains(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "http-note.txt"}, "asset")
	getHTTPStatus(t, router, "/assets/"+sourceID+"/http-note.txt", http.StatusNotFound)
	httpListed = getHTTPAssets(t, router, sourceID)
	listed = callToolStructured(t, session, "list_assets", map[string]any{"pageId": sourceID})
	assertJSONEqual(t, "list_assets after rename", listed, httpListed)
	if !arrayContainsString(httpListed["files"], "/assets/"+sourceID+"/renamed.txt") || !arrayContainsString(httpListed["files"], "/assets/"+sourceID+"/http-renamed.txt") {
		t.Fatalf("HTTP asset list after rename = %#v, want renamed assets", httpListed["files"])
	}
	recordHTTPMCPParity(t, "rename_asset", "PUT /api/pages/:id/assets/rename")
	mcpDeleted := callToolStructured(t, session, "delete_asset", map[string]any{"pageId": sourceID, "filename": "renamed.txt"})
	httpDeletedBody := deleteHTTPStatus(t, router, "/api/pages/"+sourceID+"/assets/http-renamed.txt", http.StatusOK)
	httpDeleted := decodeJSONMap(t, "HTTP delete_asset", []byte(httpDeletedBody))
	assertJSONEqual(t, "delete_asset", mcpDeleted, httpDeleted)
	assertMCPToolErrorContains(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "renamed.txt"}, "asset")
	getHTTPStatus(t, router, "/assets/"+sourceID+"/renamed.txt", http.StatusNotFound)
	assertMCPToolErrorContains(t, session, "get_asset", map[string]any{"pageId": sourceID, "filename": "http-renamed.txt"}, "asset")
	getHTTPStatus(t, router, "/assets/"+sourceID+"/http-renamed.txt", http.StatusNotFound)
	listed = callToolStructured(t, session, "list_assets", map[string]any{"pageId": sourceID})
	if arrayContainsString(listed["files"], "/assets/"+sourceID+"/renamed.txt") {
		t.Fatalf("delete_asset left renamed asset in list: %#v", listed["files"])
	}
	httpListed = getHTTPAssets(t, router, sourceID)
	assertJSONEqual(t, "list_assets after delete", listed, httpListed)
	if arrayContainsString(httpListed["files"], "/assets/"+sourceID+"/renamed.txt") || arrayContainsString(httpListed["files"], "/assets/"+sourceID+"/http-renamed.txt") {
		t.Fatalf("HTTP asset list after delete = %#v, want renamed assets absent", httpListed["files"])
	}
	recordHTTPMCPParity(t, "delete_asset", "DELETE /api/pages/:id/assets/:name")
}

func TestLocalMCPProtocol_UploadAssetRejectsOversizedInputBeforePageLookup(t *testing.T) {
	w := newLocalMCPTestWiki(t, false)
	router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: 2,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	})
	session := connectLocalMCP(t, router, "/mcp")

	errText := callToolError(t, session, "upload_asset", map[string]any{
		"pageId":        "missing-page",
		"filename":      "too-large.txt",
		"contentBase64": base64.StdEncoding.EncodeToString([]byte("abc")),
	})
	if !strings.Contains(errText, "asset_file_too_large") && !strings.Contains(strings.ToLower(errText), "too large") {
		t.Fatalf("oversized upload error = %q, want asset_file_too_large before page lookup", errText)
	}
}

func TestLocalMCPProtocol_UploadAssetRejectsMalformedBase64AsAssetPayload(t *testing.T) {
	w := newLocalMCPTestWiki(t, false)
	router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	})
	session := connectLocalMCP(t, router, "/mcp")

	page := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Asset Payload",
		"slug":  "asset-payload",
		"kind":  "page",
	}), "page")

	errText := callToolError(t, session, "upload_asset", map[string]any{
		"pageId":        stringField(t, page, "id"),
		"filename":      "bad.txt",
		"contentBase64": "not base64 %",
	})
	if !strings.Contains(errText, wikiassets.ErrCodeAssetInvalidPayload) && !strings.Contains(strings.ToLower(errText), "invalid asset payload") {
		t.Fatalf("malformed base64 upload error = %q, want %s", errText, wikiassets.ErrCodeAssetInvalidPayload)
	}
}

func TestLocalMCPProtocol_GetAssetUsesPageBoundaryValidation(t *testing.T) {
	w, storageDir := newLocalMCPTestWikiWithStorage(t, false)
	router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	})
	session := connectLocalMCP(t, router, "/mcp")

	orphanAssetDir := filepath.Join(storageDir, "assets", "missing-page")
	if err := os.MkdirAll(orphanAssetDir, 0755); err != nil {
		t.Fatalf("create orphan asset dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanAssetDir, "orphan.txt"), []byte("orphan"), 0644); err != nil {
		t.Fatalf("write orphan asset: %v", err)
	}

	errText := callToolError(t, session, "get_asset", map[string]any{"pageId": "missing-page", "filename": "orphan.txt"})
	if !strings.Contains(errText, "asset_page_not_found") && !strings.Contains(strings.ToLower(errText), "page not found") {
		t.Fatalf("orphan get_asset error = %q, want page boundary validation", errText)
	}
}

func TestLocalMCPProtocol_FeatureGatedToolParity(t *testing.T) {
	runLocalMCPProtocolFeatureGatedToolParity(t)
}

func runLocalMCPProtocolFeatureGatedToolParity(t *testing.T) {
	w, storageDir := newLocalMCPTestWikiWithStorage(t, true)
	router := newLocalMCPTestRouter(w, httpinternal.RouterOptions{
		AuthDisabled:            true,
		PublicAccess:            true,
		AllowInsecure:           true,
		MaxAssetUploadSizeBytes: assets.DefaultMaxUploadSizeBytes,
		EnableRevision:          true,
		EnableLinkRefactor:      true,
		MCPEnabled:              true,
		MCPToolListPageSize:     200,
	})
	session := connectLocalMCP(t, router, "/mcp")

	missingLatestErr := callToolError(t, session, "get_latest_revision", map[string]any{"pageId": "missing-page"})
	if !strings.Contains(missingLatestErr, "revision_not_found") && !strings.Contains(strings.ToLower(missingLatestErr), "revision") {
		t.Fatalf("missing latest revision error = %q, want revision_not_found detail", missingLatestErr)
	}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{name: "get_revision", args: map[string]any{"pageId": "missing-page", "revisionId": "missing-revision"}},
		{name: "compare_revisions", args: map[string]any{"pageId": "missing-page", "baseRevisionId": "base-revision", "targetRevisionId": "target-revision"}},
		{name: "get_revision_asset", args: map[string]any{"pageId": "missing-page", "revisionId": "missing-revision", "assetName": "missing.txt"}},
	} {
		errText := callToolError(t, session, tc.name, tc.args)
		lowerErr := strings.ToLower(errText)
		if !strings.Contains(errText, wikirevisions.ErrCodeRevisionNotFound) && !strings.Contains(lowerErr, "revision not found") && !strings.Contains(lowerErr, "revision asset not found") {
			t.Fatalf("%s missing revision error = %q, want %s", tc.name, errText, wikirevisions.ErrCodeRevisionNotFound)
		}
		if strings.Contains(lowerErr, "file does not exist") || strings.Contains(lowerErr, "no such file") {
			t.Fatalf("%s missing revision error = %q, want structured revision error instead of raw storage error", tc.name, errText)
		}
	}
	for _, tc := range []struct {
		name     string
		args     map[string]any
		wantCode string
		wantText string
	}{
		{name: "get_revision", args: map[string]any{"pageId": "missing-page", "revisionId": " "}, wantCode: wikirevisions.ErrCodeRevisionInvalidRevisionID, wantText: "revision id is required"},
		{name: "get_revision_asset", args: map[string]any{"pageId": "missing-page", "revisionId": " ", "assetName": "missing.txt"}, wantCode: wikirevisions.ErrCodeRevisionInvalidRevisionID, wantText: "revision id is required"},
		{name: "compare_revisions", args: map[string]any{"pageId": "missing-page", "baseRevisionId": " ", "targetRevisionId": "target-revision"}, wantCode: wikirevisions.ErrCodeRevisionCompareInvalidRequest, wantText: "revision compare request is invalid"},
		{name: "compare_revisions", args: map[string]any{"pageId": "missing-page", "baseRevisionId": "base-revision", "targetRevisionId": " "}, wantCode: wikirevisions.ErrCodeRevisionCompareInvalidRequest, wantText: "revision compare request is invalid"},
	} {
		errText := callToolError(t, session, tc.name, tc.args)
		assertErrorContainsAny(t, tc.name+" blank revision input", errText, tc.wantCode, tc.wantText)
	}

	target := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Target",
		"slug":  "target",
		"kind":  "page",
	}), "page")
	targetID := stringField(t, target, "id")
	ref := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Ref",
		"slug":  "ref",
		"kind":  "page",
	}), "page")
	refID := stringField(t, ref, "id")
	callToolStructured(t, session, "update_page", map[string]any{
		"id":      refID,
		"version": stringField(t, ref, "version"),
		"title":   "Ref",
		"slug":    "ref",
		"content": "[Target](/target)",
	})

	first := nestedMap(t, callToolStructured(t, session, "update_page", map[string]any{
		"id":      targetID,
		"version": stringField(t, target, "version"),
		"title":   "Target",
		"slug":    "target",
		"content": "First content",
	}), "page")
	second := nestedMap(t, callToolStructured(t, session, "update_page", map[string]any{
		"id":      targetID,
		"version": stringField(t, first, "version"),
		"title":   "Target",
		"slug":    "target",
		"content": "Second content",
	}), "page")
	httpUpdated := updateHTTPPage(t, router, targetID, map[string]any{
		"version": stringField(t, second, "version"),
		"title":   "Target",
		"slug":    "target",
		"content": "Third content from HTTP",
	})

	limitErr := callToolError(t, session, "list_revisions", map[string]any{"pageId": targetID, "limit": float64(201)})
	if !strings.Contains(limitErr, "revision_invalid_limit") && !strings.Contains(strings.ToLower(limitErr), "limit") {
		t.Fatalf("invalid list_revisions limit error = %q, want invalid limit detail", limitErr)
	}

	revisions := callToolStructured(t, session, "list_revisions", map[string]any{"pageId": targetID, "limit": float64(20)})
	httpRevisions := getHTTPMap(t, router, "/api/pages/"+targetID+"/revisions?limit=20")
	assertJSONEqual(t, "list_revisions", revisions, httpRevisions)
	recordHTTPMCPParity(t, "list_revisions", "GET /api/pages/:id/revisions")
	revisionItems, ok := revisions["revisions"].([]any)
	if !ok || len(revisionItems) < 2 {
		t.Fatalf("list_revisions = %#v, want at least two revisions", revisions["revisions"])
	}
	firstRevision := revisionItems[0].(map[string]any)
	if _, exists := firstRevision["page_id"]; exists {
		t.Fatalf("list_revisions returned raw snake_case revision: %#v", firstRevision)
	}
	if firstRevision["pageId"] != targetID {
		t.Fatalf("list_revisions pageId = %v, want %q", firstRevision["pageId"], targetID)
	}
	latest := callToolStructured(t, session, "get_latest_revision", map[string]any{"pageId": targetID})
	latestRevision := nestedMap(t, latest, "revision")
	httpLatestRevision := getHTTPLatestRevision(t, router, targetID)
	assertJSONEqual(t, "get_latest_revision", latestRevision, httpLatestRevision)
	recordHTTPMCPParity(t, "get_latest_revision", "GET /api/pages/:id/revisions/latest")
	latestRevisionID := stringField(t, latestRevision, "id")
	if _, exists := latestRevision["page_id"]; exists {
		t.Fatalf("get_latest_revision returned raw snake_case revision: %#v", latestRevision)
	}
	if latestRevision["pageId"] != targetID {
		t.Fatalf("get_latest_revision pageId = %v, want %q", latestRevision["pageId"], targetID)
	}
	snapshot := callToolStructured(t, session, "get_revision", map[string]any{"pageId": targetID, "revisionId": latestRevisionID})
	httpSnapshotAtLatest := getHTTPRevision(t, router, targetID, latestRevisionID)
	assertJSONEqual(t, "get_revision", snapshot, httpSnapshotAtLatest)
	recordHTTPMCPParity(t, "get_revision", "GET /api/pages/:id/revisions/:revisionId")
	if snapshot["content"] != "Third content from HTTP" {
		t.Fatalf("get_revision content = %v, want HTTP-updated content", snapshot["content"])
	}
	snapshotRevision := nestedMap(t, snapshot, "revision")
	if snapshotRevision["pageId"] != targetID {
		t.Fatalf("get_revision revision.pageId = %v, want %q", snapshotRevision["pageId"], targetID)
	}
	mcpAfterHTTP := nestedMap(t, callToolStructured(t, session, "update_page", map[string]any{
		"id":      targetID,
		"version": stringField(t, httpUpdated, "version"),
		"title":   "Target",
		"slug":    "target",
		"content": "Fourth content from MCP",
	}), "page")
	if mcpAfterHTTP["content"] != "Fourth content from MCP" {
		t.Fatalf("MCP update after HTTP content = %v, want MCP-updated content", mcpAfterHTTP["content"])
	}
	httpLatest := getHTTPLatestRevision(t, router, targetID)
	httpLatestRevisionID := stringField(t, httpLatest, "id")
	httpSnapshot := getHTTPRevision(t, router, targetID, httpLatestRevisionID)
	if httpSnapshot["content"] != "Fourth content from MCP" {
		t.Fatalf("HTTP revision content after MCP update = %v, want MCP-updated content", httpSnapshot["content"])
	}

	olderRevision := revisionItems[1].(map[string]any)
	comparison := callToolStructured(t, session, "compare_revisions", map[string]any{
		"pageId":           targetID,
		"baseRevisionId":   stringField(t, olderRevision, "id"),
		"targetRevisionId": httpLatestRevisionID,
	})
	httpComparison := getHTTPMap(t, router, "/api/pages/"+targetID+"/revisions/compare?base="+url.QueryEscape(stringField(t, olderRevision, "id"))+"&target="+url.QueryEscape(httpLatestRevisionID))
	assertJSONEqual(t, "compare_revisions", comparison, httpComparison)
	recordHTTPMCPParity(t, "compare_revisions", "GET /api/pages/:id/revisions/compare")
	if comparison["contentChanged"] != true {
		t.Fatalf("compare_revisions contentChanged = %v, want true", comparison["contentChanged"])
	}

	assetContent := []byte("body { color: green; }\n")
	callToolStructured(t, session, "upload_asset", map[string]any{
		"pageId":        targetID,
		"filename":      "style.css",
		"contentBase64": base64.StdEncoding.EncodeToString(assetContent),
	})
	assetRevision := nestedMap(t, callToolStructured(t, session, "get_latest_revision", map[string]any{"pageId": targetID}), "revision")
	assetRevisionID := stringField(t, assetRevision, "id")
	stripRevisionAssetManifestMIME(t, storageDir, stringField(t, assetRevision, "assetManifestHash"), "style.css")
	revisionAsset := callToolStructured(t, session, "get_revision_asset", map[string]any{
		"pageId":     targetID,
		"revisionId": assetRevisionID,
		"assetName":  "style.css",
	})
	if revisionAsset["contentBase64"] != base64.StdEncoding.EncodeToString(assetContent) {
		t.Fatalf("get_revision_asset content = %v, want uploaded asset", revisionAsset["contentBase64"])
	}
	httpRevisionAssetBody, httpRevisionAssetContentType := getHTTPRevisionAsset(t, router, targetID, assetRevisionID, "style.css")
	if httpRevisionAssetBody != string(assetContent) {
		t.Fatalf("HTTP revision asset body = %q, want uploaded asset", httpRevisionAssetBody)
	}
	if revisionAsset["mimeType"] != "text/css; charset=utf-8" {
		t.Fatalf("get_revision_asset missing-manifest MIME = %v, want CSS extension fallback", revisionAsset["mimeType"])
	}
	if !strings.HasPrefix(httpRevisionAssetContentType, revisionAsset["mimeType"].(string)) {
		t.Fatalf("HTTP revision asset content type = %q, want MCP mime type %q", httpRevisionAssetContentType, revisionAsset["mimeType"])
	}
	assetBlobPath := revisionAssetBlobPath(t, storageDir, stringField(t, assetRevision, "assetManifestHash"), "style.css")
	if err := os.Chmod(assetBlobPath, 0); err != nil {
		t.Fatalf("make revision asset blob unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(assetBlobPath, 0o644) })
	unreadableAssetErr := callToolError(t, session, "get_revision_asset", map[string]any{
		"pageId":     targetID,
		"revisionId": assetRevisionID,
		"assetName":  "style.css",
	})
	assertErrorContainsAny(t, "MCP unreadable revision asset blob", unreadableAssetErr, wikirevisions.ErrCodeRevisionPreviewAssetBlobUnavailable, "asset blob")
	unreadableAssetHTTP := getHTTPStatus(t, router, "/api/pages/"+targetID+"/revisions/"+assetRevisionID+"/assets/style.css", http.StatusInternalServerError)
	if !strings.Contains(unreadableAssetHTTP, wikirevisions.ErrCodeRevisionPreviewAssetBlobUnavailable) {
		t.Fatalf("HTTP unreadable revision asset blob error = %q, want %s", unreadableAssetHTTP, wikirevisions.ErrCodeRevisionPreviewAssetBlobUnavailable)
	}
	if err := os.Chmod(assetBlobPath, 0o644); err != nil {
		t.Fatalf("restore revision asset blob permissions: %v", err)
	}
	recordHTTPMCPParity(t, "get_revision_asset", "GET /api/pages/:id/revisions/:revisionId/assets/:name")

	invalidRefactorKindErr := callToolError(t, session, "preview_page_refactor", map[string]any{
		"id":    targetID,
		"kind":  "copy",
		"title": "Target",
		"slug":  "target-copy",
	})
	assertErrorContainsAny(t, "MCP invalid preview_page_refactor kind", invalidRefactorKindErr, wikipages.ErrCodePageInvalidRefactorKind, "invalid refactor kind")
	assertErrorDoesNotContainAny(t, "MCP invalid preview_page_refactor kind", invalidRefactorKindErr, "enum", "validating")
	invalidRefactorKindHTTP := postHTTPJSONBody(t, router, "/api/pages/"+targetID+"/refactor/preview", map[string]any{
		"kind":  "copy",
		"title": "Target",
		"slug":  "target-copy",
	}, http.StatusBadRequest)
	if !strings.Contains(invalidRefactorKindHTTP, "page_invalid_refactor_kind") {
		t.Fatalf("HTTP invalid preview_page_refactor kind error = %q, want page_invalid_refactor_kind", invalidRefactorKindHTTP)
	}
	paddedRefactorKindErr := callToolError(t, session, "preview_page_refactor", map[string]any{
		"id":    targetID,
		"kind":  " rename ",
		"title": "Target",
		"slug":  "target-padded",
	})
	assertErrorContainsAny(t, "MCP padded preview_page_refactor kind", paddedRefactorKindErr, wikipages.ErrCodePageInvalidRefactorKind, "invalid refactor kind")
	assertErrorDoesNotContainAny(t, "MCP padded preview_page_refactor kind", paddedRefactorKindErr, "enum", "validating")
	paddedRefactorKindHTTP := postHTTPJSONBody(t, router, "/api/pages/"+targetID+"/refactor/preview", map[string]any{
		"kind":  " rename ",
		"title": "Target",
		"slug":  "target-padded",
	}, http.StatusBadRequest)
	if !strings.Contains(paddedRefactorKindHTTP, "page_invalid_refactor_kind") {
		t.Fatalf("HTTP padded preview_page_refactor kind error = %q, want page_invalid_refactor_kind", paddedRefactorKindHTTP)
	}
	currentForInvalidApply := nestedMap(t, callToolStructured(t, session, "get_page", map[string]any{"id": targetID}), "page")
	invalidApplyKindErr := callToolError(t, session, "apply_page_refactor", map[string]any{
		"id":      targetID,
		"version": stringField(t, currentForInvalidApply, "version"),
		"kind":    "copy",
		"title":   "Target",
		"slug":    "target-copy",
	})
	assertErrorContainsAny(t, "MCP invalid apply_page_refactor kind", invalidApplyKindErr, wikipages.ErrCodePageInvalidRefactorKind, "invalid refactor kind")
	assertErrorDoesNotContainAny(t, "MCP invalid apply_page_refactor kind", invalidApplyKindErr, "enum", "validating")
	invalidApplyKindHTTP := postHTTPJSONBody(t, router, "/api/pages/"+targetID+"/refactor/apply", map[string]any{
		"version": stringField(t, currentForInvalidApply, "version"),
		"kind":    "copy",
		"title":   "Target",
		"slug":    "target-copy",
	}, http.StatusBadRequest)
	if !strings.Contains(invalidApplyKindHTTP, "page_invalid_refactor_kind") {
		t.Fatalf("HTTP invalid apply_page_refactor kind error = %q, want page_invalid_refactor_kind", invalidApplyKindHTTP)
	}
	whitespaceRefactorParentErr := callToolError(t, session, "preview_page_refactor", map[string]any{
		"id":       targetID,
		"kind":     "move",
		"parentId": " ",
	})
	assertErrorContainsAny(t, "MCP preview_page_refactor whitespace parentId", whitespaceRefactorParentErr, "page_invalid_parent_id", "parent")
	whitespaceRefactorParentHTTP := postHTTPJSONBody(t, router, "/api/pages/"+targetID+"/refactor/preview", map[string]any{
		"kind":     "move",
		"parentId": " ",
	}, http.StatusBadRequest)
	if !strings.Contains(whitespaceRefactorParentHTTP, "page_invalid_parent_id") {
		t.Fatalf("HTTP preview_page_refactor whitespace parentId error = %q, want page_invalid_parent_id", whitespaceRefactorParentHTTP)
	}
	refactorPaddedParent := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Refactor Padded Parent",
		"slug":  "refactor-padded-parent",
		"kind":  "section",
	}), "page")
	refactorPaddedTarget := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Refactor Padded Target",
		"slug":  "refactor-padded-target",
		"kind":  "page",
	}), "page")
	paddedApplyParentErr := callToolError(t, session, "apply_page_refactor", map[string]any{
		"id":       stringField(t, refactorPaddedTarget, "id"),
		"version":  stringField(t, refactorPaddedTarget, "version"),
		"kind":     "move",
		"parentId": " " + stringField(t, refactorPaddedParent, "id") + " ",
	})
	assertErrorContainsAny(t, "MCP apply_page_refactor padded parentId", paddedApplyParentErr, "page_invalid_parent_id", "parent")
	paddedApplyParentHTTP := postHTTPJSONBody(t, router, "/api/pages/"+stringField(t, refactorPaddedTarget, "id")+"/refactor/apply", map[string]any{
		"version":  stringField(t, refactorPaddedTarget, "version"),
		"kind":     "move",
		"parentId": " " + stringField(t, refactorPaddedParent, "id") + " ",
	}, http.StatusBadRequest)
	if !strings.Contains(paddedApplyParentHTTP, "page_invalid_parent_id") {
		t.Fatalf("HTTP apply_page_refactor padded parentId error = %q, want page_invalid_parent_id", paddedApplyParentHTTP)
	}

	preview := callToolStructured(t, session, "preview_page_refactor", map[string]any{
		"id":    targetID,
		"kind":  "rename",
		"title": "Target",
		"slug":  "target-renamed",
	})
	httpPreview := postHTTPJSON(t, router, "/api/pages/"+targetID+"/refactor/preview", map[string]any{
		"kind":  "rename",
		"title": "Target",
		"slug":  "target-renamed",
	}, http.StatusOK)
	assertJSONEqual(t, "preview_page_refactor", preview, httpPreview)
	recordHTTPMCPParity(t, "preview_page_refactor", "POST /api/pages/:id/refactor/preview")
	counts := nestedMap(t, preview, "counts")
	if counts["affectedPages"] != float64(1) {
		t.Fatalf("preview affectedPages = %v, want 1", counts["affectedPages"])
	}

	staleRefactor := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "Stale Refactor",
		"slug":  "stale-refactor",
		"kind":  "page",
	}), "page")
	staleRefactorID := stringField(t, staleRefactor, "id")
	updateHTTPPage(t, router, staleRefactorID, map[string]any{
		"version": stringField(t, staleRefactor, "version"),
		"title":   "Stale Refactor",
		"slug":    "stale-refactor",
		"content": "newer version",
	})
	staleRefactorErr := callToolError(t, session, "apply_page_refactor", map[string]any{
		"id":           staleRefactorID,
		"version":      stringField(t, staleRefactor, "version"),
		"kind":         "rename",
		"title":        "Stale Refactor",
		"slug":         "stale-refactor-mcp",
		"rewriteLinks": true,
	})
	staleRefactorHTTP := postHTTPJSONBody(t, router, "/api/pages/"+staleRefactorID+"/refactor/apply", map[string]any{
		"version":      stringField(t, staleRefactor, "version"),
		"kind":         "rename",
		"title":        "Stale Refactor",
		"slug":         "stale-refactor-http",
		"rewriteLinks": true,
	}, http.StatusConflict)
	assertPageVersionConflictParity(t, "stale apply_page_refactor", staleRefactorErr, staleRefactorHTTP)

	httpApplyTarget := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "HTTP Apply Target",
		"slug":  "http-apply-target",
		"kind":  "page",
	}), "page")
	httpApplyRef := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "HTTP Apply Ref",
		"slug":  "http-apply-ref",
		"kind":  "page",
	}), "page")
	callToolStructured(t, session, "update_page", map[string]any{
		"id":      stringField(t, httpApplyRef, "id"),
		"version": stringField(t, httpApplyRef, "version"),
		"title":   "HTTP Apply Ref",
		"slug":    "http-apply-ref",
		"content": "[HTTP Apply Target](/http-apply-target)",
	})
	httpAppliedViaRoute := postHTTPJSON(t, router, "/api/pages/"+stringField(t, httpApplyTarget, "id")+"/refactor/apply", map[string]any{
		"version":      stringField(t, httpApplyTarget, "version"),
		"kind":         "rename",
		"title":        "HTTP Apply Target",
		"slug":         "http-apply-target-renamed",
		"rewriteLinks": true,
	}, http.StatusOK)
	assertPageState(t, "HTTP apply_page_refactor success", httpAppliedViaRoute, stringField(t, httpApplyTarget, "id"), "HTTP Apply Target", "http-apply-target-renamed", "http-apply-target-renamed", "page", "")
	httpApplyRefAfter := getHTTPPageByPath(t, router, "http-apply-ref")
	if httpApplyRefAfter["content"] != "[HTTP Apply Target](/http-apply-target-renamed)" {
		t.Fatalf("HTTP apply ref content = %v, want rewritten link", httpApplyRefAfter["content"])
	}

	currentTarget := nestedMap(t, callToolStructured(t, session, "get_page", map[string]any{"id": targetID}), "page")
	applied := nestedMap(t, callToolStructured(t, session, "apply_page_refactor", map[string]any{
		"id":           targetID,
		"version":      stringField(t, currentTarget, "version"),
		"kind":         "rename",
		"title":        "Target",
		"slug":         "target-renamed",
		"rewriteLinks": true,
	}), "page")
	if applied["slug"] != "target-renamed" {
		t.Fatalf("apply_page_refactor slug = %v, want target-renamed", applied["slug"])
	}
	httpApplied := getHTTPPageByID(t, router, targetID)
	assertJSONEqual(t, "apply_page_refactor", applied, httpApplied)
	assertPageState(t, "MCP apply_page_refactor success", applied, targetID, "Target", "target-renamed", "target-renamed", "page", "")
	refHTTP := getHTTPPageByPath(t, router, "ref")
	if refHTTP["content"] != "[Target](/target-renamed)" {
		t.Fatalf("ref content after refactor = %v, want rewritten link", refHTTP["content"])
	}
	recordHTTPMCPParity(t, "apply_page_refactor", "POST /api/pages/:id/refactor/apply")

	restored := nestedMap(t, callToolStructured(t, session, "restore_revision", map[string]any{
		"pageId":     targetID,
		"revisionId": latestRevisionID,
	}), "page")
	if restored["content"] != "Third content from HTTP" {
		t.Fatalf("restore_revision content = %v, want HTTP-updated content", restored["content"])
	}
	httpRestored := getHTTPPageByID(t, router, targetID)
	assertJSONEqual(t, "restore_revision", restored, httpRestored)

	mcpRestoreMeta := nestedMap(t, callToolStructured(t, session, "create_page", map[string]any{
		"title": "MCP Restore Metadata",
		"slug":  "mcp-restore-metadata",
		"kind":  "page",
	}), "page")
	mcpRestoreMetaID := stringField(t, mcpRestoreMeta, "id")
	mcpRestoreMetaRevision := nestedMap(t, callToolStructured(t, session, "update_page", map[string]any{
		"id":      mcpRestoreMetaID,
		"version": stringField(t, mcpRestoreMeta, "version"),
		"title":   "MCP Restore Metadata",
		"slug":    "mcp-restore-metadata",
		"content": "metadata revision\n",
		"tags":    []any{"restore", "metadata"},
		"properties": map[string]any{
			"status": "archived",
		},
	}), "page")
	mcpRestoreMetaLatest := nestedMap(t, callToolStructured(t, session, "get_latest_revision", map[string]any{"pageId": mcpRestoreMetaID}), "revision")
	callToolStructured(t, session, "update_page", map[string]any{
		"id":      mcpRestoreMetaID,
		"version": stringField(t, mcpRestoreMetaRevision, "version"),
		"title":   "MCP Restore Metadata",
		"slug":    "mcp-restore-metadata",
		"content": "current revision\n",
	})
	mcpRestoredMeta := nestedMap(t, callToolStructured(t, session, "restore_revision", map[string]any{
		"pageId":     mcpRestoreMetaID,
		"revisionId": stringField(t, mcpRestoreMetaLatest, "id"),
	}), "page")
	assertRestoredMetadata(t, "MCP restore metadata", mcpRestoredMeta)
	callToolStructured(t, session, "update_page", map[string]any{
		"id":      mcpRestoreMetaID,
		"version": stringField(t, mcpRestoredMeta, "version"),
		"title":   "MCP Restore Metadata",
		"slug":    "mcp-restore-metadata",
		"content": "current revision before HTTP restore\n",
	})
	httpRestoredMCPMeta := postHTTPJSON(t, router, "/api/pages/"+mcpRestoreMetaID+"/revisions/"+stringField(t, mcpRestoreMetaLatest, "id")+"/restore", nil, http.StatusOK)
	assertRestoredMetadata(t, "HTTP restore metadata on MCP fixture", httpRestoredMCPMeta)
	assertRestorePayloadsMatch(t, mcpRestoredMeta, httpRestoredMCPMeta)

	httpRestoreMeta := postHTTPJSON(t, router, "/api/pages", map[string]any{
		"title": "HTTP Restore Metadata",
		"slug":  "http-restore-metadata",
		"kind":  "page",
	}, http.StatusCreated)
	httpRestoreMetaID := stringField(t, httpRestoreMeta, "id")
	httpRestoreMetaRevision := updateHTTPPage(t, router, httpRestoreMetaID, map[string]any{
		"version": stringField(t, httpRestoreMeta, "version"),
		"title":   "HTTP Restore Metadata",
		"slug":    "http-restore-metadata",
		"content": "metadata revision\n",
		"tags":    []string{"restore", "metadata"},
		"properties": map[string]string{
			"status": "archived",
		},
	})
	httpRestoreMetaLatest := getHTTPLatestRevision(t, router, httpRestoreMetaID)
	updateHTTPPage(t, router, httpRestoreMetaID, map[string]any{
		"version": stringField(t, httpRestoreMetaRevision, "version"),
		"title":   "HTTP Restore Metadata",
		"slug":    "http-restore-metadata",
		"content": "current revision\n",
	})
	httpRestoredMeta := postHTTPJSON(t, router, "/api/pages/"+httpRestoreMetaID+"/revisions/"+stringField(t, httpRestoreMetaLatest, "id")+"/restore", nil, http.StatusOK)
	assertRestoredMetadata(t, "HTTP restore metadata", httpRestoredMeta)
	assertRestoredMetadata(t, "HTTP restore metadata persisted", getHTTPPageByID(t, router, httpRestoreMetaID))
	recordHTTPMCPParity(t, "restore_revision", "POST /api/pages/:id/revisions/:revisionId/restore")
}

func TestLocalMCPProtocol_HTTPParityCoverageRecordedForPlanTools(t *testing.T) {
	runHTTPMCPParityCoverage(t)
}

func runHTTPMCPParityCoverage(t *testing.T) {
	t.Helper()

	if !hasHTTPMCPParityRecorded(t) {
		resetHTTPMCPParityCoverage()
		t.Run("page mutation", runLocalMCPProtocolPageMutationParity)
		t.Run("page operation", runLocalMCPProtocolPageOperationParity)
		t.Run("index and asset", runLocalMCPProtocolIndexAndAssetParity)
		t.Run("feature gated", runLocalMCPProtocolFeatureGatedToolParity)
	}
	assertHTTPMCPParityRecorded(t)
}

func newLocalMCPTestWiki(t *testing.T, enableRevision bool) *wiki.Wiki {
	t.Helper()

	w, _ := newLocalMCPTestWikiWithStorage(t, enableRevision)
	return w
}

func newLocalMCPTestWikiWithStorage(t *testing.T, enableRevision bool) (*wiki.Wiki, string) {
	t.Helper()

	storageDir := t.TempDir()
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          storageDir,
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		AuthDisabled:        true,
		EnableRevision:      enableRevision,
	})
	if err != nil {
		t.Fatalf("NewWiki failed: %v", err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Fatalf("Close wiki failed: %v", err)
		}
	})
	return w, storageDir
}

func newLocalMCPTestRouter(w *wiki.Wiki, opts httpinternal.RouterOptions) http.Handler {
	if opts.MCPEnabled && opts.MCPBindHost == "" {
		opts.MCPBindHost = "127.0.0.1"
	}
	return httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), opts)
}

func connectLocalMCP(t *testing.T, handler http.Handler, path string) *sdkmcp.ClientSession {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "leafwiki-test", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{
		Endpoint:             server.URL + path,
		HTTPClient:           server.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("Connect MCP client failed: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func listAllToolNames(t *testing.T, session *sdkmcp.ClientSession) []string {
	t.Helper()

	tools := listAllTools(t, session)
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func listAllTools(t *testing.T, session *sdkmcp.ClientSession) []*sdkmcp.Tool {
	t.Helper()

	var tools []*sdkmcp.Tool
	cursor := ""
	for {
		result, err := session.ListTools(context.Background(), &sdkmcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}
		for _, tool := range result.Tools {
			tools = append(tools, tool)
		}
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

func assertInputSchemasMatch(t *testing.T, tools []*sdkmcp.Tool, expected, expectedRequired, expectedAlternatives map[string][]string) {
	t.Helper()

	for _, tool := range tools {
		if _, ok := expected[tool.Name]; !ok {
			t.Fatalf("missing input schema expectation for tool %q", tool.Name)
		}
		if _, ok := expectedRequired[tool.Name]; !ok {
			t.Fatalf("missing input schema required expectation for tool %q", tool.Name)
		}
	}
	for name, props := range expected {
		tool := findTool(t, tools, name)
		schema := decodeToolSchema(t, "input", tool.Name, tool.InputSchema)
		properties := schemaProperties(t, "input", tool.Name, schema)
		gotProps := make([]string, 0, len(properties))
		for prop := range properties {
			gotProps = append(gotProps, prop)
			assertSchemaPropertyHasType(t, "input", name, prop, properties[prop])
		}
		sort.Strings(gotProps)
		assertSchemaPropertyOrderSorted(t, "input", name, schema)

		wantProps := append([]string{}, props...)
		sort.Strings(wantProps)
		if strings.Join(gotProps, "\n") != strings.Join(wantProps, "\n") {
			t.Fatalf("input schema for %s properties mismatch\n got: %v\nwant: %v", name, gotProps, wantProps)
		}

		assertStringSet(t, "input schema required for "+name, schemaStringSlice(schema["required"]), expectedRequired[name])
		if alternatives, ok := expectedAlternatives[name]; ok {
			assertRequiredAlternatives(t, name, schema, alternatives)
		}
	}
}

func assertOutputSchemasMatch(t *testing.T, tools []*sdkmcp.Tool, expected map[string][]string) {
	t.Helper()

	for _, tool := range tools {
		if tool.OutputSchema == nil {
			t.Fatalf("tool %s has no output schema", tool.Name)
		}
		if _, ok := expected[tool.Name]; !ok {
			t.Fatalf("missing output schema expectation for tool %q", tool.Name)
		}
	}
	for name, props := range expected {
		tool := findTool(t, tools, name)
		if tool.OutputSchema == nil {
			t.Fatalf("tool %s has no output schema", tool.Name)
		}
		schema := decodeToolSchema(t, "output", tool.Name, tool.OutputSchema)
		properties := schemaProperties(t, "output", tool.Name, schema)
		gotProps := make([]string, 0, len(properties))
		for prop := range properties {
			gotProps = append(gotProps, prop)
			assertSchemaPropertyHasType(t, "output", name, prop, properties[prop])
		}
		sort.Strings(gotProps)
		assertSchemaPropertyOrderSorted(t, "output", name, schema)

		wantProps := append([]string{}, props...)
		sort.Strings(wantProps)
		if strings.Join(gotProps, "\n") != strings.Join(wantProps, "\n") {
			t.Fatalf("output schema for %s properties mismatch\n got: %v\nwant: %v", name, gotProps, wantProps)
		}
		assertStringSet(t, "output schema required for "+name, schemaStringSlice(schema["required"]), props)
	}
}

func findTool(t *testing.T, tools []*sdkmcp.Tool, name string) *sdkmcp.Tool {
	t.Helper()

	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func decodeToolSchema(t *testing.T, kind, name string, schemaValue any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(schemaValue)
	if err != nil {
		t.Fatalf("marshal %s schema for %s: %v", kind, name, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode %s schema for %s: %v: %s", kind, name, err, raw)
	}
	if schema["type"] != "object" {
		t.Fatalf("%s schema for %s type = %v, want object: %#v", kind, name, schema["type"], schema)
	}
	return schema
}

func schemaProperties(t *testing.T, kind, name string, schema map[string]any) map[string]any {
	t.Helper()

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return properties
}

func assertSchemaPropertyOrderSorted(t *testing.T, kind, name string, schema map[string]any) {
	t.Helper()

	order := schemaStringSlice(schema["propertyOrder"])
	if len(order) == 0 {
		return
	}
	sortedOrder := append([]string{}, order...)
	sort.Strings(sortedOrder)
	if !reflect.DeepEqual(order, sortedOrder) {
		t.Fatalf("%s schema for %s propertyOrder = %v, want deterministic sorted order %v", kind, name, order, sortedOrder)
	}
}

func assertSchemaPropertyHasType(t *testing.T, kind, toolName, prop string, property any) {
	t.Helper()

	schema, ok := property.(map[string]any)
	if !ok {
		t.Fatalf("%s schema for %s.%s is not an object: %#v", kind, toolName, prop, property)
	}
	if _, ok := schema["type"]; ok {
		return
	}
	for _, key := range []string{"$ref", "anyOf", "oneOf", "allOf"} {
		if _, ok := schema[key]; ok {
			return
		}
	}
	t.Fatalf("%s schema for %s.%s has no type/ref/union: %#v", kind, toolName, prop, schema)
}

func assertRequiredAlternatives(t *testing.T, toolName string, schema map[string]any, alternatives []string) {
	t.Helper()

	anyOf, ok := schema["anyOf"].([]any)
	if !ok {
		t.Fatalf("input schema for %s has no anyOf required alternatives", toolName)
	}
	for _, alternative := range alternatives {
		found := false
		for _, raw := range anyOf {
			option, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if contains(schemaStringSlice(option["required"]), alternative) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("input schema for %s anyOf = %#v, want required alternative %q", toolName, anyOf, alternative)
		}
	}
}

func schemaStringSlice(value any) []string {
	rawItems, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(string)
		if ok {
			items = append(items, item)
		}
	}
	return items
}

func assertStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()

	got = append([]string{}, got...)
	want = append([]string{}, want...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s mismatch\n got: %v\nwant: %v", label, got, want)
	}
}

func copyToolInputProperties(src map[string][]string) map[string][]string {
	out := make(map[string][]string, len(src))
	for name, props := range src {
		out[name] = append([]string{}, props...)
	}
	return out
}

func assertToolNames(t *testing.T, got []string, want []string) {
	t.Helper()

	sortedWant := append([]string{}, want...)
	sort.Strings(sortedWant)
	if strings.Join(got, "\n") != strings.Join(sortedWant, "\n") {
		t.Fatalf("tool names mismatch\n got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(sortedWant, "\n"))
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func callToolStructured(t *testing.T, session *sdkmcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool %s failed: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("CallTool %s returned tool error: %#v", name, result.Content)
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("CallTool %s structured content has type %T: %#v", name, result.StructuredContent, result.StructuredContent)
	}
	return structured
}

func callToolError(t *testing.T, session *sdkmcp.ClientSession, name string, args map[string]any) string {
	t.Helper()

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool %s failed: %v", name, err)
	}
	if !result.IsError {
		t.Fatalf("CallTool %s succeeded, want tool error: %#v", name, result.StructuredContent)
	}
	for _, content := range result.Content {
		if text, ok := content.(*sdkmcp.TextContent); ok {
			return text.Text
		}
	}
	t.Fatalf("CallTool %s returned error without text content: %#v", name, result.Content)
	return ""
}

func getHTTPPageByPath(t *testing.T, router http.Handler, path string) map[string]any {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pages/by-path?path="+path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET page by path %q = %d: %s", path, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode HTTP page by path %q: %v", path, err)
	}
	return out
}

func getHTTPPageByID(t *testing.T, router http.Handler, pageID string) map[string]any {
	t.Helper()

	return getHTTPMap(t, router, "/api/pages/"+pageID)
}

func getHTTPValue(t *testing.T, router http.Handler, path string) any {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
	}
	return decodeJSONValue(t, "GET "+path, rec.Body.Bytes())
}

func getHTTPStatus(t *testing.T, router http.Handler, path string, wantStatus int) string {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != wantStatus {
		t.Fatalf("GET %s = %d, want %d: %s", path, rec.Code, wantStatus, rec.Body.String())
	}
	return rec.Body.String()
}

func getHTTPMap(t *testing.T, router http.Handler, path string) map[string]any {
	t.Helper()

	value := getHTTPValue(t, router, path)
	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("GET %s decoded as %T: %#v, want object", path, value, value)
	}
	return out
}

func updateHTTPPage(t *testing.T, router http.Handler, pageID string, payload map[string]any) map[string]any {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal HTTP page update: %v", err)
	}
	csrfToken, csrfCookies := issueHTTPCSRF(t, router)
	req := httptest.NewRequest(http.MethodPut, "/api/pages/"+pageID, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range csrfCookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT page %q = %d: %s", pageID, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode HTTP page update %q: %v", pageID, err)
	}
	return out
}

func putHTTPJSON(t *testing.T, router http.Handler, path string, payload map[string]any, wantStatus int) map[string]any {
	t.Helper()

	return requestHTTPJSON(t, router, http.MethodPut, path, payload, wantStatus)
}

func putHTTPJSONBody(t *testing.T, router http.Handler, path string, payload map[string]any, wantStatus int) string {
	t.Helper()

	return requestHTTPJSONBody(t, router, http.MethodPut, path, payload, wantStatus)
}

func postHTTPJSON(t *testing.T, router http.Handler, path string, payload map[string]any, wantStatus int) map[string]any {
	t.Helper()

	return requestHTTPJSON(t, router, http.MethodPost, path, payload, wantStatus)
}

func postHTTPJSONBody(t *testing.T, router http.Handler, path string, payload map[string]any, wantStatus int) string {
	t.Helper()

	return requestHTTPJSONBody(t, router, http.MethodPost, path, payload, wantStatus)
}

func postHTTPJSONNoContent(t *testing.T, router http.Handler, path string, payload map[string]any, wantStatus int) {
	t.Helper()

	body := postHTTPJSONBody(t, router, path, payload, wantStatus)
	if strings.TrimSpace(body) != "" {
		t.Fatalf("POST %s body = %q, want empty body", path, body)
	}
}

func requestHTTPJSON(t *testing.T, router http.Handler, method, path string, payload map[string]any, wantStatus int) map[string]any {
	t.Helper()

	raw := requestHTTPJSONBody(t, router, method, path, payload, wantStatus)
	return decodeJSONMap(t, method+" "+path, []byte(raw))
}

func requestHTTPJSONBody(t *testing.T, router http.Handler, method, path string, payload map[string]any, wantStatus int) string {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %s %s payload: %v", method, path, err)
	}
	csrfToken, csrfCookies := issueHTTPCSRF(t, router)
	req := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range csrfCookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
	return rec.Body.String()
}

func decodeJSONMap(t *testing.T, label string, raw []byte) map[string]any {
	t.Helper()

	value := decodeJSONValue(t, label, raw)
	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s decoded as %T: %#v, want object", label, value, value)
	}
	return out
}

func deleteHTTPStatus(t *testing.T, router http.Handler, path string, wantStatus int) string {
	t.Helper()

	csrfToken, csrfCookies := issueHTTPCSRF(t, router)
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range csrfCookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("DELETE %s = %d, want %d: %s", path, rec.Code, wantStatus, rec.Body.String())
	}
	return rec.Body.String()
}

func getHTTPSearch(t *testing.T, router http.Handler, values url.Values) map[string]any {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search?"+values.Encode(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/search = %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode HTTP search: %v", err)
	}
	return out
}

func uploadHTTPAsset(t *testing.T, router http.Handler, pageID, filename string, content []byte, wantStatus int) map[string]any {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file %q: %v", filename, err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file %q: %v", filename, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	csrfToken, csrfCookies := issueHTTPCSRF(t, router)
	req := httptest.NewRequest(http.MethodPost, "/api/pages/"+pageID+"/assets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, cookie := range csrfCookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("POST asset %s/%s = %d, want %d: %s", pageID, filename, rec.Code, wantStatus, rec.Body.String())
	}
	return decodeJSONMap(t, "POST asset "+pageID+"/"+filename, rec.Body.Bytes())
}

func getHTTPAssets(t *testing.T, router http.Handler, pageID string) map[string]any {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pages/"+pageID+"/assets", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET asset list for page %q = %d: %s", pageID, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode HTTP asset list for page %q: %v", pageID, err)
	}
	return out
}

func getHTTPAsset(t *testing.T, router http.Handler, pageID, filename string) string {
	t.Helper()

	body, _ := getHTTPAssetWithContentType(t, router, pageID, filename)
	return body
}

func getHTTPAssetWithContentType(t *testing.T, router http.Handler, pageID, filename string) (string, string) {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/"+pageID+"/"+url.PathEscape(filename), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET asset %s/%s = %d: %s", pageID, filename, rec.Code, rec.Body.String())
	}
	return rec.Body.String(), rec.Header().Get("Content-Type")
}

func getHTTPRevisionAsset(t *testing.T, router http.Handler, pageID, revisionID, filename string) (string, string) {
	t.Helper()

	rec := httptest.NewRecorder()
	path := "/api/pages/" + pageID + "/revisions/" + revisionID + "/assets/" + url.PathEscape(filename)
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET revision asset %s/%s/%s = %d: %s", pageID, revisionID, filename, rec.Code, rec.Body.String())
	}
	return rec.Body.String(), rec.Header().Get("Content-Type")
}

func stripRevisionAssetManifestMIME(t *testing.T, storageDir, manifestHash, assetName string) {
	t.Helper()

	manifestPath, manifest := readRevisionAssetManifest(t, storageDir, manifestHash)
	found := false
	for _, item := range manifest.Items {
		if item["name"] == assetName {
			delete(item, "mime_type")
			found = true
		}
	}
	if !found {
		t.Fatalf("asset manifest %s did not contain %q: %#v", manifestPath, assetName, manifest.Items)
	}
	updated, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal asset manifest %s: %v", manifestPath, err)
	}
	if err := os.WriteFile(manifestPath, updated, 0644); err != nil {
		t.Fatalf("write asset manifest %s: %v", manifestPath, err)
	}
}

func revisionAssetBlobPath(t *testing.T, storageDir, manifestHash, assetName string) string {
	t.Helper()

	_, manifest := readRevisionAssetManifest(t, storageDir, manifestHash)
	for _, item := range manifest.Items {
		if item["name"] != assetName {
			continue
		}
		hash, ok := item["sha256"].(string)
		if !ok || hash == "" {
			t.Fatalf("asset manifest entry for %q missing sha256: %#v", assetName, item)
		}
		if len(hash) < 2 {
			t.Fatalf("asset hash %q is too short", hash)
		}
		return filepath.Join(storageDir, ".leafwiki", "blobs", "assets", "sha256", hash[:2], hash)
	}
	t.Fatalf("asset manifest %q did not contain %q: %#v", manifestHash, assetName, manifest.Items)
	return ""
}

func readRevisionAssetManifest(t *testing.T, storageDir, manifestHash string) (string, struct {
	Items []map[string]any `json:"items"`
}) {
	t.Helper()

	if len(manifestHash) < 2 {
		t.Fatalf("asset manifest hash %q is too short", manifestHash)
	}
	manifestPath := filepath.Join(storageDir, ".leafwiki", "manifests", "assets", "sha256", manifestHash[:2], manifestHash+".json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read asset manifest %s: %v", manifestPath, err)
	}
	var manifest struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode asset manifest %s: %v", manifestPath, err)
	}
	return manifestPath, manifest
}

func getHTTPLatestRevision(t *testing.T, router http.Handler, pageID string) map[string]any {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pages/"+pageID+"/revisions/latest", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET latest revision for page %q = %d: %s", pageID, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode HTTP latest revision for page %q: %v", pageID, err)
	}
	return out
}

func getHTTPRevision(t *testing.T, router http.Handler, pageID, revisionID string) map[string]any {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pages/"+pageID+"/revisions/"+revisionID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET revision %s/%s = %d: %s", pageID, revisionID, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode HTTP revision %s/%s: %v", pageID, revisionID, err)
	}
	return out
}

func assertSearchResultsMatch(t *testing.T, mcpSearch, httpSearch map[string]any) {
	t.Helper()

	for _, field := range []string{"count", "offset", "limit"} {
		if mcpSearch[field] != httpSearch[field] {
			t.Fatalf("search %s mismatch: MCP=%v HTTP=%v", field, mcpSearch[field], httpSearch[field])
		}
	}
	mcpItems, ok := mcpSearch["items"].([]any)
	if !ok {
		t.Fatalf("MCP search items has type %T: %#v", mcpSearch["items"], mcpSearch["items"])
	}
	httpItems, ok := httpSearch["items"].([]any)
	if !ok {
		t.Fatalf("HTTP search items has type %T: %#v", httpSearch["items"], httpSearch["items"])
	}
	if len(mcpItems) != len(httpItems) {
		t.Fatalf("search item count mismatch: MCP=%d HTTP=%d", len(mcpItems), len(httpItems))
	}
	assertJSONEqual(t, "search items", mcpItems, httpItems)
	assertJSONEqual(t, "search tagFacets", mcpSearch["tagFacets"], httpSearch["tag_facets"])

	count, ok := httpSearch["count"].(float64)
	if !ok {
		t.Fatalf("HTTP search count has type %T: %#v", httpSearch["count"], httpSearch["count"])
	}
	offset, ok := httpSearch["offset"].(float64)
	if !ok {
		t.Fatalf("HTTP search offset has type %T: %#v", httpSearch["offset"], httpSearch["offset"])
	}
	wantHasMore := int(offset)+len(httpItems) < int(count)
	if mcpSearch["hasMore"] != wantHasMore {
		t.Fatalf("search hasMore mismatch: MCP=%v HTTP-derived=%v", mcpSearch["hasMore"], wantHasMore)
	}
}

func assertMapFieldsEqual(t *testing.T, label string, got, want map[string]any, fields []string) {
	t.Helper()

	for _, field := range fields {
		if !reflect.DeepEqual(normalizeJSON(t, got[field]), normalizeJSON(t, want[field])) {
			t.Fatalf("%s field %s mismatch:\n got: %#v\nwant: %#v", label, field, got[field], want[field])
		}
	}
}

func assertRestoredMetadata(t *testing.T, label string, page map[string]any) {
	t.Helper()

	if got := page["content"]; got != "metadata revision\n" {
		t.Fatalf("%s content = %v, want restored metadata revision content", label, got)
	}
	if got := strings.Join(stringSliceField(t, page, "tags"), ","); got != "restore,metadata" {
		t.Fatalf("%s tags = %v, want restore,metadata", label, got)
	}
	props := nestedMap(t, page, "properties")
	if got := props["status"]; got != "archived" {
		t.Fatalf("%s properties.status = %v, want archived", label, got)
	}
}

func assertPageVersionConflictParity(t *testing.T, label, mcpErr, httpBody string) {
	t.Helper()

	assertMCPPageError(t, label+" MCP", mcpErr, wikipages.ErrCodePageVersionConflict, "Page was changed by another request")
	assertHTTPPageError(t, label+" HTTP", httpBody, wikipages.ErrCodePageVersionConflict, "Page was changed by another request")
}

func assertMCPPageError(t *testing.T, label, errText, code, message string) {
	t.Helper()

	want := code + ": " + message
	if errText != want {
		t.Fatalf("%s error = %q, want %q", label, errText, want)
	}
}

func assertHTTPPageError(t *testing.T, label, body, code, message string) {
	t.Helper()

	payload := decodeJSONMap(t, label, []byte(body))
	errPayload := nestedMap(t, payload, "error")
	if got := errPayload["code"]; got != code {
		t.Fatalf("%s error.code = %v, want %q; body=%s", label, got, code, body)
	}
	if got := errPayload["message"]; got != message {
		t.Fatalf("%s error.message = %v, want %q; body=%s", label, got, message, body)
	}
}

func assertRestorePayloadsMatch(t *testing.T, mcpRestored, httpRestored map[string]any) {
	t.Helper()

	assertRestoreVolatileFieldsPresent(t, "MCP restore_revision", mcpRestored)
	assertRestoreVolatileFieldsPresent(t, "HTTP restore_revision", httpRestored)
	assertJSONEqual(t, "restore_revision response payload", normalizeRestorePayload(t, mcpRestored), normalizeRestorePayload(t, httpRestored))
}

func assertRestoreVolatileFieldsPresent(t *testing.T, label string, page map[string]any) {
	t.Helper()

	_ = stringField(t, page, "version")
	metadata := nestedMap(t, page, "metadata")
	updatedAt := stringField(t, metadata, "updatedAt")
	if _, err := time.Parse(time.RFC3339, updatedAt); err != nil {
		t.Fatalf("%s metadata.updatedAt = %q, want RFC3339 timestamp: %v", label, updatedAt, err)
	}
}

func normalizeRestorePayload(t *testing.T, page map[string]any) map[string]any {
	t.Helper()

	normalizedValue := normalizeJSON(t, page)
	normalized, ok := normalizedValue.(map[string]any)
	if !ok {
		t.Fatalf("restore payload normalized to %T, want object", normalizedValue)
	}
	delete(normalized, "version")
	if metadata, ok := normalized["metadata"].(map[string]any); ok {
		delete(metadata, "updatedAt")
	}
	return normalized
}

func assertPageState(t *testing.T, label string, page map[string]any, id, title, slug, pathValue, kind, parentID string) {
	t.Helper()

	want := map[string]string{
		"id":    id,
		"title": title,
		"slug":  slug,
		"path":  pathValue,
		"kind":  kind,
	}
	for field, expected := range want {
		if got := stringValue(page[field]); got != expected {
			t.Fatalf("%s field %s = %q, want %q; page=%#v", label, field, got, expected, page)
		}
	}
	if parentID == "" {
		if got := stringValue(page["parentId"]); got != "" {
			t.Fatalf("%s parentId = %q, want root parent; page=%#v", label, got, page)
		}
		return
	}
	if got := stringValue(page["parentId"]); got != parentID {
		t.Fatalf("%s parentId = %q, want %q; page=%#v", label, got, parentID, page)
	}
}

func assertChildrenDoNotContain(t *testing.T, label string, page map[string]any, childIDs ...string) {
	t.Helper()

	disallowed := map[string]struct{}{}
	for _, id := range childIDs {
		disallowed[id] = struct{}{}
	}
	children, _ := page["children"].([]any)
	for _, raw := range children {
		child, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("%s child has type %T: %#v", label, raw, raw)
		}
		id := stringValue(child["id"])
		if _, exists := disallowed[id]; exists {
			t.Fatalf("%s contains moved child %q: %#v", label, id, page["children"])
		}
	}
}

func assertChildOrder(t *testing.T, label string, page map[string]any, childIDs ...string) {
	t.Helper()

	children, ok := page["children"].([]any)
	if !ok {
		t.Fatalf("%s children has type %T: %#v", label, page["children"], page["children"])
	}
	if len(children) != len(childIDs) {
		t.Fatalf("%s child count = %d, want %d: %#v", label, len(children), len(childIDs), page["children"])
	}
	for i, wantID := range childIDs {
		child, ok := children[i].(map[string]any)
		if !ok {
			t.Fatalf("%s child %d has type %T: %#v", label, i, children[i], children[i])
		}
		if got := stringValue(child["id"]); got != wantID {
			t.Fatalf("%s child %d id = %q, want %q: %#v", label, i, got, wantID, page["children"])
		}
	}
}

func readPageMarkdownByRoutePath(t *testing.T, storageDir, routePath string) string {
	t.Helper()

	path := filepath.Join(append([]string{storageDir, "root"}, strings.Split(routePath, "/")...)...) + ".md"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown page %s: %v", path, err)
	}
	return string(raw)
}

func assertMCPToolErrorContains(t *testing.T, session *sdkmcp.ClientSession, name string, args map[string]any, want string) {
	t.Helper()

	errText := callToolError(t, session, name, args)
	if !strings.Contains(strings.ToLower(errText), strings.ToLower(want)) {
		t.Fatalf("%s error = %q, want detail containing %q", name, errText, want)
	}
}

func assertErrorContainsAny(t *testing.T, label, errText string, wants ...string) {
	t.Helper()

	lower := strings.ToLower(errText)
	for _, want := range wants {
		if strings.Contains(lower, strings.ToLower(want)) {
			return
		}
	}
	t.Fatalf("%s error = %q, want one of %q", label, errText, wants)
}

func assertErrorDoesNotContainAny(t *testing.T, label, errText string, rejects ...string) {
	t.Helper()

	lower := strings.ToLower(errText)
	for _, reject := range rejects {
		if strings.Contains(lower, strings.ToLower(reject)) {
			t.Fatalf("%s error = %q, did not want detail containing %q", label, errText, reject)
		}
	}
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func assertAssetURLResult(t *testing.T, label string, result map[string]any, field, pageID string) {
	t.Helper()

	raw, ok := result[field].(string)
	if !ok {
		t.Fatalf("%s %s has type %T: %#v, want string", label, field, result[field], result[field])
	}
	prefix := "/assets/" + pageID + "/"
	if !strings.HasPrefix(raw, prefix) || len(raw) <= len(prefix) {
		t.Fatalf("%s %s = %q, want asset URL under %s", label, field, raw, prefix)
	}
	if len(result) != 1 {
		t.Fatalf("%s result = %#v, want only %q field", label, result, field)
	}
}

func assertJSONEqual(t *testing.T, label string, got, want any) {
	t.Helper()

	normalizedGot := normalizeJSON(t, got)
	normalizedWant := normalizeJSON(t, want)
	if !reflect.DeepEqual(normalizedGot, normalizedWant) {
		gotJSON, _ := json.MarshalIndent(normalizedGot, "", "  ")
		wantJSON, _ := json.MarshalIndent(normalizedWant, "", "  ")
		t.Fatalf("%s mismatch:\n got: %s\nwant: %s", label, gotJSON, wantJSON)
	}
}

func normalizeJSON(t *testing.T, value any) any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON value %T: %v", value, err)
	}
	return decodeJSONValue(t, "normalize JSON", raw)
}

func decodeJSONValue(t *testing.T, label string, raw []byte) any {
	t.Helper()

	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v: %s", label, err, raw)
	}
	return out
}

func issueHTTPCSRF(t *testing.T, router http.Handler) (string, []*http.Cookie) {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config for CSRF = %d: %s", rec.Code, rec.Body.String())
	}
	token := rec.Header().Get("X-CSRF-Token")
	result := rec.Result()
	defer result.Body.Close()
	if token == "" {
		for _, cookie := range result.Cookies() {
			if cookie.Name == "leafwiki_csrf" || cookie.Name == "__Host-leafwiki_csrf" {
				token = cookie.Value
				break
			}
		}
	}
	if token == "" {
		t.Fatalf("GET /api/config did not issue CSRF token")
	}
	return token, result.Cookies()
}

func nestedMap(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()

	nested, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("%q has type %T: %#v", key, value[key], value[key])
	}
	return nested
}

func stringField(t *testing.T, value map[string]any, key string) string {
	t.Helper()

	s, ok := value[key].(string)
	if !ok || s == "" {
		t.Fatalf("%q has type %T and value %#v, want non-empty string", key, value[key], value[key])
	}
	return s
}

func stringSliceField(t *testing.T, value map[string]any, key string) []string {
	t.Helper()

	raw, ok := value[key].([]any)
	if !ok {
		t.Fatalf("%q has type %T: %#v", key, value[key], value[key])
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("%q item has type %T: %#v", key, item, item)
		}
		out = append(out, s)
	}
	return out
}

func arrayContainsObjectField(value any, field string, want any) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if ok && obj[field] == want {
			return true
		}
	}
	return false
}

func arrayContainsString(value any, want string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
