package mcp

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// discardLog is the logger the tool constructors take in tests that do not
// assert on logging (TestToolResult_LogsInternalErrorOnce pins the logging
// itself).
var discardLog = slog.New(slog.NewTextHandler(io.Discard, nil))

// recordingOps is a fake PageOps that records which method was called (and with
// which argument) and returns scripted results/errors. A zero value returns a
// valid default for every method, so a role-allowed call never panics.
type recordingOps struct {
	calls     []string
	lastLimit int

	page      *Page
	pageErr   error
	search    *SearchResult
	searchErr error
	tree      []TreeNode
	treeErr   error

	lastCreate CreatePageInput
	lastUpdate UpdatePageInput

	created   *Page
	createErr error
	updated   *Page
	updateErr error
}

func (f *recordingOps) GetPage(_ context.Context, id string) (*Page, error) {
	f.calls = append(f.calls, "GetPage:"+id)
	if f.pageErr != nil {
		return nil, f.pageErr
	}
	if f.page != nil {
		return f.page, nil
	}
	return defaultPage(), nil
}

func (f *recordingOps) GetPageByPath(_ context.Context, routePath string) (*Page, error) {
	f.calls = append(f.calls, "GetPageByPath:"+routePath)
	if f.pageErr != nil {
		return nil, f.pageErr
	}
	if f.page != nil {
		return f.page, nil
	}
	return defaultPage(), nil
}

func (f *recordingOps) Search(_ context.Context, query string, limit int) (*SearchResult, error) {
	f.calls = append(f.calls, "Search:"+query)
	f.lastLimit = limit
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if f.search != nil {
		return f.search, nil
	}
	return &SearchResult{}, nil
}

func (f *recordingOps) ListTree(context.Context) ([]TreeNode, error) {
	f.calls = append(f.calls, "ListTree")
	if f.treeErr != nil {
		return nil, f.treeErr
	}
	if f.tree != nil {
		return f.tree, nil
	}
	return nil, nil
}

func (f *recordingOps) CreatePage(_ context.Context, in CreatePageInput) (*Page, error) {
	f.calls = append(f.calls, "CreatePage:"+in.Title)
	f.lastCreate = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	return defaultPage(), nil
}

func (f *recordingOps) UpdatePage(_ context.Context, in UpdatePageInput) (*Page, error) {
	f.calls = append(f.calls, "UpdatePage:"+in.ID)
	f.lastUpdate = in
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updated != nil {
		return f.updated, nil
	}
	return defaultPage(), nil
}

func defaultPage() *Page {
	return &Page{
		ID:         "p-1",
		Title:      "Hello",
		Path:       "hello",
		Kind:       "page",
		Content:    "line one\nline two",
		Tags:       []string{},
		Properties: map[string]string{},
	}
}

// toolCtx builds a request context carrying the given user (nil = no caller).
func toolCtx(user *coreauth.User) context.Context {
	ctx := context.Background()
	if user != nil {
		ctx = context.WithValue(ctx, mcpUserKey, user)
	}
	return ctx
}

func mkUser(role string) *coreauth.User {
	return &coreauth.User{ID: "u-1", Username: "u", Role: role}
}

// callTool runs a tool's handler with the given args and returns the result text.
func callTool(t *testing.T, tool mcpserver.ServerTool, ctx context.Context, args map[string]any) (string, *mcpgo.CallToolResult) {
	t.Helper()
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}
	res, err := tool.Handler(ctx, req)
	if err != nil {
		t.Fatalf("handler returned a transport error: %v", err)
	}
	return resultText(t, res), res
}

// resultText extracts the first text content of a tool result.
func resultText(t *testing.T, res *mcpgo.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := c.(mcpgo.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// assertRoleGate runs the role matrix against a tool: a nil caller and a caller
// with an empty role are denied with mcp_forbidden and never reach the delegate,
// while viewer (the minimum) and higher roles proceed.
func assertRoleGate(t *testing.T, ctor func(*slog.Logger, PageOps) mcpserver.ServerTool, args map[string]any) {
	t.Helper()
	cases := []struct {
		name   string
		user   *coreauth.User
		denied bool
	}{
		{"nil user is denied (fail closed)", nil, true},
		{"empty role is denied", mkUser(""), true},
		{"viewer is allowed", mkUser(coreauth.RoleViewer), false},
		{"editor is allowed", mkUser(coreauth.RoleEditor), false},
		{"admin is allowed", mkUser(coreauth.RoleAdmin), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := &recordingOps{}
			tool := ctor(discardLog, ops)
			text, _ := callTool(t, tool, toolCtx(tc.user), args)
			if tc.denied {
				if !strings.Contains(text, ErrCodeMCPForbidden) {
					t.Fatalf("expected %s in %q", ErrCodeMCPForbidden, text)
				}
				if len(ops.calls) != 0 {
					t.Fatalf("denied call must not delegate, got %v", ops.calls)
				}
			} else if len(ops.calls) == 0 {
				t.Fatalf("allowed role did not reach the delegate; result %q", text)
			}
		})
	}
}

// assertEditorRoleGate runs the role matrix against an editor-gated tool: a
// nil caller, an empty role and a viewer are denied with mcp_forbidden and
// never reach the delegate, while editor (the minimum) and admin proceed.
func assertEditorRoleGate(t *testing.T, ctor func(*slog.Logger, PageOps) mcpserver.ServerTool, args map[string]any) {
	t.Helper()
	cases := []struct {
		name   string
		user   *coreauth.User
		denied bool
	}{
		{"nil user is denied (fail closed)", nil, true},
		{"empty role is denied", mkUser(""), true},
		{"viewer is denied", mkUser(coreauth.RoleViewer), true},
		{"editor is allowed", mkUser(coreauth.RoleEditor), false},
		{"admin is allowed", mkUser(coreauth.RoleAdmin), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := &recordingOps{}
			tool := ctor(discardLog, ops)
			text, _ := callTool(t, tool, toolCtx(tc.user), args)
			if tc.denied {
				if !strings.Contains(text, ErrCodeMCPForbidden) {
					t.Fatalf("expected %s in %q", ErrCodeMCPForbidden, text)
				}
				if len(ops.calls) != 0 {
					t.Fatalf("denied call must not delegate, got %v", ops.calls)
				}
			} else if len(ops.calls) == 0 {
				t.Fatalf("allowed role did not reach the delegate; result %q", text)
			}
		})
	}
}

// assertInvalidInput confirms a tool answers mcp_invalid_input (and delegates
// nothing) when a required argument is missing.
func assertInvalidInput(t *testing.T, ctor func(*slog.Logger, PageOps) mcpserver.ServerTool, arg string) {
	t.Helper()
	ops := &recordingOps{}
	tool := ctor(discardLog, ops)
	text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{})
	if !strings.Contains(text, ErrCodeMCPInvalidInput) {
		t.Fatalf("missing %q: expected %s, got %q", arg, ErrCodeMCPInvalidInput, text)
	}
	if len(ops.calls) != 0 {
		t.Fatalf("invalid input must not delegate, got %v", ops.calls)
	}
}

// assertCodePassthrough confirms a use-case LocalizedError from the delegate
// surfaces with its code intact. The page_not_found rows carry the exact error
// the real adapter now produces for a missing page (pinned in
// internal/wiki/mcp_adapter_test.go); the slice itself stays service-free.
func assertCodePassthrough(t *testing.T, ctor func(*slog.Logger, PageOps) mcpserver.ServerTool, args map[string]any, code string, setErr func(*recordingOps)) {
	t.Helper()
	ops := &recordingOps{}
	setErr(ops)
	tool := ctor(discardLog, ops)
	text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), args)
	if !strings.Contains(text, code) {
		t.Fatalf("expected the use-case code %q in the result, got %q", code, text)
	}
}

// assertWriteCodePassthrough is the write-tool version of
// assertCodePassthrough: it runs the tool as an editor, the minimum role the
// write tools require, so the delegate runs and the scripted error surfaces.
func assertWriteCodePassthrough(t *testing.T, ctor func(*slog.Logger, PageOps) mcpserver.ServerTool, args map[string]any, code string, setErr func(*recordingOps)) {
	t.Helper()
	ops := &recordingOps{}
	setErr(ops)
	tool := ctor(discardLog, ops)
	text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleEditor)), args)
	if !strings.Contains(text, code) {
		t.Fatalf("expected the use-case code %q in the result, got %q", code, text)
	}
}

// ─── get_page ───────────────────────────────────────────────────────────────

func TestGetPageTool(t *testing.T) {
	ctor := newGetPageTool

	t.Run("success renders metadata and line-numbered content", func(t *testing.T) {
		ops := &recordingOps{page: &Page{
			ID:      "p-9",
			Title:   "Spec Page",
			Path:    "docs/spec",
			Kind:    "page",
			Content: "# Title\nbody line",
			Tags:    []string{"a", "b"},
			Properties: map[string]string{
				"status": "draft",
			},
			CreatedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt: time.Date(2025, 1, 3, 4, 5, 6, 0, time.UTC),
		}}
		tool := newGetPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": "p-9"})
		for _, want := range []string{
			"Spec Page", "ID: p-9", "Path: docs/spec", "Kind: page",
			"Created: 2025-01-02 03:04:05", "Updated: 2025-01-03 04:05:06",
			"Tags: a, b", "status: draft", "0001 | # Title", "0002 | body line",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in result:\n%s", want, text)
			}
		}
		if res.IsError {
			t.Fatalf("success result must not be an error: %s", text)
		}
	})

	t.Run("missing id is invalid input", func(t *testing.T) {
		assertInvalidInput(t, ctor, "id")
	})

	t.Run("an id at the length cap delegates", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": strings.Repeat("x", maxPageIDArgLength)})
		if len(ops.calls) != 1 {
			t.Fatalf("an id at the cap must delegate, got %v", ops.calls)
		}
	})

	t.Run("an id over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"id": strings.Repeat("x", maxPageIDArgLength+1)})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized id, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized id must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertCodePassthrough(t, ctor, map[string]any{"id": "p-9"}, "page_not_found",
			func(f *recordingOps) {
				f.pageErr = sharederrors.NewLocalizedError("page_not_found", "Page not found", "page not found", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertRoleGate(t, ctor, map[string]any{"id": "p-9"})
	})
}

// ─── get_page_by_path ────────────────────────────────────────────────────────

func TestGetPageByPathTool(t *testing.T) {
	ctor := newGetPageByPathTool

	t.Run("trims surrounding slashes and whitespace before delegating", func(t *testing.T) {
		// The REST route trims the same way (TrimSpace plus slash trim), so
		// both are a client convenience the tool normalises away.
		tests := []struct {
			name string
			path string
		}{
			{name: "leading and trailing slash", path: "/docs/intro/"},
			{name: "surrounding whitespace and slashes", path: "  /docs/intro/  "},
			{name: "surrounding whitespace", path: "  docs/intro  "},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				ops := &recordingOps{page: &Page{ID: "p-2", Title: "T", Path: "docs/intro", Kind: "page", Content: "x"}}
				tool := newGetPageByPathTool(discardLog, ops)
				callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"path": tc.path})
				if len(ops.calls) != 1 || ops.calls[0] != "GetPageByPath:docs/intro" {
					t.Fatalf("expected the trimmed route path, got %v", ops.calls)
				}
			})
		}
	})

	t.Run("missing path is invalid input", func(t *testing.T) {
		assertInvalidInput(t, ctor, "path")
	})

	t.Run("whitespace-only path is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageByPathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"path": "  "})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for a whitespace-only path, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("a whitespace-only path must not delegate, got %v", ops.calls)
		}
	})

	t.Run("a path at the length cap delegates", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageByPathTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"path": strings.Repeat("x", maxRoutePathArgLength)})
		if len(ops.calls) != 1 {
			t.Fatalf("a path at the cap must delegate, got %v", ops.calls)
		}
	})

	t.Run("a path over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{}
		tool := newGetPageByPathTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"path": strings.Repeat("x", maxRoutePathArgLength+1)})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized path, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized path must not delegate, got %v", ops.calls)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertCodePassthrough(t, ctor, map[string]any{"path": "docs/intro"}, "page_not_found",
			func(f *recordingOps) {
				f.pageErr = sharederrors.NewLocalizedError("page_not_found", "Page not found", "page not found", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertRoleGate(t, ctor, map[string]any{"path": "docs/intro"})
	})
}

// TestGetPageByPathTool_HostilePaths pins the path-traversal guard: every
// hostile input must come back as an errored tool result (invalid input when
// the trimmed path is empty, not-found otherwise) — never as page content.
// The property is structurally guaranteed (the route lookup walks slugs and
// rejects empty or missing segments, so no traversal can resolve to a page),
// but the matrix keeps it pinned if the lookup ever changes. The fake
// delegate answers like the real adapter does for any unknown route: the
// page_not_found LocalizedError.
func TestGetPageByPathTool_HostilePaths(t *testing.T) {
	pageNotFound := sharederrors.NewLocalizedError("page_not_found", "Page not found", "page not found", nil)
	tests := []struct {
		name      string
		path      string
		want      string // the error code expected in the result text
		delegated bool
	}{
		{name: "single parent reference", path: "..", want: "page_not_found", delegated: true},
		{name: "nested parent reference", path: "../..", want: "page_not_found", delegated: true},
		{name: "traversal to an absolute system file", path: "../../etc/passwd", want: "page_not_found", delegated: true},
		{name: "absolute system file", path: "/etc/passwd", want: "page_not_found", delegated: true},
		{name: "only separators", path: "///", want: ErrCodeMCPInvalidInput, delegated: false},
		{name: "windows traversal", path: `..\..\windows\system32`, want: "page_not_found", delegated: true},
		{name: "windows drive path", path: "C:/Users", want: "page_not_found", delegated: true},
		{name: "windows drive path with backslashes", path: `C:\Users`, want: "page_not_found", delegated: true},
		{name: "empty segment in the middle", path: "a//b", want: "page_not_found", delegated: true},
		{name: "traversal resolving to an existing page", path: "docs/../welcome-to-leafwiki", want: "page_not_found", delegated: true},
		{name: "trailing suffix traversal", path: "welcome-to-leafwiki/../..", want: "page_not_found", delegated: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ops := &recordingOps{pageErr: pageNotFound}
			tool := newGetPageByPathTool(discardLog, ops)
			text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"path": tc.path})
			if !res.IsError {
				t.Fatalf("hostile path %q produced a successful result: %s", tc.path, text)
			}
			if !strings.Contains(text, tc.want) {
				t.Fatalf("expected code %q in the result, got %q", tc.want, text)
			}
			if got := len(ops.calls) == 1; got != tc.delegated {
				t.Fatalf("delegated = %v, want %v (calls %v)", got, tc.delegated, ops.calls)
			}
			// A failed call must never carry page content (line-numbered
			// body lines start with "NNNN | ").
			if strings.Contains(text, " | ") {
				t.Fatalf("hostile path %q produced page content: %s", tc.path, text)
			}
		})
	}
}

// ─── search ──────────────────────────────────────────────────────────────────

func TestSearchTool(t *testing.T) {
	ctor := newSearchTool

	t.Run("success renders matches and passes a bounded limit", func(t *testing.T) {
		ops := &recordingOps{search: &SearchResult{
			Count: 1,
			Items: []SearchItem{{ID: "p-1", Title: "Found Page", Path: "found", Kind: "page", Excerpt: "an excerpt"}},
		}}
		tool := newSearchTool(discardLog, ops)
		text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"query": "needle"})
		for _, want := range []string{"Found 1 results (showing 1)", "Found Page", "ID: p-1", "Path: found", "Excerpt: an excerpt"} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in result:\n%s", want, text)
			}
		}
	})

	t.Run("the header disambiguates total matches from shown items", func(t *testing.T) {
		ops := &recordingOps{search: &SearchResult{
			Count: 25,
			Items: []SearchItem{
				{ID: "p-1", Title: "A", Path: "a", Kind: "page"},
				{ID: "p-2", Title: "B", Path: "b", Kind: "page"},
				{ID: "p-3", Title: "C", Path: "c", Kind: "page"},
			},
		}}
		tool := newSearchTool(discardLog, ops)
		text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"query": "needle", "limit": 3})
		if !strings.Contains(text, "Found 25 results (showing 3)") {
			t.Fatalf("expected the header to state the total and the shown count, got:\n%s", text)
		}
	})

	t.Run("an over-cap limit is clamped to the maximum", func(t *testing.T) {
		ops := &recordingOps{search: &SearchResult{}}
		tool := newSearchTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"query": "q", "limit": 9999})
		if ops.lastLimit != searchMaxLimit {
			t.Fatalf("limit = %d, want the cap %d", ops.lastLimit, searchMaxLimit)
		}
	})

	t.Run("a non-positive limit falls back to the default", func(t *testing.T) {
		ops := &recordingOps{search: &SearchResult{}}
		tool := newSearchTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"query": "q", "limit": 0})
		if ops.lastLimit != searchDefaultLimit {
			t.Fatalf("limit = %d, want the default %d", ops.lastLimit, searchDefaultLimit)
		}
	})

	t.Run("a query at the length cap delegates", func(t *testing.T) {
		ops := &recordingOps{search: &SearchResult{}}
		tool := newSearchTool(discardLog, ops)
		callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"query": strings.Repeat("x", maxSearchQueryArgLength)})
		if len(ops.calls) != 1 {
			t.Fatalf("a query at the cap must delegate, got %v", ops.calls)
		}
	})

	t.Run("a query over the length cap is invalid input", func(t *testing.T) {
		ops := &recordingOps{search: &SearchResult{}}
		tool := newSearchTool(discardLog, ops)
		text, res := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), map[string]any{"query": strings.Repeat("x", maxSearchQueryArgLength+1)})
		if !res.IsError || !strings.Contains(text, ErrCodeMCPInvalidInput) {
			t.Fatalf("expected %s for an oversized query, got %q", ErrCodeMCPInvalidInput, text)
		}
		if len(ops.calls) != 0 {
			t.Fatalf("an oversized query must not delegate, got %v", ops.calls)
		}
	})

	t.Run("missing query is invalid input", func(t *testing.T) {
		assertInvalidInput(t, ctor, "query")
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertCodePassthrough(t, ctor, map[string]any{"query": "q"}, "search_unavailable",
			func(f *recordingOps) {
				f.searchErr = sharederrors.NewLocalizedError("search_unavailable", "Search unavailable", "search unavailable", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertRoleGate(t, ctor, map[string]any{"query": "q"})
	})
}

// ─── list_tree ───────────────────────────────────────────────────────────────

func TestListTreeTool(t *testing.T) {
	ctor := newListTreeTool

	t.Run("renders the tree with nested indentation", func(t *testing.T) {
		ops := &recordingOps{tree: []TreeNode{
			{ID: "s-1", Title: "Docs", Slug: "docs", Kind: "section", Path: "docs",
				Children: []TreeNode{{ID: "p-1", Title: "Intro", Slug: "intro", Kind: "page", Path: "docs/intro"}}},
		}}
		tool := newListTreeTool(discardLog, ops)
		text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), nil)
		wantRoot := "- Docs [s-1] (section) /docs\n"
		wantChild := "  - Intro [p-1] (page) /docs/intro\n"
		if !strings.Contains(text, wantRoot) || !strings.Contains(text, wantChild) {
			t.Fatalf("expected the tree with a nested child, got:\n%s", text)
		}
	})

	t.Run("empty tree is reported", func(t *testing.T) {
		ops := &recordingOps{tree: nil}
		tool := newListTreeTool(discardLog, ops)
		text, _ := callTool(t, tool, toolCtx(mkUser(coreauth.RoleViewer)), nil)
		if text != "The wiki is empty." {
			t.Fatalf("expected the empty message, got %q", text)
		}
	})

	t.Run("localized error keeps its code", func(t *testing.T) {
		assertCodePassthrough(t, ctor, nil, "page_internal_error",
			func(f *recordingOps) {
				f.treeErr = sharederrors.NewLocalizedError("page_internal_error", "Tree not loaded", "tree not loaded", nil)
			},
		)
	})

	t.Run("role gate", func(t *testing.T) {
		assertRoleGate(t, ctor, nil)
	})
}
