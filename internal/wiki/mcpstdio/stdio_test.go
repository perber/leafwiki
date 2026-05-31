package mcpstdio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfigValidateDefaultsAndEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Endpoint != "http://127.0.0.1:8080/mcp" {
		t.Fatalf("default endpoint = %q", cfg.Endpoint)
	}
	if cfg.RequestTimeout != 2*time.Minute {
		t.Fatalf("default request timeout = %v", cfg.RequestTimeout)
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Fatalf("default shutdown timeout = %v", cfg.ShutdownTimeout)
	}
	if cfg.MaxFrameSize != 128*1024*1024 {
		t.Fatalf("default max frame size = %d", cfg.MaxFrameSize)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}

	cfg.Endpoint = "not a url"
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid endpoint should fail validation")
	}
}

func TestRunForwardsFramesAndTracksSessionAndProtocol(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	var sessionHeaders []string
	var protocolHeaders []string
	var authHeaders []string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST or DELETE", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		mu.Lock()
		bodies = append(bodies, string(body))
		sessionHeaders = append(sessionHeaders, r.Header.Get("Mcp-Session-Id"))
		protocolHeaders = append(protocolHeaders, r.Header.Get("Mcp-Protocol-Version"))
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		mu.Unlock()

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Accept") != "application/json, text/event-stream" {
			t.Errorf("accept = %q", r.Header.Get("Accept"))
		}

		if strings.Contains(string(body), `"initialize"`) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "session-123")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`))
	}))
	defer upstream.Close()

	stdin := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test"}}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n",
	)
	var stdout, stderr bytes.Buffer
	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	cfg.APIKey = "lwk_secret_token"

	if err := Run(context.Background(), cfg, stdin, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("upstream body count = %d, want 2", len(bodies))
	}
	if bodies[0] != `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test"}}}` {
		t.Fatalf("initialize body was not forwarded unchanged: %s", bodies[0])
	}
	if bodies[1] != `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` {
		t.Fatalf("tools/list body was not forwarded unchanged: %s", bodies[1])
	}
	if sessionHeaders[0] != "" || sessionHeaders[1] != "session-123" {
		t.Fatalf("session headers = %#v", sessionHeaders)
	}
	if protocolHeaders[0] != "" || protocolHeaders[1] != "2025-11-25" {
		t.Fatalf("protocol headers = %#v", protocolHeaders)
	}
	if authHeaders[0] != "Bearer lwk_secret_token" || authHeaders[1] != "Bearer lwk_secret_token" {
		t.Fatalf("auth headers = %#v", authHeaders)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout lines = %#v", lines)
	}
	assertJSONEqual(t, lines[0], `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`)
	assertJSONEqual(t, lines[1], `{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`)
	if strings.Contains(stdout.String(), "lwk_secret_token") || strings.Contains(stderr.String(), "lwk_secret_token") {
		t.Fatal("api key leaked to output")
	}
}

func TestRunNotificationAcceptedEmitsNoStdout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunRequestWithAcceptedEmptyResponseReturnsError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"tools/list"}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, float64(4))
	if !strings.Contains(stderr.String(), "empty") {
		t.Fatalf("stderr = %q, want empty response diagnostic", stderr.String())
	}
}

func TestRunResponseFrameWithAcceptedEmptyResponseEmitsNoStdout(t *testing.T) {
	var gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	input := `{"jsonrpc":"2.0","id":4,"result":{"ok":true}}`
	err := Run(context.Background(), cfg, strings.NewReader(input+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotBody != input {
		t.Fatalf("forwarded body = %q, want %q", gotBody, input)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunDoesNotMirrorTools(t *testing.T) {
	var methods []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		methods = append(methods, req.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	if err := Run(context.Background(), cfg, strings.NewReader(input), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := strings.Join(methods, ","); got != "initialize" {
		t.Fatalf("upstream methods = %q, want only initialize", got)
	}
}

func TestRunOnlyInitializeResponseUpdatesProtocolVersion(t *testing.T) {
	var protocolHeaders []string
	var count int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		count++
		protocolHeaders = append(protocolHeaders, r.Header.Get("Mcp-Protocol-Version"))
		w.Header().Set("Content-Type", "application/json")
		switch count {
		case 1:
			w.Header().Set("Mcp-Session-Id", "session-123")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`))
		case 2:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"protocolVersion":"poisoned-tool-result"}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"ok":true}}`))
		}
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call"}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}` + "\n"
	err := Run(context.Background(), cfg, strings.NewReader(input), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(protocolHeaders) != 3 {
		t.Fatalf("protocol headers = %#v", protocolHeaders)
	}
	if protocolHeaders[0] != "" || protocolHeaders[1] != "2025-11-25" || protocolHeaders[2] != "2025-11-25" {
		t.Fatalf("protocol headers = %#v", protocolHeaders)
	}
}

func TestRunMalformedJSONReturnsParseErrorAndContinues(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":7,"result":{"ok":true}}`))
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	input := "not-json\n" + `{"jsonrpc":"2.0","id":7,"method":"ping"}` + "\n"
	if err := Run(context.Background(), cfg, strings.NewReader(input), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout lines = %#v", lines)
	}
	assertJSONRPCError(t, lines[0], -32700, nil)
	assertJSONEqual(t, lines[1], `{"jsonrpc":"2.0","id":7,"result":{"ok":true}}`)
}

func TestRunOversizedFrameReturnsErrorAndDoesNotForward(t *testing.T) {
	var called bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	cfg.MaxFrameSize = 32
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":5,"method":"tool","params":{"text":"too-large"}}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if called {
		t.Fatal("oversized frame was forwarded")
	}
	assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, nil)
}

func TestRunLargeFrameWithinLimitIsForwarded(t *testing.T) {
	largeContent := strings.Repeat("a", 2048)
	input := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"upload_asset","arguments":{"contentBase64":"` + largeContent + `"}}}`
	var gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":5,"result":{"ok":true}}`))
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	cfg.MaxFrameSize = int64(len(input) + 1)
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), cfg, strings.NewReader(input+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotBody != input {
		t.Fatalf("forwarded body mismatch")
	}
	assertJSONEqual(t, strings.TrimSpace(stdout.String()), `{"jsonrpc":"2.0","id":5,"result":{"ok":true}}`)
}

func TestRunHTTPErrorForRequestReturnsJSONRPCError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope lwk_secret_token", http.StatusUnauthorized)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	cfg.APIKey = "lwk_secret_token"
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"initialize"}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, float64(9))
	if !strings.Contains(stdout.String(), `"status":401`) {
		t.Fatalf("stdout missing status data: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "lwk_secret_token") || strings.Contains(stderr.String(), "lwk_secret_token") {
		t.Fatal("api key leaked")
	}
}

func TestRunHTTPForbiddenAndServerErrorForRequestsReturnJSONRPCErrors(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(status), status)
			}))
			defer upstream.Close()

			cfg := DefaultConfig()
			cfg.Endpoint = upstream.URL
			var stdout, stderr bytes.Buffer
			err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"initialize"}`+"\n"), &stdout, &stderr)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, float64(9))
			if !strings.Contains(stdout.String(), fmt.Sprintf(`"status":%d`, status)) {
				t.Fatalf("stdout missing status %d: %s", status, stdout.String())
			}
			if !strings.Contains(stderr.String(), fmt.Sprintf("%d", status)) {
				t.Fatalf("stderr = %q, want status %d", stderr.String(), status)
			}
		})
	}
}

func TestRunHTTPErrorForNotificationLogsOnly(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "403") {
		t.Fatalf("stderr = %q, want bounded 403 diagnostic", stderr.String())
	}
}

func TestRunInvalidIDBearingNonResponseFrameReturnsHTTPError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":41}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, float64(41))
	if !strings.Contains(stdout.String(), `"status":502`) {
		t.Fatalf("stdout missing status data: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "502") {
		t.Fatalf("stderr = %q, want status diagnostic", stderr.String())
	}
}

func TestRunUnreachableUpstreamReturnsJSONRPCError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	endpoint := "http://" + listener.Addr().String() + "/mcp"
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Endpoint = endpoint
	cfg.RequestTimeout = 100 * time.Millisecond
	var stdout, stderr bytes.Buffer
	err = Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, float64(1))
	if !strings.Contains(stderr.String(), "upstream") {
		t.Fatalf("stderr = %q, want upstream diagnostic", stderr.String())
	}
}

func TestRunRejectsSSEResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message\n"))
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"initialize"}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, float64(2))
	if !strings.Contains(stderr.String(), "SSE") {
		t.Fatalf("stderr = %q, want SSE diagnostic", stderr.String())
	}
}

func TestRunRejectsNonClosingSSEResponseWithoutWaitingForRequestTimeout(t *testing.T) {
	bodyClosed := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
		close(bodyClosed)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	cfg.RequestTimeout = 2 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, cfg, strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"initialize"}`+"\n"), &stdout, &stderr)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("run did not stop after cancellation")
		}
		t.Fatal("SSE response was not rejected before reading a non-closing body")
	}
	assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, float64(2))
	if !strings.Contains(stderr.String(), "SSE") {
		t.Fatalf("stderr = %q, want SSE diagnostic", stderr.String())
	}
	select {
	case <-bodyClosed:
	case <-time.After(time.Second):
		t.Fatal("SSE response body was not closed")
	}
}

func TestRunRejectsUnsupportedContentType(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain"))
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"initialize"}`+"\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertJSONRPCError(t, strings.TrimSpace(stdout.String()), -32000, float64(3))
	if !strings.Contains(stderr.String(), "text/plain") {
		t.Fatalf("stderr = %q, want content type", stderr.String())
	}
}

func TestRunSessionExpired404UsesClearDiagnostic(t *testing.T) {
	var count int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "session-123")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	if err := Run(context.Background(), cfg, strings.NewReader(input), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout lines = %#v", lines)
	}
	assertJSONRPCError(t, lines[1], -32000, float64(2))
	if !strings.Contains(stderr.String(), "missing or expired") {
		t.Fatalf("stderr = %q, want missing/expired diagnostic", stderr.String())
	}
}

func TestRunBatchRejectionDoesNotPreventLaterRequest(t *testing.T) {
	var count int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"batch unsupported"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`))
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	input := `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	if err := Run(context.Background(), cfg, strings.NewReader(input), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout lines = %#v", lines)
	}
	assertJSONEqual(t, lines[0], `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"batch unsupported"}}`)
	assertJSONEqual(t, lines[1], `{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`)
}

func TestRunShutdownSendsDeleteWhenSessionExists(t *testing.T) {
	var deleteSeen bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "session-123")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`))
		case http.MethodDelete:
			deleteSeen = true
			if r.Header.Get("Mcp-Session-Id") != "session-123" {
				t.Errorf("delete session header = %q", r.Header.Get("Mcp-Session-Id"))
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !deleteSeen {
		t.Fatal("shutdown DELETE was not sent")
	}
}

func TestRunShutdownTimeoutIsIndependentFromRequestTimeout(t *testing.T) {
	deleteStarted := make(chan struct{}, 1)
	deleteCompleted := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "session-123")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`))
		case http.MethodDelete:
			deleteStarted <- struct{}{}
			time.Sleep(150 * time.Millisecond)
			deleteCompleted <- struct{}{}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	cfg.RequestTimeout = 25 * time.Millisecond
	cfg.ShutdownTimeout = time.Second
	var stdout, stderr bytes.Buffer
	start := time.Now()
	if err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Fatalf("run returned after %v; shutdown DELETE did not get shutdown timeout budget", elapsed)
	}
	select {
	case <-deleteStarted:
	default:
		t.Fatal("shutdown DELETE was not sent")
	}
	select {
	case <-deleteCompleted:
	default:
		t.Fatal("shutdown DELETE did not complete")
	}
	if strings.Contains(stderr.String(), "shutdown delete failed") {
		t.Fatalf("stderr = %q, want no shutdown delete failure", stderr.String())
	}
}

func TestRunShutdownWithoutSessionSkipsDelete(t *testing.T) {
	var deleteSeen bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteSeen = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if deleteSeen {
		t.Fatal("delete should be skipped without session")
	}
}

func TestRunDeleteFailureLogsOnlyToStderr(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "session-123")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`))
		case http.MethodDelete:
			http.Error(w, "cleanup failed", http.StatusInternalServerError)
		}
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.Endpoint = upstream.URL
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), cfg, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout polluted by cleanup: %#v", lines)
	}
	if !strings.Contains(stderr.String(), "cleanup") {
		t.Fatalf("stderr = %q, want cleanup warning", stderr.String())
	}
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var gotJSON any
	var wantJSON any
	if err := json.Unmarshal([]byte(got), &gotJSON); err != nil {
		t.Fatalf("got is not json: %v\n%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
		t.Fatalf("want is not json: %v\n%s", err, want)
	}
	if !jsonEqual(gotJSON, wantJSON) {
		t.Fatalf("json mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

func assertJSONRPCError(t *testing.T, got string, code float64, id any) {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("response is not json: %v\n%s", err, got)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Fatalf("jsonrpc = %#v", resp["jsonrpc"])
	}
	if id == nil {
		if resp["id"] != nil {
			t.Fatalf("id = %#v, want nil", resp["id"])
		}
	} else if resp["id"] != id {
		t.Fatalf("id = %#v, want %#v", resp["id"], id)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object: %#v", resp)
	}
	if errObj["code"] != code {
		t.Fatalf("error code = %#v, want %#v", errObj["code"], code)
	}
}

func jsonEqual(a, b any) bool {
	return jsonCanonical(a) == jsonCanonical(b)
}

func jsonCanonical(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
