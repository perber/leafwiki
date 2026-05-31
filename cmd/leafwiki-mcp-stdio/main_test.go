package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunCLIHelpWritesUsageAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(context.Background(), []string{"--help"}, envMap(nil), strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	usage := stdout.String()
	for _, want := range []string{"--endpoint", "--api-key", "--request-timeout", "--shutdown-timeout", "--max-frame-size"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %s:\n%s", want, usage)
		}
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLIUnknownFlagWritesUsageToStderrOnly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(context.Background(), []string{"--bogus"}, envMap(nil), strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") || !strings.Contains(stderr.String(), "--endpoint") {
		t.Fatalf("stderr = %q, want parse error and usage", stderr.String())
	}
}

func TestRunCLIInvalidEndpointFailsBeforeProtocolOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(context.Background(), []string{"--endpoint", "not-a-url"}, envMap(nil), strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid endpoint") {
		t.Fatalf("stderr = %q, want invalid endpoint", stderr.String())
	}
}

func TestRunCLIUsesEnvironmentAndCLIOverrides(t *testing.T) {
	var authHeaders []string
	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("environment endpoint should be overridden by CLI")
	}))
	defer upstreamA.Close()
	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer upstreamB.Close()

	env := envMap(map[string]string{
		"LEAFWIKI_MCP_ENDPOINT":               upstreamA.URL,
		"LEAFWIKI_MCP_API_KEY":                "lwk_env_secret",
		"LEAFWIKI_MCP_STDIO_REQUEST_TIMEOUT":  "5s",
		"LEAFWIKI_MCP_STDIO_SHUTDOWN_TIMEOUT": "1s",
		"LEAFWIKI_MCP_STDIO_MAX_FRAME_SIZE":   "1MiB",
	})
	var stdout, stderr bytes.Buffer
	code := runCLI(
		context.Background(),
		[]string{"--endpoint", upstreamB.URL, "--api-key", "lwk_cli_secret"},
		env,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if len(authHeaders) != 1 || authHeaders[0] != "Bearer lwk_cli_secret" {
		t.Fatalf("auth headers = %#v", authHeaders)
	}
	if strings.Contains(stderr.String(), "lwk_env_secret") || strings.Contains(stderr.String(), "lwk_cli_secret") {
		t.Fatalf("stderr leaked key: %s", stderr.String())
	}
}

func TestRunCLIAcceptsAPIKeyFromEnvironment(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer upstream.Close()

	env := envMap(map[string]string{
		"LEAFWIKI_MCP_ENDPOINT": upstream.URL,
		"LEAFWIKI_MCP_API_KEY":  "lwk_env_secret",
	})
	var stdout, stderr bytes.Buffer
	code := runCLI(context.Background(), nil, env, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if gotAuth != "Bearer lwk_env_secret" {
		t.Fatalf("auth = %q", gotAuth)
	}
}

func TestRunCLIInvalidDurationFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(context.Background(), []string{"--request-timeout", "bogus"}, envMap(nil), strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "request-timeout") {
		t.Fatalf("stderr = %q, want request-timeout", stderr.String())
	}
}

func TestRunCLISignalContextStopsAndCleansUp(t *testing.T) {
	deleteSeen := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "session-123")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`))
		case http.MethodDelete:
			deleteSeen <- struct{}{}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer upstream.Close()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	stdout := newSignalAwareWriter()
	var stderr bytes.Buffer
	go func() {
		done <- runCLI(ctx, []string{"--endpoint", upstream.URL}, envMap(nil), reader, &stdout, &stderr)
	}()
	if _, err := writer.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case <-stdout.wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("sidecar did not write initialize response")
	}
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runCLI did not stop after context cancellation")
	}
	select {
	case <-deleteSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown delete was not sent")
	}
}

func envMap(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

type signalAwareWriter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	once  sync.Once
	wrote chan struct{}
}

func newSignalAwareWriter() signalAwareWriter {
	return signalAwareWriter{wrote: make(chan struct{})}
}

func (w *signalAwareWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if n > 0 {
		w.once.Do(func() { close(w.wrote) })
	}
	return n, err
}

func (w *signalAwareWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}
