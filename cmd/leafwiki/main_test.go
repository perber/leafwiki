package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	httpmetrics "github.com/perber/wiki/internal/http/metrics"
)

func TestWriteUsage_UsesLongFlags(t *testing.T) {
	var buf bytes.Buffer

	writeUsage(&buf)

	output := buf.String()
	for _, expected := range []string{
		"--jwt-secret",
		"--admin-password",
		"--admin-username",
		"--admin-email",
		"--allow-insecure",
		"--enable-metrics",
		"--metrics-host",
		"--metrics-port",
		"--data-dir",
		"--unix-socket",
		"--log-format",
		"LEAFWIKI_UNIX_SOCKET",
		"LEAFWIKI_LOG_FORMAT",
		"LEAFWIKI_ADMIN_USERNAME",
		"LEAFWIKI_ADMIN_EMAIL",
		"LEAFWIKI_ENABLE_METRICS",
		"LEAFWIKI_METRICS_HOST",
		"LEAFWIKI_METRICS_PORT",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected usage output to contain %q, got %q", expected, output)
		}
	}
}

func TestRegisterFlags_AcceptsSingleDashLongFlags(t *testing.T) {
	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)

	err := fs.Parse([]string{
		"-jwt-secret=test-secret",
		"-admin-password=test-password",
		"-admin-username=test-admin",
		"-admin-email=test-admin@example.com",
		"-allow-insecure=true",
		"-enable-metrics=true",
		"-metrics-host=127.0.0.2",
		"-metrics-port=9100",
		"-unix-socket=/tmp/leafwiki.sock",
	})
	if err != nil {
		t.Fatalf("expected single-dash long flags to parse, got %v (%s)", err, errOut.String())
	}

	if got := *flags.jwtSecret; got != "test-secret" {
		t.Fatalf("expected jwt secret %q, got %q", "test-secret", got)
	}
	if got := *flags.adminPassword; got != "test-password" {
		t.Fatalf("expected admin password %q, got %q", "test-password", got)
	}
	if got := *flags.adminUsername; got != "test-admin" {
		t.Fatalf("expected admin username %q, got %q", "test-admin", got)
	}
	if got := *flags.adminEmail; got != "test-admin@example.com" {
		t.Fatalf("expected admin email %q, got %q", "test-admin@example.com", got)
	}
	if !*flags.allowInsecure {
		t.Fatalf("expected allow-insecure to be true")
	}
	if !*flags.enableMetrics {
		t.Fatalf("expected enable-metrics to be true")
	}
	if got := *flags.metricsHost; got != "127.0.0.2" {
		t.Fatalf("expected metrics host %q, got %q", "127.0.0.2", got)
	}
	if got := *flags.metricsPort; got != "9100" {
		t.Fatalf("expected metrics port %q, got %q", "9100", got)
	}
	if got := *flags.unixSocket; got != "/tmp/leafwiki.sock" {
		t.Fatalf("expected unix socket %q, got %q", "/tmp/leafwiki.sock", got)
	}
}

func TestValidateHTTPRemoteUserConfig(t *testing.T) {
	tests := []struct {
		name            string
		enabled         bool
		trustedProxyIPs string
		wantErr         bool
	}{
		{"disabled, no IPs", false, "", false},
		{"disabled, with IPs", false, "127.0.0.1", false},
		{"enabled, with IPs", true, "127.0.0.1", false},
		{"enabled, multiple IPs", true, "127.0.0.1,172.18.0.0/16", false},
		{"enabled, no IPs", true, "", true},
		{"enabled, whitespace only", true, "   ", true},
		{"enabled, commas only", true, ",,,", true},
		{"enabled, commas and whitespace", true, " , , ", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHTTPRemoteUserConfig(tc.enabled, tc.trustedProxyIPs)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateHTTPRemoteUserConfig(%v, %q) error = %v, wantErr %v", tc.enabled, tc.trustedProxyIPs, err, tc.wantErr)
			}
		})
	}
}

func TestValidateHTTPRemoteUserAutoCreateConfig(t *testing.T) {
	tests := []struct {
		name              string
		autoCreateEnabled bool
		remoteUserEnabled bool
		defaultRole       string
		wantErr           bool
	}{
		{"auto-create disabled, everything else irrelevant", false, false, "", false},
		{"auto-create enabled, remote-user disabled", true, false, "viewer", true},
		{"auto-create enabled, remote-user enabled, valid role", true, true, "viewer", false},
		{"auto-create enabled, remote-user enabled, editor role", true, true, "editor", false},
		{"auto-create enabled, remote-user enabled, admin role forbidden", true, true, "admin", true},
		{"auto-create enabled, remote-user enabled, invalid role", true, true, "superuser", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHTTPRemoteUserAutoCreateConfig(tc.autoCreateEnabled, tc.remoteUserEnabled, tc.defaultRole)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateHTTPRemoteUserAutoCreateConfig(%v, %v, %q) error = %v, wantErr %v", tc.autoCreateEnabled, tc.remoteUserEnabled, tc.defaultRole, err, tc.wantErr)
			}
		})
	}
}

func TestValidateRedirectURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty", "", false},
		{"http", "http://idp.example.com/login", false},
		{"https", "https://idp.example.com/login", false},
		{"uppercase scheme", "HTTPS://idp.example.com/login", false},
		{"mixed-case scheme", "Https://idp.example.com/login", false},
		{"javascript scheme", "javascript:alert(1)", true},
		{"relative path", "/login", true},
		{"data scheme", "data:text/html,<script>alert(1)</script>", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRedirectURL("login-url", tc.url)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateRedirectURL(%q) error = %v, wantErr %v", tc.url, err, tc.wantErr)
			}
		})
	}
}

// TestValidateRedirectURL_UserManagementURL confirms --user-management-url is
// validated the same way as --login-url/--logout-url (http(s) only, no relative
// paths or dangerous schemes) — it's rendered as a plain <a href>, but an
// unsafe scheme there is still attacker-controlled markup.
func TestValidateRedirectURL_UserManagementURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty", "", false},
		{"https", "https://idp.example.com/users", false},
		{"javascript scheme", "javascript:alert(1)", true},
		{"relative path", "/users", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRedirectURL("user-management-url", tc.url)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateRedirectURL(%q) error = %v, wantErr %v", tc.url, err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "user-management-url") {
				t.Fatalf("validateRedirectURL(%q) error = %v, want it to mention the flag name", tc.url, err)
			}
		})
	}
}

func TestResolveLogoutURL(t *testing.T) {
	tests := []struct {
		name               string
		logoutURL          string
		deprecatedFlagVal  string
		visited            map[string]bool
		wantResolved       string
		wantUsedDeprecated bool
	}{
		{
			name:         "new flag set, deprecated ignored",
			logoutURL:    "https://idp.example.com/logout",
			wantResolved: "https://idp.example.com/logout",
		},
		{
			name:               "only deprecated flag set",
			deprecatedFlagVal:  "https://idp.example.com/logout",
			visited:            map[string]bool{"http-remote-user-logout-url": true},
			wantResolved:       "https://idp.example.com/logout",
			wantUsedDeprecated: true,
		},
		{
			name:         "neither set",
			wantResolved: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			visited := tc.visited
			if visited == nil {
				visited = map[string]bool{}
			}
			resolved, usedDeprecated := resolveLogoutURL(tc.logoutURL, tc.deprecatedFlagVal, visited, "LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL")
			if resolved != tc.wantResolved {
				t.Fatalf("resolveLogoutURL() resolved = %q, want %q", resolved, tc.wantResolved)
			}
			if usedDeprecated != tc.wantUsedDeprecated {
				t.Fatalf("resolveLogoutURL() usedDeprecated = %v, want %v", usedDeprecated, tc.wantUsedDeprecated)
			}
		})
	}
}

func TestResolveString_TrimsCLIFlagValue(t *testing.T) {
	visited := map[string]bool{"login-url": true}
	got := resolveString("login-url", " https://idp.example.com/login ", visited, "LEAFWIKI_LOGIN_URL", "")
	if want := "https://idp.example.com/login"; got != want {
		t.Fatalf("resolveString() = %q, want %q", got, want)
	}
}

func TestResolveLogFormat_Precedence(t *testing.T) {
	tests := []struct {
		name     string
		flagVal  string
		visited  bool
		envVal   string
		wantForm string
	}{
		{
			name:     "neither set falls back to default",
			wantForm: "text",
		},
		{
			name:     "env var sets json",
			envVal:   "json",
			wantForm: "json",
		},
		{
			name:     "env var is case-insensitive",
			envVal:   "JSON",
			wantForm: "json",
		},
		{
			name:     "cli flag overrides env var",
			flagVal:  "text",
			visited:  true,
			envVal:   "json",
			wantForm: "text",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LEAFWIKI_LOG_FORMAT", tc.envVal)
			visited := map[string]bool{}
			if tc.visited {
				visited["log-format"] = true
			}
			got := resolveLogFormat("log-format", tc.flagVal, visited, "LEAFWIKI_LOG_FORMAT", "text")
			if got != tc.wantForm {
				t.Fatalf("resolveLogFormat() = %q, want %q", got, tc.wantForm)
			}
		})
	}
}

func TestSetupLogger_SelectsHandlerByFormat(t *testing.T) {
	t.Run("text format writes non-JSON output", func(t *testing.T) {
		var buf bytes.Buffer
		setupLogger(&buf, "text")
		slog.Default().Info("hello")

		if json.Valid(buf.Bytes()) {
			t.Fatalf("expected non-JSON text output, got %q", buf.String())
		}
		if !strings.Contains(buf.String(), "msg=hello") {
			t.Fatalf("expected text output to contain msg=hello, got %q", buf.String())
		}
	})

	t.Run("json format writes valid JSON output", func(t *testing.T) {
		var buf bytes.Buffer
		setupLogger(&buf, "json")
		slog.Default().Info("hello")

		if !json.Valid(buf.Bytes()) {
			t.Fatalf("expected valid JSON output, got %q", buf.String())
		}
	})
}

func TestRunRestoreSnapshotCommand_MissingArg_ReturnsUsageError(t *testing.T) {
	err := runRestoreSnapshotCommand(t.TempDir(), []string{"restore-snapshot"})
	if !errors.Is(err, errRestoreSnapshotUsage) {
		t.Fatalf("runRestoreSnapshotCommand() error = %v, want errRestoreSnapshotUsage", err)
	}
}

func TestRunRestoreSnapshotCommand_InvalidZipPath_PropagatesError(t *testing.T) {
	dataDir := t.TempDir()
	zipPath := filepath.Join(dataDir, "does-not-exist.zip")

	err := runRestoreSnapshotCommand(dataDir, []string{"restore-snapshot", zipPath})
	if err == nil {
		t.Fatal("runRestoreSnapshotCommand() expected an error for a non-existent snapshot zip, got nil")
	}
	if errors.Is(err, errRestoreSnapshotUsage) {
		t.Fatalf("runRestoreSnapshotCommand() error = %v, want a restore error, not the usage error", err)
	}
}

func TestValidateGitBackupRemote(t *testing.T) {
	const (
		sshKey   = "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n"
		keyPath  = "/etc/leafwiki/id_ed25519"
		user     = "jochumdev"
		password = "github_pat_secret"
	)
	tests := []struct {
		name         string
		remote       string
		sshKey       string
		sshKeyPath   string
		httpUsername string
		httpPassword string
		wantErr      bool
	}{
		{name: "no remote is local-only and needs no credentials"},

		{name: "ssh remote with inline key", remote: "git@github.com:user/repo.git", sshKey: sshKey},
		{name: "ssh remote with key path", remote: "git@github.com:user/repo.git", sshKeyPath: keyPath},
		{name: "ssh url with key", remote: "ssh://git@github.com/user/repo.git", sshKey: sshKey},
		{name: "ssh remote without key", remote: "git@github.com:user/repo.git", wantErr: true},
		{name: "ssh remote with only http credentials", remote: "git@github.com:user/repo.git", httpUsername: user, httpPassword: password, wantErr: true},

		{name: "https remote with username and password", remote: "https://github.com/user/repo.git", httpUsername: user, httpPassword: password},
		{name: "http remote with username and password", remote: "http://gitea.internal/user/repo.git", httpUsername: user, httpPassword: password},
		{name: "uppercase https scheme", remote: "HTTPS://github.com/user/repo.git", httpUsername: user, httpPassword: password},
		{name: "https remote with credentials embedded in the URL", remote: "https://jochumdev:github_pat_secret@github.com/user/repo.git"},
		{name: "https remote without any credentials", remote: "https://github.com/user/repo.git", wantErr: true},
		{name: "https remote with username only", remote: "https://github.com/user/repo.git", httpUsername: user, wantErr: true},
		{name: "https remote with password only", remote: "https://github.com/user/repo.git", httpPassword: password, wantErr: true},
		// An SSH key is not a substitute for HTTP credentials.
		{name: "https remote with only an ssh key", remote: "https://github.com/user/repo.git", sshKey: sshKey, wantErr: true},

		{name: "file remote is not supported", remote: "file:///srv/backup.git", sshKey: sshKey, wantErr: true},
		{name: "bare path is not supported", remote: "/srv/backup.git", sshKey: sshKey, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGitBackupRemote(tc.remote, tc.sshKey, tc.sshKeyPath, tc.httpUsername, tc.httpPassword)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateGitBackupRemote(%q, ...) error = %v, wantErr %v", tc.remote, err, tc.wantErr)
			}
		})
	}
}

func TestValidateListenConfig(t *testing.T) {
	tests := []struct {
		name       string
		unixSocket string
		visited    map[string]bool
		wantErr    bool
	}{
		{
			name:       "tcp only is allowed",
			unixSocket: "",
			visited:    map[string]bool{"host": true, "port": true},
			wantErr:    false,
		},
		{
			name:       "unix socket only is allowed",
			unixSocket: "/tmp/leafwiki.sock",
			visited:    map[string]bool{},
			wantErr:    false,
		},
		{
			name:       "unix socket with host is rejected",
			unixSocket: "/tmp/leafwiki.sock",
			visited:    map[string]bool{"host": true},
			wantErr:    true,
		},
		{
			name:       "unix socket with port is rejected",
			unixSocket: "/tmp/leafwiki.sock",
			visited:    map[string]bool{"port": true},
			wantErr:    true,
		},
		{
			name:       "unix socket with host and port is rejected",
			unixSocket: "/tmp/leafwiki.sock",
			visited:    map[string]bool{"host": true, "port": true},
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateListenConfig(tc.unixSocket, tc.visited)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateListenConfig(%q, %v) error = %v, wantErr %v", tc.unixSocket, tc.visited, err, tc.wantErr)
			}
		})
	}
}

func TestRegisterFlags_AcceptsDoubleDashLongFlags(t *testing.T) {
	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)

	err := fs.Parse([]string{
		"--jwt-secret=test-secret",
		"--admin-password=test-password",
		"--admin-username=test-admin",
		"--admin-email=test-admin@example.com",
		"--allow-insecure=true",
		"--enable-metrics=true",
		"--metrics-host=127.0.0.2",
		"--metrics-port=9100",
		"--unix-socket=/tmp/leafwiki.sock",
	})
	if err != nil {
		t.Fatalf("expected double-dash long flags to parse, got %v (%s)", err, errOut.String())
	}

	if got := *flags.jwtSecret; got != "test-secret" {
		t.Fatalf("expected jwt secret %q, got %q", "test-secret", got)
	}
	if got := *flags.adminPassword; got != "test-password" {
		t.Fatalf("expected admin password %q, got %q", "test-password", got)
	}
	if got := *flags.adminUsername; got != "test-admin" {
		t.Fatalf("expected admin username %q, got %q", "test-admin", got)
	}
	if got := *flags.adminEmail; got != "test-admin@example.com" {
		t.Fatalf("expected admin email %q, got %q", "test-admin@example.com", got)
	}
	if !*flags.allowInsecure {
		t.Fatalf("expected allow-insecure to be true")
	}
	if !*flags.enableMetrics {
		t.Fatalf("expected enable-metrics to be true")
	}
	if got := *flags.metricsHost; got != "127.0.0.2" {
		t.Fatalf("expected metrics host %q, got %q", "127.0.0.2", got)
	}
	if got := *flags.metricsPort; got != "9100" {
		t.Fatalf("expected metrics port %q, got %q", "9100", got)
	}
	if got := *flags.unixSocket; got != "/tmp/leafwiki.sock" {
		t.Fatalf("expected unix socket %q, got %q", "/tmp/leafwiki.sock", got)
	}
}

func TestStartMetricsServer_ServesOnlyMetricsEndpoint(t *testing.T) {
	metrics := httpmetrics.NewHTTPMetrics("test")
	stopServer, addr, err := startMetricsServer(metrics, "127.0.0.1", "0")
	if err != nil {
		t.Fatalf("startMetricsServer() error = %v", err)
	}
	defer stopServer()

	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /metrics response failed: %v", err)
	}
	if !strings.Contains(string(body), "leafwiki_http_requests_in_flight") {
		t.Fatalf("expected metrics output, got %q", string(body))
	}

	notFoundResp, err := http.Get("http://" + addr + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer func() {
		_ = notFoundResp.Body.Close()
	}()

	if notFoundResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 from non-metrics path, got %d", notFoundResp.StatusCode)
	}
}

func TestRemoveStaleUnixSocket_RemovesExistingSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "leafwiki.sock")
	if len(socketPath) >= 100 {
		t.Skipf("socket path too long (%d chars) for this platform", len(socketPath))
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	if err := removeStaleUnixSocket(socketPath); err != nil {
		t.Fatalf("removeStaleUnixSocket() error = %v", err)
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("expected socket to be removed, stat err = %v", err)
	}
}

func TestRemoveStaleUnixSocket_RejectsNonSocketPath(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "leafwiki.sock")
	if err := os.WriteFile(socketPath, []byte("not a socket"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	err := removeStaleUnixSocket(socketPath)
	if err == nil {
		t.Fatalf("expected error for non-socket path")
	}
	if !strings.Contains(err.Error(), "is not a socket") {
		t.Fatalf("expected non-socket error, got %v", err)
	}
}

func TestListenOnUnixSocket_CreatesSocketWithExpectedPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "leafwiki.sock")
	if len(socketPath) >= 100 {
		t.Skipf("socket path too long (%d chars) for this platform", len(socketPath))
	}
	listener, err := listenOnUnixSocket(socketPath)
	if err != nil {
		t.Fatalf("listenOnUnixSocket() error = %v", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("expected %s to be a socket, mode = %v", socketPath, info.Mode())
	}
	if got := info.Mode().Perm(); got != 0660 {
		t.Fatalf("expected socket permissions 0660, got %#o", got)
	}
}

func TestListenOnUnixSocket_WindowsReturnsHelpfulError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific behavior")
	}
	_, err := listenOnUnixSocket(`C:\leafwiki.sock`)
	if err == nil {
		t.Fatalf("expected error on windows")
	}
	if !strings.Contains(err.Error(), "not supported on windows") {
		t.Fatalf("expected windows support error, got %v", err)
	}
}

type testSignal string

func (s testSignal) String() string { return string(s) }
func (testSignal) Signal()          {}

func TestServeWithLifecycle_GracefulShutdownWaitsForInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	reloadSignals := make(chan os.Signal)
	shutdownSignals := make(chan os.Signal, 1)

	runErr := make(chan error, 1)
	go func() {
		runErr <- serveWithLifecycle(server, listener, nil, func() {}, reloadSignals, shutdownSignals)
	}()

	respCh := make(chan *http.Response, 1)
	reqErrCh := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			reqErrCh <- err
			return
		}
		respCh <- resp
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach handler")
	}

	shutdownSignals <- testSignal("shutdown")

	select {
	case err := <-runErr:
		t.Fatalf("server exited before request completed: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(release)

	var resp *http.Response
	select {
	case err := <-reqErrCh:
		t.Fatalf("request failed: %v", err)
	case resp = <-respCh:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not complete")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("closing response body failed: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Fatalf("expected body ok, got %q", string(body))
	}

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("expected clean shutdown, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit after request completed")
	}
}

func TestServeWithLifecycle_ReloadSignalTriggersCallbackWithoutStoppingServer(t *testing.T) {
	var reloadCalls atomic.Int32

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	reloadDone := make(chan struct{}, 1)
	reloadSignals := make(chan os.Signal, 1)
	shutdownSignals := make(chan os.Signal, 1)

	runErr := make(chan error, 1)
	go func() {
		runErr <- serveWithLifecycle(server, listener, nil, func() {
			reloadCalls.Add(1)
			reloadDone <- struct{}{}
		}, reloadSignals, shutdownSignals)
	}()

	reloadSignals <- testSignal("reload")

	select {
	case <-reloadDone:
	case <-time.After(2 * time.Second):
		t.Fatal("reload callback was not triggered")
	}

	resp, err := http.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("request after reload failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("closing response body failed: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", resp.StatusCode)
	}

	if reloadCalls.Load() != 1 {
		t.Fatalf("expected one reload call, got %d", reloadCalls.Load())
	}

	shutdownSignals <- testSignal("shutdown")

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("expected clean shutdown, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after shutdown signal")
	}
}

func TestServeWithLifecycle_ShutdownTimeoutStillRunsCleanup(t *testing.T) {
	previousTimeout := gracefulShutdownTimeout
	gracefulShutdownTimeout = 50 * time.Millisecond
	t.Cleanup(func() {
		gracefulShutdownTimeout = previousTimeout
	})

	started := make(chan struct{})
	handlerCanceled := make(chan struct{}, 1)
	cleanupCalled := make(chan struct{}, 1)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
			handlerCanceled <- struct{}{}
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	reloadSignals := make(chan os.Signal)
	shutdownSignals := make(chan os.Signal, 1)
	runErr := make(chan error, 1)
	go func() {
		runErr <- serveWithLifecycle(server, listener, func() {
			select {
			case cleanupCalled <- struct{}{}:
			default:
			}
		}, func() {}, reloadSignals, shutdownSignals)
	}()

	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if err == nil && resp != nil {
			_ = resp.Body.Close()
		}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach handler")
	}

	shutdownSignals <- testSignal("shutdown")

	select {
	case err := <-runErr:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context deadline exceeded, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit after shutdown timeout")
	}

	select {
	case <-cleanupCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup was not called on shutdown timeout")
	}

	select {
	case <-handlerCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not canceled after shutdown timeout")
	}
}

func TestServeWithLifecycle_ShutdownDoesNotWaitForInFlightReload(t *testing.T) {
	reloadStarted := make(chan struct{})
	releaseReload := make(chan struct{})
	reloadFinished := make(chan struct{})

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	reloadSignals := make(chan os.Signal, 1)
	shutdownSignals := make(chan os.Signal, 1)
	runErr := make(chan error, 1)
	go func() {
		runErr <- serveWithLifecycle(server, listener, nil, func() {
			close(reloadStarted)
			go func() {
				<-releaseReload
				close(reloadFinished)
			}()
		}, reloadSignals, shutdownSignals)
	}()

	reloadSignals <- testSignal("reload")

	select {
	case <-reloadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("reload did not start")
	}

	shutdownSignals <- testSignal("shutdown")

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("expected clean shutdown, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop while reload was still running")
	}

	close(releaseReload)

	select {
	case <-reloadFinished:
	case <-time.After(2 * time.Second):
		t.Fatal("reload did not finish after release")
	}
}

// goBuildLdflagsLine extracts the `go build ... -ldflags="..."` line from a
// Dockerfile so tests can assert on exactly what gets baked into the binary.
func goBuildLdflagsLine(t *testing.T, dockerfilePath string) string {
	t.Helper()

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", dockerfilePath, err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, "-ldflags=") {
			return line
		}
	}

	t.Fatalf("no -ldflags line found in %s", dockerfilePath)
	return ""
}

// TestDockerfile_GoBuildLdflags_InjectsAppVersion pins the bug where
// Dockerfile declared ARG APP_VERSION and threaded it into the frontend
// build, but never into the Go binary's -ldflags — so every release image
// silently shipped main.Version's "dev" default (e.g. into the
// leafwiki_build_info metric).
func TestDockerfile_GoBuildLdflags_InjectsAppVersion(t *testing.T) {
	line := goBuildLdflagsLine(t, filepath.Join("..", "..", "Dockerfile"))

	if !strings.Contains(line, "-X main.Version=${APP_VERSION}") {
		t.Fatalf("expected Dockerfile go build ldflags to inject main.Version from APP_VERSION, got: %s", line)
	}
}

// TestDockerfileBuilder_GoBuildLdflags_InjectsAppVersion is the same
// regression check for Dockerfile.builder, used by `make release` to
// produce the binaries attached to GitHub Releases.
func TestDockerfileBuilder_GoBuildLdflags_InjectsAppVersion(t *testing.T) {
	line := goBuildLdflagsLine(t, filepath.Join("..", "..", "Dockerfile.builder"))

	if !strings.Contains(line, "-X main.Version=${APP_VERSION}") {
		t.Fatalf("expected Dockerfile.builder go build ldflags to inject main.Version from APP_VERSION, got: %s", line)
	}
}

// dockerBuildStageDeclaresArg checks that the multi-stage Docker build
// stage containing marker (e.g. the `go build` line) is preceded by its own
// `ARG argName` declaration *within that stage*. Docker scopes ARG per
// build stage: an ARG declared in an earlier stage does not carry into a
// later one, even though `${argName}` still substitutes silently as an
// empty string there instead of failing the build.
func dockerBuildStageDeclaresArg(t *testing.T, dockerfilePath, marker, argName string) bool {
	t.Helper()

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", dockerfilePath, err)
	}

	lines := strings.Split(string(content), "\n")

	markerIdx := -1
	for i, line := range lines {
		if strings.Contains(line, marker) {
			markerIdx = i
			break
		}
	}
	if markerIdx == -1 {
		t.Fatalf("no line containing %q found in %s", marker, dockerfilePath)
	}

	stageStart := 0
	for i := markerIdx; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "FROM ") {
			stageStart = i
			break
		}
	}

	for _, line := range lines[stageStart:markerIdx] {
		if strings.TrimSpace(line) == "ARG "+argName {
			return true
		}
	}
	return false
}

// TestDockerfile_GoBuildStage_DeclaresAppVersionArg pins the bug where
// Dockerfile's backend-build stage used ${APP_VERSION} in its go build
// -ldflags without re-declaring `ARG APP_VERSION` in that stage (it was
// only declared in the earlier frontend-build stage). Docker scopes ARG per
// stage, so the ldflags line silently baked in an empty version string
// instead of failing the build — every v0.12.x release image/binary shipped
// main.Version="" (visible in the leafwiki_build_info metric and in the
// snapshot/restore version-mismatch check, which silently no-ops when
// WikiVersion is empty).
func TestDockerfile_GoBuildStage_DeclaresAppVersionArg(t *testing.T) {
	if !dockerBuildStageDeclaresArg(t, filepath.Join("..", "..", "Dockerfile"), "go build", "APP_VERSION") {
		t.Fatal("Dockerfile's go build stage uses ${APP_VERSION} without declaring ARG APP_VERSION in that stage")
	}
}

// TestDockerfileBuilder_GoBuildStage_DeclaresAppVersionArg is the same
// regression check for Dockerfile.builder's builder stage.
func TestDockerfileBuilder_GoBuildStage_DeclaresAppVersionArg(t *testing.T) {
	if !dockerBuildStageDeclaresArg(t, filepath.Join("..", "..", "Dockerfile.builder"), "go build", "APP_VERSION") {
		t.Fatal("Dockerfile.builder's builder stage uses ${APP_VERSION} without declaring ARG APP_VERSION in that stage")
	}
}

// TestResolveVersionScript_AppVersionEnvOverride_ReturnsEnvValue exercises
// the deterministic branch of scripts/resolve-version.sh (the shared
// algorithm used by both `make build`/`make run` and vite.config.ts). The
// git-describe fallback branch is intentionally not tested here since its
// output depends on the local repo's tag state and CI checkout depth.
func TestResolveVersionScript_AppVersionEnvOverride_ReturnsEnvValue(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "scripts", "resolve-version.sh")

	cmd := exec.Command(scriptPath)
	cmd.Env = append(os.Environ(), "APP_VERSION=v9.9.9-test")

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve-version.sh failed: %v", err)
	}

	if got := strings.TrimSpace(string(out)); got != "v9.9.9-test" {
		t.Fatalf("expected resolve-version.sh to echo APP_VERSION override, got: %q", got)
	}
}

// TestMakefile_BuildAndRunTargets_InjectVersionLdflags pins that the local
// `make build`/`make run` targets inject main.Version the same way the
// release/Docker targets do, instead of leaving local builds on the "dev"
// default silently.
func TestMakefile_BuildAndRunTargets_InjectVersionLdflags(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("failed to read Makefile: %v", err)
	}
	content := string(raw)

	for _, target := range []string{"build:", "run:"} {
		idx := strings.Index(content, target)
		if idx == -1 {
			t.Fatalf("Makefile target %q not found", target)
		}

		recipeEnd := strings.Index(content[idx:], "\n\n")
		if recipeEnd == -1 {
			recipeEnd = len(content) - idx
		}
		recipe := content[idx : idx+recipeEnd]

		if !strings.Contains(resolveMakeVars(content, recipe), "-X main.Version=$(VERSION)") {
			t.Fatalf("expected Makefile %q recipe to inject main.Version from $(VERSION), got: %s", target, recipe)
		}
	}
}

var makeVarAssignment = regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\s*(?::=|\?=|=)\s*(.*)$`)

// resolveMakeVars expands $(NAME) references in s using NAME's assignment
// elsewhere in the Makefile, so the check above still works when a recipe
// builds its ldflags from a variable (e.g. $(LDFLAGS)) instead of a literal
// string. $(VERSION) itself is left unresolved since the test asserts on
// that exact reference.
func resolveMakeVars(content, s string) string {
	assignments := map[string]string{}
	for _, m := range makeVarAssignment.FindAllStringSubmatch(content, -1) {
		assignments[m[1]] = m[2]
	}
	delete(assignments, "VERSION")

	for changed := true; changed; {
		changed = false
		for name, value := range assignments {
			token := "$(" + name + ")"
			if strings.Contains(s, token) {
				s = strings.ReplaceAll(s, token, value)
				changed = true
			}
		}
	}
	return s
}

// TestViteConfig_ResolvesVersionViaSharedScript pins that the frontend
// build resolves its version through the same scripts/resolve-version.sh
// used by the Go build, rather than a second, independently-drifting
// git-describe implementation in JS.
func TestViteConfig_ResolvesVersionViaSharedScript(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "ui", "leafwiki-ui", "vite.config.ts"))
	if err != nil {
		t.Fatalf("failed to read vite.config.ts: %v", err)
	}

	if !strings.Contains(string(content), "scripts/resolve-version.sh") {
		t.Fatalf("expected vite.config.ts to resolve its version via scripts/resolve-version.sh, got:\n%s", content)
	}
}
