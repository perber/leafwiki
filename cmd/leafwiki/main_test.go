package main

import (
	"bytes"
	"flag"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteUsage_UsesLongFlags(t *testing.T) {
	var buf bytes.Buffer

	writeUsage(&buf)

	output := buf.String()
	for _, expected := range []string{
		"--jwt-secret",
		"--admin-password",
		"--allow-insecure",
		"--data-dir",
		"--unix-socket",
		"LEAFWIKI_UNIX_SOCKET",
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
		"-allow-insecure=true",
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
	if !*flags.allowInsecure {
		t.Fatalf("expected allow-insecure to be true")
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
		"--allow-insecure=true",
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
	if !*flags.allowInsecure {
		t.Fatalf("expected allow-insecure to be true")
	}
	if got := *flags.unixSocket; got != "/tmp/leafwiki.sock" {
		t.Fatalf("expected unix socket %q, got %q", "/tmp/leafwiki.sock", got)
	}
}

func TestRemoveStaleUnixSocket_RemovesExistingSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "leafwiki.sock")

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
