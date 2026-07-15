package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/backup"
	"github.com/perber/wiki/internal/core/ignore"
	"github.com/perber/wiki/internal/core/tools"
	httpinternal "github.com/perber/wiki/internal/http"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
	wikibackup "github.com/perber/wiki/internal/wiki/backup"
)

const (
	gitBackupSSHKeyFlagName = "git-backup-ssh-key"
	errInvalidEnvVarValue   = "Invalid environment variable value"
)

func writeUsage(w io.Writer) {
	if _, err := fmt.Fprintln(w, `LeafWiki – lightweight selfhosted wiki 🌿

	Usage:
	leafwiki --jwt-secret <SECRET> --admin-password <PASSWORD> [--host <HOST>] [--port <PORT>] [--unix-socket <PATH>] [--data-dir <DIR>]
	leafwiki --disable-auth [--host <HOST>] [--port <PORT>] [--unix-socket <PATH>] [--data-dir <DIR>]
	leafwiki reset-admin-password
	leafwiki --help

	Options:
	--host             Host/IP address to bind the server to (default: 127.0.0.1)
	--port             Port to run the server on (default: 8080)
	--unix-socket      Path to a unix domain socket to listen on (overrides --host and --port)
	--data-dir         Path to data directory (default: ./data)
	--admin-password   Initial admin password (used only if no admin exists)
	--admin-username   Initial admin username (used only if no admin exists) (default: admin)
	--admin-email      Initial admin email (used only if no admin exists) (default: admin@localhost)
	--jwt-secret       Secret for signing auth tokens (JWT) (required)
	--public-access    Allow public access to the wiki only with read access (default: false)
	--allow-insecure   Allow insecure HTTP connections (default: false)                      
	--access-token-timeout  Access token timeout duration (e.g. 24h, 15m) (default: 15m)
	--refresh-token-timeout Refresh token timeout duration (e.g. 168h) (default: 168h)
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
	--enable-metrics              Enable the Prometheus /metrics endpoint on a separate listener (default: false)
	--metrics-host                Host/IP for the metrics listener (default: 127.0.0.1)
	--metrics-port                Port for the metrics listener (default: 9091)
	--max-revision-history        Maximum revisions kept per page; 0 = unlimited (default: 100)
	--revision-coalesce-window    Window for coalescing rapid successive saves by the same author (e.g. 5m, 0 = disabled) (default: 5m)
	--enable-http-remote-user       Enable reverse-proxy authentication via HTTP header (default: false)
	--http-remote-user-header-name  HTTP header carrying the username from a trusted proxy (default: Remote-User)
	--trusted-proxy-ips             Comma-separated trusted proxy IPs/CIDRs (e.g. 127.0.0.1,172.18.0.0/16)
	--login-url                     URL the frontend redirects to instead of the built-in login form
	                                 (e.g. an external SSO/IdP login page) (default: "")
	--logout-url                    URL the frontend redirects to after logout
	                                 (e.g. an external SSO/IdP logout page) (default: "")
	--http-remote-user-logout-url   Deprecated: use --logout-url instead
	--user-management-url           URL to an external user-management page; when set, the built-in
	                                 User Management UI is replaced with a link to this URL (default: "")
	--disable-request-log           Suppress per-request HTTP access log lines (default: false)
	--git-backup                   Enable git backup to a remote repository (default: false)
	--git-backup-author-name       Git commit author name for backups (default: LeafWiki Backup)
	--git-backup-author-email      Git commit author email for backups (default: backup@leafwiki.local)
	--git-backup-remote            Git remote URL (SSH) for backups (required when git-backup is enabled)
	--git-backup-branch            Git branch to push to (default: main)
	--git-backup-ssh-key-path      Path to SSH private key for git backup
	--git-backup-ssh-key           Raw SSH private key for git backup (env var preferred)
	--git-backup-ssh-known-hosts   Path to known_hosts file for SSH host key verification (MITM protection)
	--git-backup-interval          Git backup interval (e.g. 60m, 2h); 0 = manual-only, no automatic scheduling (default: 60m)

	Environment variables:
	LEAFWIKI_HOST
	LEAFWIKI_PORT
	LEAFWIKI_UNIX_SOCKET
	LEAFWIKI_DATA_DIR
	LEAFWIKI_JWT_SECRET
	LEAFWIKI_LOG_LEVEL
	LEAFWIKI_ADMIN_PASSWORD
	LEAFWIKI_ADMIN_USERNAME
	LEAFWIKI_ADMIN_EMAIL
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
	LEAFWIKI_ENABLE_METRICS
	LEAFWIKI_METRICS_HOST
	LEAFWIKI_METRICS_PORT
	LEAFWIKI_MAX_REVISION_HISTORY
	LEAFWIKI_REVISION_COALESCE_WINDOW
	LEAFWIKI_ENABLE_HTTP_REMOTE_USER
	LEAFWIKI_HTTP_REMOTE_USER_HEADER_NAME
	LEAFWIKI_TRUSTED_PROXY_IPS
	LEAFWIKI_LOGIN_URL
	LEAFWIKI_LOGOUT_URL
	LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL  (deprecated: use LEAFWIKI_LOGOUT_URL instead)
	LEAFWIKI_USER_MANAGEMENT_URL
	LEAFWIKI_DISABLE_REQUEST_LOG
	LEAFWIKI_GIT_BACKUP
	LEAFWIKI_GIT_BACKUP_AUTHOR_NAME
	LEAFWIKI_GIT_BACKUP_AUTHOR_EMAIL
	LEAFWIKI_GIT_BACKUP_REMOTE
	LEAFWIKI_GIT_BACKUP_BRANCH
	LEAFWIKI_GIT_BACKUP_SSH_KEY_PATH
	LEAFWIKI_GIT_BACKUP_SSH_KEY
	LEAFWIKI_GIT_BACKUP_SSH_KNOWN_HOSTS
	LEAFWIKI_GIT_BACKUP_INTERVAL
	`); err != nil {
		panic(err)
	}
}

func printUsage() {
	writeUsage(os.Stdout)
}

func setupLogger() {
	level := slog.LevelInfo
	switch os.Getenv("LEAFWIKI_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "error":
		level = slog.LevelError
	case "warn":
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

var gracefulShutdownTimeout = 10 * time.Second

type cliFlags struct {
	host                    *string
	port                    *string
	unixSocket              *string
	dataDir                 *string
	adminUsername           *string
	adminEmail              *string
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
	enableMetrics           *bool
	metricsHost             *string
	metricsPort             *string
	maxRevisionHistory      *int
	enableHTTPRemoteUser    *bool
	httpRemoteUserHeader    *string
	trustedProxyIPs         *string
	loginURL                *string
	logoutURL               *string
	httpRemoteUserLogoutURL *string
	userManagementURL       *string
	disableRequestLog       *bool
	gitBackup               *bool
	gitBackupAuthorName     *string
	gitBackupAuthorEmail    *string
	gitBackupRemote         *string
	gitBackupBranch         *string
	gitBackupSSHKeyPath     *string
	gitBackupSSHKey         *string
	gitBackupSSHKnownHosts  *string
	gitBackupInterval       *time.Duration
	revisionCoalesceWindow  *time.Duration
}

func registerFlags(fs *flag.FlagSet) *cliFlags {
	return &cliFlags{
		host:                    fs.String("host", "", "host/IP address to bind the server to (e.g. 127.0.0.1 or 0.0.0.0)"),
		port:                    fs.String("port", "", "port to run the server on"),
		unixSocket:              fs.String("unix-socket", "", "path to a unix domain socket to listen on; overrides --host and --port"),
		dataDir:                 fs.String("data-dir", "", "path to data directory"),
		adminUsername:           fs.String("admin-username", "", "initial admin username (used only if no admin exists) (default: admin)"),
		adminEmail:              fs.String("admin-email", "", "initial admin email (used only if no admin exists) (default: admin@localhost)"),
		adminPassword:           fs.String("admin-password", "", "initial admin password"),
		jwtSecret:               fs.String("jwt-secret", "", "JWT secret for authentication"),
		publicAccess:            fs.Bool("public-access", false, "allow public access to the wiki with read access (default: false)"),
		allowInsecure:           fs.Bool("allow-insecure", false, "allow insecure HTTP connections (default: false)"),
		injectCodeInHeader:      fs.String("inject-code-in-header", "", "raw string injected into <head> (default: \"\")"),
		customStylesheet:        fs.String("custom-stylesheet", "", "path to a custom CSS file served as /custom.css"),
		disableAuth:             fs.Bool("disable-auth", false, "disable authentication completely (default: false) (WARNING: only use in trusted networks!)"),
		hideLinkMetadataSection: fs.Bool("hide-link-metadata-section", false, "hide link metadata section (default: false)"),
		accessTokenTimeout:      fs.Duration("access-token-timeout", 15*time.Minute, "access token timeout duration (e.g. 24h, 15m) (default: 15m)"),
		refreshTokenTimeout:     fs.Duration("refresh-token-timeout", 7*24*time.Hour, "refresh token timeout duration (e.g. 168h) (default: 168h)"),
		basePath:                fs.String("base-path", "", "URL prefix when served behind a reverse proxy (e.g. /wiki)"),
		maxAssetUploadSize:      fs.String("max-asset-upload-size", "", "maximum size for asset uploads (for example 50MiB, 50MB, 52428800)"),
		enableRevision:          fs.Bool("enable-revision", false, "enable the revision / page history feature (default: false)"),
		enableLinkRefactor:      fs.Bool("enable-link-refactor", false, "enable the link refactoring dialog and rewrite flow (default: false)"),
		enableMetrics:           fs.Bool("enable-metrics", false, "enable the Prometheus /metrics endpoint on a separate listener (default: false)"),
		metricsHost:             fs.String("metrics-host", "", "host/IP address for the Prometheus metrics listener (default: 127.0.0.1)"),
		metricsPort:             fs.String("metrics-port", "", "port for the Prometheus metrics listener (default: 9091)"),
		maxRevisionHistory:      fs.Int("max-revision-history", 100, "maximum revisions kept per page; 0 = unlimited (default: 100)"),
		enableHTTPRemoteUser:    fs.Bool("enable-http-remote-user", false, "enable reverse-proxy authentication via HTTP header (default: false)"),
		httpRemoteUserHeader:    fs.String("http-remote-user-header-name", "Remote-User", "HTTP header name carrying the username from a trusted proxy (default: Remote-User)"),
		trustedProxyIPs:         fs.String("trusted-proxy-ips", "", "comma-separated list of trusted proxy IPs/CIDRs (e.g. 127.0.0.1,172.18.0.0/16)"),
		loginURL:                fs.String("login-url", "", "URL the frontend redirects to instead of the built-in login form (e.g. an external SSO/IdP login page)"),
		logoutURL:               fs.String("logout-url", "", "URL the frontend redirects to after logout (e.g. an external SSO/IdP logout page)"),
		httpRemoteUserLogoutURL: fs.String("http-remote-user-logout-url", "", "deprecated: use --logout-url instead"),
		userManagementURL:       fs.String("user-management-url", "", "URL to an external user-management page; when set, the built-in User Management UI is replaced with a link to this URL"),
		disableRequestLog:       fs.Bool("disable-request-log", false, "suppress per-request HTTP access log lines (default: false)"),
		gitBackup:               fs.Bool("git-backup", false, "enable git backup to a remote repository (default: false)"),
		gitBackupAuthorName:     fs.String("git-backup-author-name", "", "git commit author name for backups (default: LeafWiki Backup)"),
		gitBackupAuthorEmail:    fs.String("git-backup-author-email", "", "git commit author email for backups (default: backup@leafwiki.local)"),
		gitBackupRemote:         fs.String("git-backup-remote", "", "git remote URL (SSH) for backups (required when git-backup is enabled)"),
		gitBackupBranch:         fs.String("git-backup-branch", "", "git branch to push to (default: main)"),
		gitBackupSSHKeyPath:     fs.String("git-backup-ssh-key-path", "", "path to SSH private key for git backup"),
		gitBackupSSHKey:         fs.String(gitBackupSSHKeyFlagName, "", "raw SSH private key for git backup (env var preferred)"),
		gitBackupSSHKnownHosts:  fs.String("git-backup-ssh-known-hosts", "", "path to known_hosts file for SSH host key verification (MITM protection)"),
		gitBackupInterval:       fs.Duration("git-backup-interval", 60*time.Minute, "git backup interval (e.g. 60m, 2h); 0 = manual-only, no automatic scheduling (default: 60m)"),
		revisionCoalesceWindow:  fs.Duration("revision-coalesce-window", 5*time.Minute, "window for coalescing rapid successive saves by the same author; 0 = disabled (default: 5m)"),
	}
}

func main() {
	setupLogger()
	exitCode := 0
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

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
	unixSocket := resolveString("unix-socket", *flags.unixSocket, visited, "LEAFWIKI_UNIX_SOCKET", "")
	dataDir := resolveString("data-dir", *flags.dataDir, visited, "LEAFWIKI_DATA_DIR", "./data")
	adminPassword := resolveString("admin-password", *flags.adminPassword, visited, "LEAFWIKI_ADMIN_PASSWORD", "")
	// Empty stays empty here; auth.UserService applies the "admin"/"admin@localhost"
	// fallback itself, so that default lives in exactly one place.
	adminUsername := resolveString("admin-username", *flags.adminUsername, visited, "LEAFWIKI_ADMIN_USERNAME", "")
	adminEmail := resolveString("admin-email", *flags.adminEmail, visited, "LEAFWIKI_ADMIN_EMAIL", "")
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
	enableMetrics := resolveBool("enable-metrics", *flags.enableMetrics, visited, "LEAFWIKI_ENABLE_METRICS")
	metricsHost := resolveString("metrics-host", *flags.metricsHost, visited, "LEAFWIKI_METRICS_HOST", "127.0.0.1")
	metricsPort := resolveString("metrics-port", *flags.metricsPort, visited, "LEAFWIKI_METRICS_PORT", "9091")
	maxRevisionHistory := resolveInt("max-revision-history", *flags.maxRevisionHistory, visited, "LEAFWIKI_MAX_REVISION_HISTORY", 100)
	revisionCoalesceWindow := resolveDuration("revision-coalesce-window", *flags.revisionCoalesceWindow, visited, "LEAFWIKI_REVISION_COALESCE_WINDOW")
	enableHTTPRemoteUser := resolveBool("enable-http-remote-user", *flags.enableHTTPRemoteUser, visited, "LEAFWIKI_ENABLE_HTTP_REMOTE_USER")
	httpRemoteUserHeader := resolveString("http-remote-user-header-name", *flags.httpRemoteUserHeader, visited, "LEAFWIKI_HTTP_REMOTE_USER_HEADER_NAME", "Remote-User")
	trustedProxyIPsRaw := resolveString("trusted-proxy-ips", *flags.trustedProxyIPs, visited, "LEAFWIKI_TRUSTED_PROXY_IPS", "")
	loginURL := resolveString("login-url", *flags.loginURL, visited, "LEAFWIKI_LOGIN_URL", "")
	logoutURL := resolveString("logout-url", *flags.logoutURL, visited, "LEAFWIKI_LOGOUT_URL", "")
	if resolved, usedDeprecated := resolveLogoutURL(logoutURL, *flags.httpRemoteUserLogoutURL, visited, "LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL"); usedDeprecated {
		slog.Default().Warn("--http-remote-user-logout-url/LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL is deprecated, use --logout-url/LEAFWIKI_LOGOUT_URL instead")
		logoutURL = resolved
	}
	userManagementURL := resolveString("user-management-url", *flags.userManagementURL, visited, "LEAFWIKI_USER_MANAGEMENT_URL", "")
	disableRequestLog := resolveBool("disable-request-log", *flags.disableRequestLog, visited, "LEAFWIKI_DISABLE_REQUEST_LOG")
	gitBackupEnabled := resolveBool("git-backup", *flags.gitBackup, visited, "LEAFWIKI_GIT_BACKUP")
	gitBackupAuthorName := resolveString("git-backup-author-name", *flags.gitBackupAuthorName, visited, "LEAFWIKI_GIT_BACKUP_AUTHOR_NAME", "LeafWiki Backup")
	gitBackupAuthorEmail := resolveString("git-backup-author-email", *flags.gitBackupAuthorEmail, visited, "LEAFWIKI_GIT_BACKUP_AUTHOR_EMAIL", "backup@leafwiki.local")
	gitBackupRemote := resolveString("git-backup-remote", *flags.gitBackupRemote, visited, "LEAFWIKI_GIT_BACKUP_REMOTE", "")
	gitBackupBranch := resolveString("git-backup-branch", *flags.gitBackupBranch, visited, "LEAFWIKI_GIT_BACKUP_BRANCH", "main")
	gitBackupSSHKeyPath := resolveString("git-backup-ssh-key-path", *flags.gitBackupSSHKeyPath, visited, "LEAFWIKI_GIT_BACKUP_SSH_KEY_PATH", "")
	gitBackupSSHKey := resolveString(gitBackupSSHKeyFlagName, *flags.gitBackupSSHKey, visited, "LEAFWIKI_GIT_BACKUP_SSH_KEY", "")
	gitBackupInterval := resolveDuration("git-backup-interval", *flags.gitBackupInterval, visited, "LEAFWIKI_GIT_BACKUP_INTERVAL")
	gitBackupSSHKnownHosts := resolveString("git-backup-ssh-known-hosts", *flags.gitBackupSSHKnownHosts, visited, "LEAFWIKI_GIT_BACKUP_SSH_KNOWN_HOSTS", "")
	trustedProxies, err := authmw.ParseTrustedProxies(trustedProxyIPsRaw)
	if err != nil {
		fail("invalid --trusted-proxy-ips value", "error", err)
	}
	if err := validateListenConfig(unixSocket, visited); err != nil {
		fail("Invalid listen configuration", "error", err)
	}

	if err := validateHTTPRemoteUserConfig(enableHTTPRemoteUser, trustedProxyIPsRaw); err != nil {
		fail("Invalid HTTP remote user configuration", "error", err)
	}

	if err := validateRedirectURL("login-url", loginURL); err != nil {
		fail("Invalid login URL configuration", "error", err)
	}
	if err := validateRedirectURL("logout-url", logoutURL); err != nil {
		fail("Invalid logout URL configuration", "error", err)
	}
	// --user-management-url is only ever used as a plain <a href> in the frontend
	// (relative paths and other schemes work fine there), so it isn't restricted
	// to http(s) like --login-url/--logout-url, which the browser navigates to directly.

	if enableHTTPRemoteUser {
		slog.Default().Info("Reverse-proxy authentication enabled",
			"header", httpRemoteUserHeader,
			"trusted_proxies", trustedProxyIPsRaw,
		)
	}
	if enableMetrics {
		slog.Default().Info("Prometheus metrics enabled",
			"metrics_host", metricsHost,
			"metrics_port", metricsPort,
		)
	}

	// Validate git backup configuration
	// Note: git-backup-remote is optional (local-only mode is supported)
	if gitBackupEnabled && gitBackupRemote != "" && gitBackupSSHKey == "" && gitBackupSSHKeyPath == "" {
		fail("--git-backup-ssh-key or --git-backup-ssh-key-path is required when --git-backup-remote is set. Use LEAFWIKI_GIT_BACKUP_SSH_KEY or LEAFWIKI_GIT_BACKUP_SSH_KEY_PATH.")
	}

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "reset-admin-password":
			user, err := tools.ResetAdminPassword(dataDir, adminUsername, adminEmail)
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

	if !disableAuth {
		if jwtSecret == "" {
			fail("JWT secret is required. Set it using --jwt-secret or LEAFWIKI_JWT_SECRET environment variable.")
		}

		if adminPassword == "" {
			fail("admin password is required. Set it using --admin-password or LEAFWIKI_ADMIN_PASSWORD environment variable.")
		}
	}

	var metrics *httpmetrics.HTTPMetrics
	if enableMetrics {
		metrics = httpmetrics.NewHTTPMetrics()
	}

	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:             dataDir,
		AdminUsername:          adminUsername,
		AdminEmail:             adminEmail,
		AdminPassword:          adminPassword,
		JWTSecret:              jwtSecret,
		AccessTokenTimeout:     accessTokenTimeout,
		RefreshTokenTimeout:    refreshTokenTimeout,
		AuthDisabled:           disableAuth,
		EnableRevision:         enableRevision,
		MaxRevisionHistory:     maxRevisionHistory,
		RevisionCoalesceWindow: revisionCoalesceWindow,
		Metrics:                metrics,
	})
	if err != nil {
		fail("Failed to initialize Wiki", "error", err)
	}

	// Log .leafwikiignore status
	rootDir := filepath.Join(dataDir, "root")
	if ignoreFile, err := ignore.LoadFromDir(rootDir); err != nil {
		slog.Default().Warn("invalid .leafwikiignore", "error", err)
	} else if ignoreFile != nil {
		slog.Default().Info("loaded .leafwikiignore", "patterns", ignoreFile.PatternCount())
	}

	defer func() {
		if err := w.Close(); err != nil {
			slog.Default().Error("Failed to close Wiki", "error", err)
		}
	}()

	// Initialize git backup if enabled
	var backupScheduler *backup.Scheduler
	if gitBackupEnabled {
		if gitBackupRemote != "" && !strings.HasPrefix(gitBackupRemote, "git@") && !strings.HasPrefix(gitBackupRemote, "ssh://") {
			fail("--git-backup-remote must be an SSH URL (e.g. git@github.com:user/repo.git or ssh://...)")
		}
		if visited[gitBackupSSHKeyFlagName] {
			slog.Warn("SSH private key passed via --git-backup-ssh-key flag is visible in process listings; prefer the LEAFWIKI_GIT_BACKUP_SSH_KEY environment variable")
		}
		backupRepo, err := backup.Init(backup.Config{
			Enabled:           true,
			RootDir:           filepath.Join(dataDir, "root"),
			AssetsDir:         filepath.Join(dataDir, "assets"),
			AuthorName:        gitBackupAuthorName,
			AuthorEmail:       gitBackupAuthorEmail,
			RemoteURL:         gitBackupRemote,
			Branch:            gitBackupBranch,
			SSHKeyPath:        gitBackupSSHKeyPath,
			SSHKey:            gitBackupSSHKey,
			SSHKnownHostsPath: gitBackupSSHKnownHosts,
			Interval:          gitBackupInterval,
		})
		if err != nil {
			fail("git backup init failed: %v", err)
		}
		backupScheduler = backup.NewScheduler(backupRepo)
		defer backupScheduler.Stop()
		w.SetBackupRoutes(wikibackup.NewRoutes(backupRepo, backupScheduler, w.AuthService()))
	}

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            publicAccess,
		InjectCodeInHeader:      injectCodeInHeader,
		CustomStylesheet:        customStylesheet,
		AllowInsecure:           allowInsecure,
		HideLinkMetadataSection: hideLinkMetadataSection,
		AccessTokenTimeout:      accessTokenTimeout,
		RefreshTokenTimeout:     refreshTokenTimeout,
		AuthDisabled:            disableAuth,
		BasePath:                basePath,
		MaxAssetUploadSizeBytes: maxAssetUploadSize,
		EnableRevision:          enableRevision,
		EnableLinkRefactor:      enableLinkRefactor,
		Metrics:                 metrics,
		GitBackupEnabled:        gitBackupEnabled,
		HTTPRemoteUser: httpinternal.HTTPRemoteUserConfig{
			Enabled:        enableHTTPRemoteUser,
			HeaderName:     httpRemoteUserHeader,
			TrustedProxies: trustedProxies,
			UserService:    w.UserService(),
		},
		DisableRequestLog: disableRequestLog,
		UserManagementURL: userManagementURL,
		LoginURL:          loginURL,
		LogoutURL:         logoutURL,
	})

	reloadSignals := make(chan os.Signal, 1)
	notifyReloadSignals(reloadSignals)
	defer signal.Stop(reloadSignals)

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdownSignals)

	if err := runServer(router, host, port, unixSocket, dataDir, metrics, metricsHost, metricsPort, w.TriggerResyncAsync, reloadSignals, shutdownSignals); err != nil {
		slog.Default().Error("Failed to start server", "error", err)
		exitCode = 1
		return
	}
}

func runServer(
	router *gin.Engine,
	host, port, unixSocket, dataDir string,
	metrics *httpmetrics.HTTPMetrics,
	metricsHost, metricsPort string,
	reload func(),
	reloadSignals, shutdownSignals <-chan os.Signal,
) error {
	server := &http.Server{
		Handler:           router.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	var shutdownMetricsServer func()
	if metrics != nil {
		stopMetricsServer, _, err := startMetricsServer(metrics, metricsHost, metricsPort)
		if err != nil {
			return err
		}
		shutdownMetricsServer = stopMetricsServer
		defer shutdownMetricsServer()
	}

	if unixSocket == "" {
		listenAddr := host + ":" + port
		slog.Default().Info("Starting LeafWiki", "address", listenAddr, "data_dir", dataDir)
		listener, err := net.Listen("tcp", listenAddr)
		if err != nil {
			return err
		}
		return serveWithLifecycle(server, listener, nil, reload, reloadSignals, shutdownSignals)
	}

	listener, err := listenOnUnixSocket(unixSocket)
	if err != nil {
		return err
	}

	cleanup := func() {
		_ = listener.Close()
		_ = os.Remove(unixSocket)
	}
	defer cleanup()

	slog.Default().Info(
		"Starting LeafWiki",
		"unix_socket", unixSocket,
		"data_dir", dataDir,
		"host_port_overridden", true,
	)

	return serveWithLifecycle(server, listener, cleanup, reload, reloadSignals, shutdownSignals)
}

func startMetricsServer(metrics *httpmetrics.HTTPMetrics, host, port string) (func(), string, error) {
	listenAddr := host + ":" + port
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, "", err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.HTTPHandler())

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Default().Info("Starting metrics server", "address", listener.Addr().String())

	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		slog.Default().Error("metrics server stopped unexpectedly", "error", err)
	}()

	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
				slog.Default().Error("failed to close metrics server", "error", errors.Join(err, closeErr))
				return
			}
			slog.Default().Error("failed to shut down metrics server gracefully", "error", err)
		}
	}, listener.Addr().String(), nil
}

func listenOnUnixSocket(socketPath string) (net.Listener, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("unix sockets are not supported on windows")
	}
	if err := removeStaleUnixSocket(socketPath); err != nil {
		return nil, err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socketPath, 0660); err != nil {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, err
	}

	return listener, nil
}

func removeStaleUnixSocket(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("unix socket path already exists and is not a socket: %s", socketPath)
		}
		return os.Remove(socketPath)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// CLI > ENV > default(flag)
func resolveString(flagName, flagVal string, visited map[string]bool, envVar string, def string) string {
	// If flag was explicitly set, it takes precedence
	if visited[flagName] {
		return strings.TrimSpace(flagVal)
	}
	// Next, check environment variable
	if env := strings.TrimSpace(os.Getenv(envVar)); env != "" {
		return env
	}
	// Fall back to provided default when flag wasn't set and no env var is present
	return def
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
		fail(errInvalidEnvVarValue, "variable", envVar, "value", env, "expected", "true/false/1/0/yes/no")
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
		fail(errInvalidEnvVarValue, "variable", envVar, "value", env, "expected", "integer")
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
		fail(errInvalidEnvVarValue, "variable", envVar, "value", env, "expected", "duration like 24h, 15m")
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

// resolveLogoutURL resolves --logout-url/LEAFWIKI_LOGOUT_URL, falling back to
// the deprecated --http-remote-user-logout-url/LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL
// when the new option isn't set. usedDeprecated tells the caller whether to log
// a deprecation warning.
func resolveLogoutURL(logoutURL, deprecatedFlagVal string, visited map[string]bool, deprecatedEnvVar string) (resolved string, usedDeprecated bool) {
	if logoutURL != "" {
		return logoutURL, false
	}
	deprecated := resolveString("http-remote-user-logout-url", deprecatedFlagVal, visited, deprecatedEnvVar, "")
	if deprecated == "" {
		return "", false
	}
	return deprecated, true
}

func validateRedirectURL(flagName, url string) error {
	if url == "" {
		return nil
	}
	lower := strings.ToLower(url)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return fmt.Errorf("--%s must start with http:// or https://", flagName)
	}
	return nil
}

func validateListenConfig(unixSocket string, visited map[string]bool) error {
	if unixSocket == "" {
		return nil
	}
	if visited["host"] || visited["port"] {
		return fmt.Errorf("--unix-socket cannot be used together with --host or --port")
	}
	return nil
}

// serveWithLifecycle runs the HTTP server while handling live reload signals and
// graceful shutdown signals. reload is non-blocking (fires a goroutine internally).
func serveWithLifecycle(
	server *http.Server,
	listener net.Listener,
	cleanup func(),
	reload func(),
	reloadSignals, shutdownSignals <-chan os.Signal,
) error {
	serveErrCh := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErrCh <- err
	}()

	stopReloads := make(chan struct{})
	var shuttingDown atomic.Bool
	var reloadWG sync.WaitGroup
	reloadWG.Add(1)
	go func() {
		defer reloadWG.Done()
		for {
			select {
			case <-stopReloads:
				return
			case sig, ok := <-reloadSignals:
				if !ok {
					return
				}
				if shuttingDown.Load() {
					return
				}
				slog.Default().Info("reload signal received: reloading from filesystem", "signal", sig.String())
				reload()
			}
		}
	}()

	stopReloader := sync.OnceFunc(func() {
		close(stopReloads)
	})
	defer stopReloader()

	waitForReloader := sync.OnceFunc(func() {
		reloadWG.Wait()
	})
	defer waitForReloader()

	for {
		select {
		case err := <-serveErrCh:
			return err
		case sig, ok := <-shutdownSignals:
			if !ok {
				shutdownSignals = nil
				continue
			}

			slog.Default().Info("shutdown signal received: shutting down server", "signal", sig.String())
			shuttingDown.Store(true)
			stopReloader()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
			err := server.Shutdown(shutdownCtx)
			cancel()
			if err != nil {
				closeErr := server.Close()
				if closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
					err = errors.Join(err, closeErr)
				}
			}

			if cleanup != nil {
				cleanup()
			}

			waitForReloader()
			serveErr := <-serveErrCh
			if err != nil {
				return err
			}
			return serveErr
		}
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
