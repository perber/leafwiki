package main

import (
	"bytes"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/wiki"
)

func TestWriteUsage_UsesLongFlags(t *testing.T) {
	var buf bytes.Buffer

	writeUsage(&buf)

	output := buf.String()
	if !strings.Contains(output, "leafwiki --jwt-secret <SECRET> --admin-password <PASSWORD> [--host <HOST>] [--port <PORT>] [--data-dir <DIR>] [--root-dir <DIR>]") {
		t.Fatalf("expected authenticated startup usage to include --root-dir, got %q", output)
	}
	for _, expected := range []string{
		"--jwt-secret",
		"--admin-password",
		"--allow-insecure",
		"--data-dir",
		"--root-dir",
		"--enable-mcp",
		"LEAFWIKI_ROOT_DIR",
		"LEAFWIKI_ENABLE_MCP",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected usage output to contain %q, got %q", expected, output)
		}
	}
}

func TestResolveWorkspace_DefaultsRootDirUnderDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")

	workspace := resolveWorkspaceForArgs(t, []string{"--data-dir=" + dataDir})

	if workspace.DataDir != dataDir {
		t.Fatalf("DataDir = %q, want %q", workspace.DataDir, dataDir)
	}
	if got, want := workspace.RootDir, filepath.Join(dataDir, "root"); got != want {
		t.Fatalf("RootDir = %q, want %q", got, want)
	}
}

func TestResolveWorkspace_EnvRootDirOverridesDefault(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	rootDir := filepath.Join(t.TempDir(), "content")
	t.Setenv("LEAFWIKI_ROOT_DIR", rootDir)

	workspace := resolveWorkspaceForArgs(t, []string{"--data-dir=" + dataDir})

	if workspace.RootDir != rootDir {
		t.Fatalf("RootDir = %q, want env root %q", workspace.RootDir, rootDir)
	}
}

func TestResolveWorkspace_CLIRootDirOverridesEnv(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	envRootDir := filepath.Join(t.TempDir(), "env-content")
	cliRootDir := filepath.Join(t.TempDir(), "cli-content")
	t.Setenv("LEAFWIKI_ROOT_DIR", envRootDir)

	workspace := resolveWorkspaceForArgs(t, []string{
		"--data-dir=" + dataDir,
		"--root-dir=" + cliRootDir,
	})

	if workspace.RootDir != cliRootDir {
		t.Fatalf("RootDir = %q, want CLI root %q", workspace.RootDir, cliRootDir)
	}
}

func TestResolveWorkspace_NormalizesPaths(t *testing.T) {
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "data")
	rootDir := filepath.Join(baseDir, "content")

	workspace := resolveWorkspaceForArgs(t, []string{
		"--data-dir= " + dataDir + string(filepath.Separator) + ". ",
		"--root-dir= " + rootDir + string(filepath.Separator) + ". ",
	})

	if workspace.DataDir != dataDir {
		t.Fatalf("DataDir = %q, want normalized %q", workspace.DataDir, dataDir)
	}
	if workspace.RootDir != rootDir {
		t.Fatalf("RootDir = %q, want normalized %q", workspace.RootDir, rootDir)
	}
}

func TestValidateWorkspaceRejectsSameDataAndRootDir(t *testing.T) {
	dir := t.TempDir()

	err := validateWorkspaceDirs(dir, filepath.Clean(filepath.Join(dir, ".")))
	if err == nil {
		t.Fatalf("expected RootDir == DataDir to be rejected")
	}
	if !strings.Contains(err.Error(), "root dir must be different from data dir") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateWorkspaceRejectsRootDirContainingDataDir(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "wiki")
	dataDir := filepath.Join(rootDir, "data")

	err := validateWorkspaceDirs(dataDir, rootDir)
	if err == nil {
		t.Fatalf("expected RootDir containing DataDir to be rejected")
	}
	if !strings.Contains(err.Error(), "root dir must not contain data dir") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestResolveStartupWorkspace_SkipsWorkspaceValidationForResetAdminPassword(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEAFWIKI_ROOT_DIR", dir)

	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)
	if err := fs.Parse([]string{"--data-dir=" + dir, "reset-admin-password"}); err != nil {
		t.Fatalf("parse flags: %v (%s)", err, errOut.String())
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })

	if _, shouldStart, err := resolveStartupWorkspace(flags, visited, fs.Args()); err != nil || shouldStart {
		t.Fatalf("resolveStartupWorkspace reset = shouldStart %v err %v, want no validation and no startup", shouldStart, err)
	}
}

func TestValidateMCPStartupOptions_RequiresLoopbackHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		enableMCP   bool
		disableAuth bool
		remoteUser  bool
		host        string
		wantErr     bool
	}{
		{name: "disabled MCP ignores host/auth", enableMCP: false, disableAuth: false, host: "0.0.0.0"},
		{name: "MCP allows normal auth on loopback", enableMCP: true, disableAuth: false, host: "127.0.0.1"},
		{name: "MCP allows legacy disabled auth on loopback", enableMCP: true, disableAuth: true, host: "127.0.0.1"},
		{name: "MCP rejects wildcard host", enableMCP: true, disableAuth: true, host: "0.0.0.0", wantErr: true},
		{name: "MCP allows remote user middleware on loopback", enableMCP: true, disableAuth: false, remoteUser: true, host: "127.0.0.1"},
		{name: "MCP allows localhost", enableMCP: true, disableAuth: true, host: "localhost"},
		{name: "MCP allows IPv4 loopback", enableMCP: true, disableAuth: true, host: "127.0.0.1"},
		{name: "MCP allows IPv6 loopback", enableMCP: true, disableAuth: true, host: "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLocalMCPOptions(localMCPOptions{
				EnableMCP:        tt.enableMCP,
				DisableAuth:      tt.disableAuth,
				HTTPRemoteUserOn: tt.remoteUser,
				Host:             tt.host,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateLocalMCPOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveLocalMCPOptions_UsesFlagEnvPrecedenceBeforeValidation(t *testing.T) {
	t.Setenv("LEAFWIKI_ENABLE_MCP", "true")
	t.Setenv("LEAFWIKI_DISABLE_AUTH", "true")
	t.Setenv("LEAFWIKI_HOST", "0.0.0.0")

	opts := resolveLocalMCPOptionsForArgs(t, nil)
	if !opts.EnableMCP || !opts.DisableAuth || opts.Host != "0.0.0.0" {
		t.Fatalf("resolved MCP opts from env = %#v, want env-enabled MCP on wildcard host", opts)
	}
	if err := validateLocalMCPOptions(opts); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("validate env-resolved MCP opts = %v, want loopback error", err)
	}

	opts = resolveLocalMCPOptionsForArgs(t, []string{"--host=127.0.0.1"})
	if opts.Host != "127.0.0.1" {
		t.Fatalf("CLI host did not override env host: %#v", opts)
	}
	if err := validateLocalMCPOptions(opts); err != nil {
		t.Fatalf("validate CLI-overridden MCP opts: %v", err)
	}
}

func TestResolveLocalMCPOptions_EnvRemoteUserCombinationIsAllowed(t *testing.T) {
	t.Setenv("LEAFWIKI_ENABLE_MCP", "true")
	t.Setenv("LEAFWIKI_DISABLE_AUTH", "false")
	t.Setenv("LEAFWIKI_HOST", "127.0.0.1")
	t.Setenv("LEAFWIKI_ENABLE_HTTP_REMOTE_USER", "true")

	opts := resolveLocalMCPOptionsForArgs(t, nil)
	if !opts.HTTPRemoteUserOn {
		t.Fatalf("resolved MCP opts = %#v, want remote-user enabled from env", opts)
	}
	if err := validateLocalMCPOptions(opts); err != nil {
		t.Fatalf("validate env-resolved MCP opts with remote-user auth: %v", err)
	}
}

func TestBuildHTTPRouterOptions_PropagatesMCPEnablement(t *testing.T) {
	opts := buildHTTPRouterOptions(httpRouterOptionsInput{
		publicAccess:        true,
		authDisabled:        true,
		enableMCP:           true,
		host:                "127.0.0.1",
		mcpToolListPageSize: 7,
	})

	if !opts.MCPEnabled {
		t.Fatalf("expected MCPEnabled to be true")
	}
	if opts.MCPToolListPageSize != 7 {
		t.Fatalf("expected MCPToolListPageSize 7, got %d", opts.MCPToolListPageSize)
	}
	if opts.MCPBindHost != "127.0.0.1" {
		t.Fatalf("expected MCPBindHost 127.0.0.1, got %q", opts.MCPBindHost)
	}
}

func TestBuildListenAddress_HandlesIPv6Loopback(t *testing.T) {
	got := buildListenAddress("::1", "8080")
	if got != "[::1]:8080" {
		t.Fatalf("buildListenAddress(::1, 8080) = %q, want %q", got, "[::1]:8080")
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

func TestRegisterFlags_AcceptsDoubleDashLongFlags(t *testing.T) {
	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)

	err := fs.Parse([]string{
		"--jwt-secret=test-secret",
		"--admin-password=test-password",
		"--allow-insecure=true",
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
}

func TestRegisterFlags_AcceptsEnableMCPFlag(t *testing.T) {
	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)

	err := fs.Parse([]string{"--enable-mcp=true"})
	if err != nil {
		t.Fatalf("expected enable-mcp flag to parse, got %v (%s)", err, errOut.String())
	}

	if flags.enableMCP == nil || !*flags.enableMCP {
		t.Fatalf("expected enable-mcp to be true")
	}
}

func TestRegisterFlags_AcceptsRootDirFlag(t *testing.T) {
	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)

	err := fs.Parse([]string{"--root-dir=/tmp/leafwiki-content"})
	if err != nil {
		t.Fatalf("expected root-dir flag to parse, got %v (%s)", err, errOut.String())
	}

	if flags.rootDir == nil || *flags.rootDir != "/tmp/leafwiki-content" {
		t.Fatalf("expected root-dir to be parsed, got %#v", flags.rootDir)
	}
}

func resolveLocalMCPOptionsForArgs(t *testing.T, args []string) localMCPOptions {
	t.Helper()

	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v (%s)", err, errOut.String())
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	return resolveLocalMCPOptions(flags, visited)
}

func resolveWorkspaceForArgs(t *testing.T, args []string) wiki.Workspace {
	t.Helper()

	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v (%s)", err, errOut.String())
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	workspace, err := resolveWorkspace(flags, visited)
	if err != nil {
		t.Fatalf("resolveWorkspace: %v", err)
	}
	return workspace
}
