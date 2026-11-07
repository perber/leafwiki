package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/test_utils"
)

// TestMCP_SessionIsBoundToItsOwner is the owner-binding end to end (ADR-0019):
// a session ID is valid only for the authenticated user who created it. User
// B presenting A's ID gets the same 404 the transport answers for an unknown
// ID — B cannot list tools, call a tool, or terminate the session over it —
// while A's own session works end to end.
func TestMCP_SessionIsBoundToItsOwner(t *testing.T) {
	w, router := newMCPRouterTest(t, false)
	sessA := loginSession(t, router, "admin", "adminpassword")
	if _, err := w.UserService().CreateUser("mcp-owner-b", "mcp-owner-b@example.com", "ownerbpass123", coreauth.RoleViewer); err != nil {
		t.Fatalf("CreateUser (second user) error: %v", err)
	}
	sessB := loginSession(t, router, "mcp-owner-b", "ownerbpass123")

	initRec := mcpPost(t, router, mcpInitializeBody, "", &sessA)
	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for A's initialize, got %d: %s", initRec.Code, initRec.Body.String())
	}
	sessionID := initRec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected the initialize response to carry a Mcp-Session-Id header")
	}

	t.Run("B presenting A's session ID gets a 404", func(t *testing.T) {
		rec := mcpPost(t, router, mcpToolsListBody, sessionID, &sessB)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for another user's session ID, got %d: %.200s", rec.Code, rec.Body.String())
		}
		// A tool call over the same ID is refused the same way.
		callBody := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_tree","arguments":{}}}`
		rec = mcpPost(t, router, callBody, sessionID, &sessB)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for a tool call over another user's session ID, got %d: %.200s", rec.Code, rec.Body.String())
		}
	})

	t.Run("B cannot terminate A's session", func(t *testing.T) {
		// B's DELETE is a protocol-level no-op for an ID B does not own (200,
		// like terminating any other unknown ID)…
		delRec := mcpDelete(t, router, sessionID, &sessB)
		if delRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for B's DELETE (a no-op for an ID B does not own), got %d: %.200s", delRec.Code, delRec.Body.String())
		}
		// …so A's session is still live.
		rec := mcpPost(t, router, mcpToolsListBody, sessionID, &sessA)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected A's session to survive B's DELETE, got %d: %.200s", rec.Code, rec.Body.String())
		}
	})

	t.Run("B presenting A's session ID on GET gets a 404, not a stream", func(t *testing.T) {
		// mcp-go's GET handler does not validate the session ID (the spec
		// does not require it for GET), so the owner binding is enforced in
		// the gin wrapper: B gets the transport's own 404 for an unknown
		// ID — plain text, not an event stream — never A's live stream.
		rec := mcpGet(t, router, sessionID, &sessB)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for another user's session ID on GET, got %d: %.200s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Fatalf("expected the plain-text refusal, got content-type %q", ct)
		}
	})

	t.Run("A opening her own stream on GET still works", func(t *testing.T) {
		// The owner's own session ID must still open the stream: a GET with
		// the owner's ID passes the wrapper's check and mcp-go serves the
		// stream as before (the single-user SSE behaviour is unchanged).
		done := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer close(done)
			router.ServeHTTP(w, req)
		}))
		t.Cleanup(srv.Close)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/mcp", nil)
		if err != nil {
			t.Fatalf("build GET request: %v", err)
		}
		req.Header.Set("Mcp-Session-Id", sessionID)
		for _, c := range sessA.cookies {
			req.AddCookie(c)
		}

		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("GET /api/mcp: %v", err)
		}
		defer test_utils.WrapCloseWithErrorCheck(resp.Body.Close, t)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 opening A's own SSE stream, got %d", resp.StatusCode)
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
	})

	t.Run("A's own session works end to end", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_page_by_path","arguments":{"path":"welcome-to-leafwiki"}}}`
		text := mcpToolCallText(t, router, body, sessionID, &sessA)
		if !strings.Contains(text, "Welcome to LeafWiki") {
			t.Fatalf("get_page_by_path did not return the welcome page:\n%s", text)
		}
	})
}
