package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/perber/wiki/internal/core/tools"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

func writeUsage(w io.Writer) {
	if _, err := fmt.Fprintln(w, `LeafWiki – lightweight selfhosted wiki 🌿

	Usage:
	leafwiki --jwt-secret <SECRET> --admin-password <PASSWORD> [--host <HOST>] [--port <PORT>] [--data-dir <DIR>] [--root-dir <DIR>]
	leafwiki --disable-auth [--host <HOST>] [--port <PORT>] [--data-dir <DIR>] [--root-dir <DIR>]
	leafwiki reset-admin-password
	leafwiki --help

	Options:
	--host             Host/IP address to bind the server to (default: 127.0.0.1)
	--port             Port to run the server on (default: 8080)
	--data-dir         Path to data directory (default: ./data)
	--root-dir         Path to managed markdown content directory (default: <data-dir>/root)
	--admin-password   Initial admin password (used only if no admin exists)
	--jwt-secret       Secret for signing auth tokens (JWT) (required)
	--public-access    Allow public access to the wiki only with read access (default: false)
	--allow-insecure   Allow insecure HTTP connections (default: false)                      
	--access-token-timeout  Access token timeout duration (e.g. 24h, 15m) (default: 15m)
	--refresh-token-timeout Refresh token timeout duration (e.g. 168h, 7d) (default: 7d)
	--inject-code-in-header  Raw HTML/JS code injected into <head> tag (e.g., analytics, custom CSS) (default: "")
	                         WARNING: Use only with trusted code to avoid XSS vulnerabilities. No sanitization is performed.
	--custom-stylesheet      Path to a .css file inside the data dir, served publicly as /custom.css
	                         (or <base-path>/custom.css when --base-path is set) (default: "")
	--disable-auth                Disable authentication completely (default: false) (WARNING: only use in trusted networks!)
	--hide-link-metadata-section  Hide link metadata section in the frontend UI (default: false)
	--base-path                   URL prefix when served behind a reverse proxy (e.g. /wiki) (default: "")
	--max-asset-upload-size       Maximum size for asset uploads (for example 50MiB, 50MB, 52428800) (default: 50MiB)
	--enable-revision             Enable the revision / page history feature (default: false)
	--enable-link-refactor        Enable the link refactoring dialog and rewrite flow (default: false)
	--enable-mcp                  Enable local MCP Streamable HTTP endpoint (requires loopback host) (default: false)
	--max-revision-history        Maximum revisions kept per page; 0 = unlimited (default: 100)
	--enable-http-remote-user       Enable reverse-proxy authentication via HTTP header (default: false)
	--http-remote-user-header-name  HTTP header carrying the username from a trusted proxy (default: Remote-User)
	--trusted-proxy-ips             Comma-separated trusted proxy IPs/CIDRs (e.g. 127.0.0.1,172.18.0.0/16)
	--http-remote-user-logout-url   URL the frontend redirects to after logout in proxy-auth mode (default: "")
	--disable-request-log           Suppress per-request HTTP access log lines (default: false)

	Environment variables:
	LEAFWIKI_HOST
	LEAFWIKI_PORT
	LEAFWIKI_DATA_DIR
	LEAFWIKI_ROOT_DIR
	LEAFWIKI_JWT_SECRET
	LEAFWIKI_LOG_LEVEL
	LEAFWIKI_ADMIN_PASSWORD
	LEAFWIKI_PUBLIC_ACCESS
	LEAFWIKI_ALLOW_INSECURE
	LEAFWIKI_INJECT_CODE_IN_HEADER
	LEAFWIKI_CUSTOM_STYLESHEET
	LEAFWIKI_ACCESS_TOKEN_TIMEOUT
	LEAFWIKI_REFRESH_TOKEN_TIMEOUT
	LEAFWIKI_DISABLE_AUTH
	LEAFWIKI_HIDE_LINK_METADATA_SECTION
	LEAFWIKI_BASE_PATH
	LEAFWIKI_MAX_ASSET_UPLOAD_SIZE
	LEAFWIKI_ENABLE_REVISION
	LEAFWIKI_ENABLE_LINK_REFACTOR
	LEAFWIKI_ENABLE_MCP
	LEAFWIKI_MAX_REVISION_HISTORY
	LEAFWIKI_ENABLE_HTTP_REMOTE_USER
	LEAFWIKI_HTTP_REMOTE_USER_HEADER_NAME
	LEAFWIKI_TRUSTED_PROXY_IPS
	LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL
	LEAFWIKI_DISABLE_REQUEST_LOG
	`); err != nil {
		panic(err)
	}
}

func printUsage() {
	writeUsage(os.Stdout)
}

func setupLogger() {
	level := slog.LevelInfo
	if os.Getenv("LEAFWIKI_LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	} else if (os.Getenv("LEAFWIKI_LOG_LEVEL")) == "error" {
		level = slog.LevelError
	} else if (os.Getenv("LEAFWIKI_LOG_LEVEL")) == "warn" {
		level = slog.LevelWarn
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})

	slog.SetDefault(slog.New(handler))
}

func fail(msg string, args ...any) {
	slog.Default().Error(msg, args...)
	os.Exit(1)
}

type cliFlags struct {
	host                    *string
	port                    *string
	dataDir                 *string
	rootDir                 *string
	adminPassword           *string
	jwtSecret               *string
	publicAccess            *bool
	allowInsecure           *bool
	injectCodeInHeader      *string
	customStylesheet        *string
	disableAuth             *bool
	hideLinkMetadataSection *bool
	accessTokenTimeout      *time.Duration
	refreshTokenTimeout     *time.Duration
	basePath                *string
	maxAssetUploadSize      *string
	enableRevision          *bool
	enableLinkRefactor      *bool
	enableMCP               *bool
	maxRevisionHistory      *int
	enableHTTPRemoteUser    *bool
	httpRemoteUserHeader    *string
	trustedProxyIPs         *string
	httpRemoteUserLogoutURL *string
	disableRequestLog       *bool
}

func registerFlags(fs *flag.FlagSet) *cliFlags {
	return &cliFlags{
		host:                    fs.String("host", "", "host/IP address to bind the server to (e.g. 127.0.0.1 or 0.0.0.0)"),
		port:                    fs.String("port", "", "port to run the server on"),
		dataDir:                 fs.String("data-dir", "", "path to data directory"),
		rootDir:                 fs.String("root-dir", "", "path to managed markdown content directory"),
		adminPassword:           fs.String("admin-password", "", "initial admin password"),
		jwtSecret:               fs.String("jwt-secret", "", "JWT secret for authentication"),
		publicAccess:            fs.Bool("public-access", false, "allow public access to the wiki with read access (default: false)"),
		allowInsecure:           fs.Bool("allow-insecure", false, "allow insecure HTTP connections (default: false)"),
		injectCodeInHeader:      fs.String("inject-code-in-header", "", "raw string injected into <head> (default: \"\")"),
		customStylesheet:        fs.String("custom-stylesheet", "", "path to a custom CSS file served as /custom.css"),
		disableAuth:             fs.Bool("disable-auth", false, "disable authentication completely (default: false) (WARNING: only use in trusted networks!)"),
		hideLinkMetadataSection: fs.Bool("hide-link-metadata-section", false, "hide link metadata section (default: false)"),
		accessTokenTimeout:      fs.Duration("access-token-timeout", 15*time.Minute, "access token timeout duration (e.g. 24h, 15m) (default: 15m)"),
		refreshTokenTimeout:     fs.Duration("refresh-token-timeout", 7*24*time.Hour, "refresh token timeout duration (e.g. 168h, 7d) (default: 7d)"),
		basePath:                fs.String("base-path", "", "URL prefix when served behind a reverse proxy (e.g. /wiki)"),
		maxAssetUploadSize:      fs.String("max-asset-upload-size", "", "maximum size for asset uploads (for example 50MiB, 50MB, 52428800)"),
		enableRevision:          fs.Bool("enable-revision", false, "enable the revision / page history feature (default: false)"),
		enableLinkRefactor:      fs.Bool("enable-link-refactor", false, "enable the link refactoring dialog and rewrite flow (default: false)"),
		enableMCP:               fs.Bool("enable-mcp", false, "enable local MCP Streamable HTTP endpoint (requires loopback host)"),
		maxRevisionHistory:      fs.Int("max-revision-history", 100, "maximum revisions kept per page; 0 = unlimited (default: 100)"),
		enableHTTPRemoteUser:    fs.Bool("enable-http-remote-user", false, "enable reverse-proxy authentication via HTTP header (default: false)"),
		httpRemoteUserHeader:    fs.String("http-remote-user-header-name", "Remote-User", "HTTP header name carrying the username from a trusted proxy (default: Remote-User)"),
		trustedProxyIPs:         fs.String("trusted-proxy-ips", "", "comma-separated list of trusted proxy IPs/CIDRs (e.g. 127.0.0.1,172.18.0.0/16)"),
		httpRemoteUserLogoutURL: fs.String("http-remote-user-logout-url", "", "URL the frontend redirects to after logout when reverse-proxy auth is active (e.g. https://auth.example.com/logout)"),
		disableRequestLog:       fs.Bool("disable-request-log", false, "suppress per-request HTTP access log lines (default: false)"),
	}
}

func main() {
	setupLogger()
	flag.Usage = func() {
		writeUsage(flag.CommandLine.Output())
	}

	flags := registerFlags(flag.CommandLine)
	flag.Parse()

	// Track which flags were explicitly set on CLI
	visited := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { visited[f.Name] = true })

	host := resolveString("host", *flags.host, visited, "LEAFWIKI_HOST", "127.0.0.1")
	port := resolveString("port", *flags.port, visited, "LEAFWIKI_PORT", "8080")
	dataDir := resolveString("data-dir", *flags.dataDir, visited, "LEAFWIKI_DATA_DIR", "./data")
	workspace, shouldStartWiki, err := resolveStartupWorkspace(flags, visited, flag.Args())
	if err != nil {
		fail("Invalid workspace configuration", "error", err)
	}
	if shouldStartWiki {
		dataDir = workspace.DataDir
	}
	adminPassword := resolveString("admin-password", *flags.adminPassword, visited, "LEAFWIKI_ADMIN_PASSWORD", "")
	jwtSecret := resolveString("jwt-secret", *flags.jwtSecret, visited, "LEAFWIKI_JWT_SECRET", "")
	injectCodeInHeader := resolveString("inject-code-in-header", *flags.injectCodeInHeader, visited, "LEAFWIKI_INJECT_CODE_IN_HEADER", "")
	customStylesheet := resolveString("custom-stylesheet", *flags.customStylesheet, visited, "LEAFWIKI_CUSTOM_STYLESHEET", "")
	allowInsecure := resolveBool("allow-insecure", *flags.allowInsecure, visited, "LEAFWIKI_ALLOW_INSECURE")
	publicAccess := resolveBool("public-access", *flags.publicAccess, visited, "LEAFWIKI_PUBLIC_ACCESS")
	hideLinkMetadataSection := resolveBool("hide-link-metadata-section", *flags.hideLinkMetadataSection, visited, "LEAFWIKI_HIDE_LINK_METADATA_SECTION")
	accessTokenTimeout := resolveDuration("access-token-timeout", *flags.accessTokenTimeout, visited, "LEAFWIKI_ACCESS_TOKEN_TIMEOUT")
	refreshTokenTimeout := resolveDuration("refresh-token-timeout", *flags.refreshTokenTimeout, visited, "LEAFWIKI_REFRESH_TOKEN_TIMEOUT")
	// If disable-auth is set, later logic will override publicAccess accordingly
	disableAuth := resolveBool("disable-auth", *flags.disableAuth, visited, "LEAFWIKI_DISABLE_AUTH")
	basePath := normalizeBasePath(resolveString("base-path", *flags.basePath, visited, "LEAFWIKI_BASE_PATH", ""))
	maxAssetUploadSize := parseByteSize(
		resolveString("max-asset-upload-size", *flags.maxAssetUploadSize, visited, "LEAFWIKI_MAX_ASSET_UPLOAD_SIZE", "50MiB"),
		"max asset upload size",
	)
	enableRevision := resolveBool("enable-revision", *flags.enableRevision, visited, "LEAFWIKI_ENABLE_REVISION")
	enableLinkRefactor := resolveBool("enable-link-refactor", *flags.enableLinkRefactor, visited, "LEAFWIKI_ENABLE_LINK_REFACTOR")
	enableMCP := resolveBool("enable-mcp", *flags.enableMCP, visited, "LEAFWIKI_ENABLE_MCP")
	maxRevisionHistory := resolveInt("max-revision-history", *flags.maxRevisionHistory, visited, "LEAFWIKI_MAX_REVISION_HISTORY", 100)
	enableHTTPRemoteUser := resolveBool("enable-http-remote-user", *flags.enableHTTPRemoteUser, visited, "LEAFWIKI_ENABLE_HTTP_REMOTE_USER")
	httpRemoteUserHeader := resolveString("http-remote-user-header-name", *flags.httpRemoteUserHeader, visited, "LEAFWIKI_HTTP_REMOTE_USER_HEADER_NAME", "Remote-User")
	trustedProxyIPsRaw := resolveString("trusted-proxy-ips", *flags.trustedProxyIPs, visited, "LEAFWIKI_TRUSTED_PROXY_IPS", "")
	httpRemoteUserLogoutURL := resolveString("http-remote-user-logout-url", *flags.httpRemoteUserLogoutURL, visited, "LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL", "")
	disableRequestLog := resolveBool("disable-request-log", *flags.disableRequestLog, visited, "LEAFWIKI_DISABLE_REQUEST_LOG")
	trustedProxies, err := authmw.ParseTrustedProxies(trustedProxyIPsRaw)
	if err != nil {
		fail("invalid --trusted-proxy-ips value", "error", err)
	}

	if err := validateHTTPRemoteUserConfig(enableHTTPRemoteUser, trustedProxyIPsRaw); err != nil {
		fail("Invalid HTTP remote user configuration", "error", err)
	}

	if enableHTTPRemoteUser {
		slog.Default().Info("Reverse-proxy authentication enabled",
			"header", httpRemoteUserHeader,
			"trusted_proxies", trustedProxyIPsRaw,
		)
	}

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "reset-admin-password":
			user, err := tools.ResetAdminPassword(dataDir)
			if err != nil {
				fail("Password reset failed", "error", err)
			}

			fmt.Println("Admin password reset successfully.")
			fmt.Printf("New password for user %s: %s\n", user.Username, user.Password)
			return
		case "--help", "-h", "help":
			printUsage()
			return
		default:
			fmt.Printf("Unknown command: %s\n\n", args[0])
			printUsage()
			return
		}
	}

	if disableAuth {
		publicAccess = true
		slog.Default().Warn("Authentication disabled. Wiki is publicly accessible without authentication.")
	}

	if err := validateLocalMCPOptions(resolveLocalMCPOptions(flags, visited)); err != nil {
		fail("Invalid MCP configuration", "error", err)
	}

	if allowInsecure {
		slog.Default().Warn("allow-insecure enabled. Auth cookies may be transmitted over plain HTTP (INSECURE).")
	}

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			fail("Failed to create data directory", "error", err)
		}
		slog.Default().Info("Data directory created", "path", dataDir)
	}
	if _, err := os.Stat(workspace.RootDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workspace.RootDir, 0755); err != nil {
			fail("Failed to create root directory", "error", err)
		}
	}

	if !disableAuth {
		if jwtSecret == "" {
			fail("JWT secret is required. Set it using --jwt-secret or LEAFWIKI_JWT_SECRET environment variable.")
		}

		if adminPassword == "" {
			fail("admin password is required. Set it using --admin-password or LEAFWIKI_ADMIN_PASSWORD environment variable.")
		}
	}

	w, err := wiki.NewWiki(&wiki.WikiOptions{
		Workspace:           workspace,
		StorageDir:          dataDir,
		AdminPassword:       adminPassword,
		JWTSecret:           jwtSecret,
		AccessTokenTimeout:  accessTokenTimeout,
		RefreshTokenTimeout: refreshTokenTimeout,
		AuthDisabled:        disableAuth,
		EnableRevision:      enableRevision,
		MaxRevisionHistory:  maxRevisionHistory,
	})
	if err != nil {
		fail("Failed to initialize Wiki", "error", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			slog.Default().Error("Failed to close Wiki", "error", err)
		}
	}()

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), buildHTTPRouterOptions(httpRouterOptionsInput{
		publicAccess:            publicAccess,
		injectCodeInHeader:      injectCodeInHeader,
		customStylesheet:        customStylesheet,
		allowInsecure:           allowInsecure,
		hideLinkMetadataSection: hideLinkMetadataSection,
		accessTokenTimeout:      accessTokenTimeout,
		refreshTokenTimeout:     refreshTokenTimeout,
		authDisabled:            disableAuth,
		basePath:                basePath,
		maxAssetUploadSize:      maxAssetUploadSize,
		enableRevision:          enableRevision,
		enableLinkRefactor:      enableLinkRefactor,
		enableMCP:               enableMCP,
		host:                    host,
		httpRemoteUser: httpinternal.HTTPRemoteUserConfig{
			Enabled:        enableHTTPRemoteUser,
			HeaderName:     httpRemoteUserHeader,
			TrustedProxies: trustedProxies,
			UserService:    w.UserService(),
			LogoutURL:      httpRemoteUserLogoutURL,
		},
		disableRequestLog: disableRequestLog,
	}))

	listenAddr := buildListenAddress(host, port)
	slog.Default().Info("Starting LeafWiki", "address", listenAddr, "data_dir", dataDir)
	if err := router.Run(listenAddr); err != nil {
		fail("Failed to start server", "error", err)
	}
}

func buildListenAddress(host, port string) string {
	return net.JoinHostPort(host, port)
}

// CLI > ENV > default(flag)
func resolveString(flagName, flagVal string, visited map[string]bool, envVar string, def string) string {
	// If flag was explicitly set, it takes precedence
	if visited[flagName] {
		return flagVal
	}
	// Next, check environment variable
	if env := strings.TrimSpace(os.Getenv(envVar)); env != "" {
		return env
	}
	// Fall back to provided default when flag wasn't set and no env var is present
	return def
}

func resolveWorkspace(flags *cliFlags, visited map[string]bool) (wiki.Workspace, error) {
	dataDir := resolveString("data-dir", *flags.dataDir, visited, "LEAFWIKI_DATA_DIR", "./data")
	rootDir := resolveString("root-dir", *flags.rootDir, visited, "LEAFWIKI_ROOT_DIR", "")
	workspace := wiki.NormalizeWorkspace(wiki.Workspace{
		ID:      "default",
		DataDir: dataDir,
		RootDir: rootDir,
	})
	if err := validateWorkspaceDirs(workspace.DataDir, workspace.RootDir); err != nil {
		return wiki.Workspace{}, err
	}
	return workspace, nil
}

func resolveStartupWorkspace(flags *cliFlags, visited map[string]bool, args []string) (wiki.Workspace, bool, error) {
	if len(args) > 0 {
		return wiki.Workspace{}, false, nil
	}
	workspace, err := resolveWorkspace(flags, visited)
	if err != nil {
		return wiki.Workspace{}, true, err
	}
	return workspace, true, nil
}

func validateWorkspaceDirs(dataDir string, rootDir string) error {
	return wiki.ValidateWorkspace(wiki.Workspace{
		ID:      "default",
		DataDir: dataDir,
		RootDir: rootDir,
	})
}

// CLI > ENV > default(flag)
func resolveBool(flagName string, flagVal bool, visited map[string]bool, envVar string) bool {
	if visited[flagName] {
		return flagVal
	}
	if env := strings.TrimSpace(os.Getenv(envVar)); env != "" {
		if b, ok := parseBool(env); ok {
			return b
		}
		// If env var is set but invalid, fail fast (helps operators)
		fail("Invalid environment variable value", "variable", envVar, "value", env, "expected", "true/false/1/0/yes/no")
	}
	return flagVal // default from flag
}

func resolveInt(flagName string, flagVal int, visited map[string]bool, envVar string, def int) int {
	if visited[flagName] {
		return flagVal
	}
	if env := strings.TrimSpace(os.Getenv(envVar)); env != "" {
		var n int
		if _, err := fmt.Sscanf(env, "%d", &n); err == nil {
			return n
		}
		fail("Invalid environment variable value", "variable", envVar, "value", env, "expected", "integer")
	}
	return def
}

func resolveDuration(flagName string, flagVal time.Duration, visited map[string]bool, envVar string) time.Duration {
	if visited[flagName] {
		return flagVal
	}
	if env := strings.TrimSpace(os.Getenv(envVar)); env != "" {
		if d, ok := parseDuration(env); ok {
			return d
		}
		// If env var is set but invalid, fail fast (helps operators)
		fail("Invalid environment variable value", "variable", envVar, "value", env, "expected", "duration like 24h, 15m")
	}
	return flagVal // default from flag
}

func parseByteSize(raw string, label string) int64 {
	size, err := humanize.ParseBytes(strings.TrimSpace(raw))
	if err != nil {
		fail("Invalid byte size value", "setting", label, "value", raw, "error", err)
	}
	if size == 0 {
		fail("Byte size value must be greater than zero", "setting", label, "value", raw)
	}
	if size > math.MaxInt64 {
		fail("Byte size value is too large", "setting", label, "value", raw)
	}
	return int64(size)
}

func parseBool(s string) (bool, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "true", "1", "yes", "y", "on":
		return true, true
	case "false", "0", "no", "n", "off":
		return false, true
	}

	return false, false
}

func parseDuration(s string) (time.Duration, bool) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, false
	}
	return d, true
}

func validateHTTPRemoteUserConfig(enabled bool, trustedProxyIPsRaw string) error {
	if !enabled {
		return nil
	}
	hasTrustedProxy := false
	for _, entry := range strings.Split(trustedProxyIPsRaw, ",") {
		if strings.TrimSpace(entry) != "" {
			hasTrustedProxy = true
			break
		}
	}
	if !hasTrustedProxy {
		return fmt.Errorf("--trusted-proxy-ips is required when --enable-http-remote-user is set. Set it using --trusted-proxy-ips or LEAFWIKI_TRUSTED_PROXY_IPS")
	}
	return nil
}

type localMCPOptions struct {
	EnableMCP        bool
	DisableAuth      bool
	HTTPRemoteUserOn bool
	Host             string
}

func resolveLocalMCPOptions(flags *cliFlags, visited map[string]bool) localMCPOptions {
	return localMCPOptions{
		EnableMCP:        resolveBool("enable-mcp", *flags.enableMCP, visited, "LEAFWIKI_ENABLE_MCP"),
		DisableAuth:      resolveBool("disable-auth", *flags.disableAuth, visited, "LEAFWIKI_DISABLE_AUTH"),
		HTTPRemoteUserOn: resolveBool("enable-http-remote-user", *flags.enableHTTPRemoteUser, visited, "LEAFWIKI_ENABLE_HTTP_REMOTE_USER"),
		Host:             resolveString("host", *flags.host, visited, "LEAFWIKI_HOST", "127.0.0.1"),
	}
}

func validateLocalMCPOptions(opts localMCPOptions) error {
	if !opts.EnableMCP {
		return nil
	}
	if !httpinternal.IsLoopbackHost(opts.Host) {
		return fmt.Errorf("--enable-mcp requires a loopback host (localhost, 127.0.0.1, or ::1)")
	}
	return nil
}

type httpRouterOptionsInput struct {
	publicAccess            bool
	injectCodeInHeader      string
	customStylesheet        string
	allowInsecure           bool
	hideLinkMetadataSection bool
	accessTokenTimeout      time.Duration
	refreshTokenTimeout     time.Duration
	authDisabled            bool
	basePath                string
	maxAssetUploadSize      int64
	enableRevision          bool
	enableLinkRefactor      bool
	enableMCP               bool
	host                    string
	mcpToolListPageSize     int
	httpRemoteUser          httpinternal.HTTPRemoteUserConfig
	disableRequestLog       bool
}

func buildHTTPRouterOptions(in httpRouterOptionsInput) httpinternal.RouterOptions {
	return httpinternal.RouterOptions{
		PublicAccess:            in.publicAccess,
		InjectCodeInHeader:      in.injectCodeInHeader,
		CustomStylesheet:        in.customStylesheet,
		AllowInsecure:           in.allowInsecure,
		HideLinkMetadataSection: in.hideLinkMetadataSection,
		AccessTokenTimeout:      in.accessTokenTimeout,
		RefreshTokenTimeout:     in.refreshTokenTimeout,
		AuthDisabled:            in.authDisabled,
		BasePath:                in.basePath,
		MaxAssetUploadSizeBytes: in.maxAssetUploadSize,
		EnableRevision:          in.enableRevision,
		EnableLinkRefactor:      in.enableLinkRefactor,
		MCPEnabled:              in.enableMCP,
		MCPBindHost:             in.host,
		MCPToolListPageSize:     in.mcpToolListPageSize,
		HTTPRemoteUser:          in.httpRemoteUser,
		DisableRequestLog:       in.disableRequestLog,
	}
}

// normalizeBasePath normalizes the base path to the form "/mypath" (no trailing slash).
// Accepts "mypath", "/mypath", "/mypath/", etc. Returns "" for root.
func normalizeBasePath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "/")
	if s == "" {
		return ""
	}
	return "/" + s
}
