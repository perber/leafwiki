package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
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
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/email"
	"github.com/perber/wiki/internal/core/ignore"
	"github.com/perber/wiki/internal/core/tools"
	httpinternal "github.com/perber/wiki/internal/http"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/restore"
	"github.com/perber/wiki/internal/snapshot"
	"github.com/perber/wiki/internal/wiki"
	wikibackup "github.com/perber/wiki/internal/wiki/backup"
	wikiinstancesettings "github.com/perber/wiki/internal/wiki/instancesettings"
	wikirestore "github.com/perber/wiki/internal/wiki/restore"
	wikisnapshot "github.com/perber/wiki/internal/wiki/snapshot"
	"github.com/urfave/cli/v3"
)

// Version is the LeafWiki build version. It defaults to "dev" for
// raw `go build`/`go run` (bypassing make); `make build`, `make run`, and the
// release/Docker builds all inject the real version resolved by
// scripts/resolve-version.sh via -ldflags "-X main.Version=v0.12.0".
var Version = "dev"

const (
	gitBackupSSHKeyFlagName       = "git-backup-ssh-key"
	gitBackupHTTPPasswordFlagName = "git-backup-http-password"
)

func setupLogger(w io.Writer, format string) {
	level := slog.LevelInfo
	switch os.Getenv("LEAFWIKI_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "error":
		level = slog.LevelError
	case "warn":
		level = slog.LevelWarn
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func fail(msg string, args ...any) {
	slog.Default().Error(msg, args...)
	os.Exit(1)
}

var errRestoreSnapshotUsage = errors.New("restore-snapshot requires a path to a snapshot zip: leafwiki [--data-dir <DIR>] restore-snapshot <path-to-zip>")

// runRestoreSnapshotCommand implements the `restore-snapshot` command.
func runRestoreSnapshotCommand(dataDir, snapshotPath string) error {
	if snapshotPath == "" {
		return errRestoreSnapshotUsage
	}
	if err := restore.RestoreOffline(dataDir, snapshotPath); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}
	fmt.Println("Snapshot restored successfully. Start the server normally to pick up the restored data.")
	return nil
}

var gracefulShutdownTimeout = 10 * time.Second

// errLogged reports a failure that has already been logged through the
// configured logger; errReported one that was written to stderr before the
// logger existed. main exits on either without reporting it a second time.
var (
	errLogged   = errors.New("logged error")
	errReported = errors.New("reported error")
)

func main() {
	// Errors that surface here come from flag parsing and validation, which
	// happen before there is a configured logger - they are CLI diagnostics, so
	// they go to stderr as plain text. Anything that fails once the logger is up
	// logs itself and returns errLogged.
	if err := newRootCommand().Run(context.Background(), os.Args); err != nil {
		if !errors.Is(err, errLogged) && !errors.Is(err, errReported) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}

func runResetAdminPasswordCommand(cfg *serverConfig) error {
	user, err := tools.ResetAdminPassword(cfg.server.dataDir, cfg.auth.adminUsername, cfg.auth.adminEmail)
	if err != nil {
		return fmt.Errorf("password reset failed: %w", err)
	}

	fmt.Println("Admin password reset successfully.")
	fmt.Printf("New password for user %s: %s\n", user.Username, user.Password)
	return nil
}

// runServerCommand is the root action: with no subcommand given, leafwiki runs
// the wiki server.
func runServerCommand(_ context.Context, cmd *cli.Command, cfg *serverConfig) error {
	basePath := normalizeBasePath(cfg.server.basePath)
	maxAssetUploadSize := mustParseByteSize(cfg.frontend.maxAssetUploadSize, "max asset upload size")
	restoreUploadMaxSize := mustParseByteSize(cfg.backup.restoreUploadMaxSize, "restore upload max size")

	logoutURL := cfg.proxy.logoutURL
	if resolved, usedDeprecated := resolveLogoutURL(logoutURL, cfg.proxy.httpRemoteUserLogoutURL); usedDeprecated {
		slog.Default().Warn("--http-remote-user-logout-url/LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL is deprecated, use --logout-url/LEAFWIKI_LOGOUT_URL instead")
		logoutURL = resolved
	}

	smtpEnabled := cfg.email.smtpHost != ""

	trustedProxies, err := authmw.ParseTrustedProxies(cfg.proxy.trustedProxyIPs)
	if err != nil {
		fail("invalid --trusted-proxy-ips value", "error", err)
	}
	if err := validateListenConfig(cfg.server.unixSocket, cmd.IsSet("host"), cmd.IsSet("port")); err != nil {
		fail("Invalid listen configuration", "error", err)
	}

	if err := validateHTTPRemoteUserConfig(cfg.proxy.enableHTTPRemoteUser, cfg.proxy.trustedProxyIPs); err != nil {
		fail("Invalid HTTP remote user configuration", "error", err)
	}

	if err := validateHTTPRemoteUserAutoCreateConfig(cfg.proxy.enableHTTPRemoteUserAutoCreate, cfg.proxy.enableHTTPRemoteUser, cfg.proxy.httpRemoteUserDefaultRole); err != nil {
		fail("Invalid HTTP remote user auto-create configuration", "error", err)
	}

	if err := validateSMTPConfig(cfg.email.smtpHost, cfg.email.smtpFrom, cfg.email.publicURL, cfg.email.smtpSecurity); err != nil {
		fail("Invalid SMTP configuration", "error", err)
	}
	if smtpEnabled {
		// A misconfigured public URL produces broken links in emails that
		// have already gone out — irreversible — so log the resolved value
		// once at boot rather than only on first use.
		slog.Default().Info("SMTP email enabled", "host", cfg.email.smtpHost, "port", cfg.email.smtpPort, "security", cfg.email.smtpSecurity, "publicUrl", cfg.email.publicURL)
	}

	if cfg.proxy.enableHTTPRemoteUser {
		slog.Default().Info("Reverse-proxy authentication enabled",
			"header", cfg.proxy.httpRemoteUserHeader,
			"trusted_proxies", cfg.proxy.trustedProxyIPs,
			"auto_create", cfg.proxy.enableHTTPRemoteUserAutoCreate,
			"default_role", cfg.proxy.httpRemoteUserDefaultRole,
		)
	}
	if cfg.metrics.enableMetrics {
		slog.Default().Info("Prometheus metrics enabled",
			"metrics_host", cfg.metrics.metricsHost,
			"metrics_port", cfg.metrics.metricsPort,
		)
	}

	// Validate git backup configuration
	// Note: git-backup-remote is optional (local-only mode is supported)
	if cfg.backup.gitBackup {
		if err := validateGitBackupRemote(cfg.backup.gitBackupRemote, cfg.backup.gitBackupSSHKey, cfg.backup.gitBackupSSHKeyPath, cfg.backup.gitBackupHTTPUsername, cfg.backup.gitBackupHTTPPassword); err != nil {
			fail("Invalid git backup configuration", "error", err)
		}
	}

	if cfg.auth.disableAuth {
		slog.Default().Warn("Authentication disabled. Wiki is publicly accessible without authentication.")
	}

	if cfg.server.allowInsecure {
		slog.Default().Warn("allow-insecure enabled. Auth cookies may be transmitted over plain HTTP (INSECURE).")
	}

	// Check if data directory exists
	if _, err := os.Stat(cfg.server.dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.server.dataDir, 0755); err != nil {
			fail("Failed to create data directory", "error", err)
		}
		slog.Default().Info("Data directory created", "path", cfg.server.dataDir)
	}

	publicAccessService := buildPublicAccessService(cmd, cfg)

	if !cfg.auth.disableAuth {
		if cfg.auth.jwtSecret == "" {
			fail("JWT secret is required. Set it using --jwt-secret or LEAFWIKI_JWT_SECRET environment variable.")
		}

		if cfg.auth.adminPassword == "" {
			fail("admin password is required. Set it using --admin-password or LEAFWIKI_ADMIN_PASSWORD environment variable.")
		}
	}

	var metrics *httpmetrics.HTTPMetrics
	if cfg.metrics.enableMetrics {
		metrics = httpmetrics.NewHTTPMetrics(Version)
	}

	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:             cfg.server.dataDir,
		AdminUsername:          cfg.auth.adminUsername,
		AdminEmail:             cfg.auth.adminEmail,
		AdminPassword:          cfg.auth.adminPassword,
		JWTSecret:              cfg.auth.jwtSecret,
		TOTPEncryptionKey:      cfg.auth.totpEncryptionKey,
		AccessTokenTimeout:     cfg.auth.accessTokenTimeout,
		RefreshTokenTimeout:    cfg.auth.refreshTokenTimeout,
		AuthDisabled:           cfg.auth.disableAuth,
		EnableRevision:         cfg.frontend.enableRevision,
		EnableAPIKeyManagement: cfg.frontend.enableAPIKeyManagement,
		MaxRevisionHistory:     cfg.frontend.maxRevisionHistory,
		EditorLimit:            cfg.auth.editorLimit,
		RevisionCoalesceWindow: cfg.frontend.revisionCoalesceWindow,
		SMTP: email.Config{
			Host:               cfg.email.smtpHost,
			Port:               cfg.email.smtpPort,
			Username:           cfg.email.smtpUsername,
			Password:           cfg.email.smtpPassword,
			From:               cfg.email.smtpFrom,
			FromName:           cfg.email.smtpFromName,
			Security:           email.Security(cfg.email.smtpSecurity),
			InsecureSkipVerify: cfg.email.smtpInsecureSkipVerify,
			Timeout:            cfg.email.smtpTimeout,
			PublicURL:          cfg.email.publicURL,
		},
		Metrics: metrics,
	})
	if err != nil {
		fail("Failed to initialize Wiki", "error", err)
	}

	w.SetInstanceSettingsRoutes(wikiinstancesettings.NewRoutes(publicAccessService, w.AuthService(), slog.Default()))

	// Log .leafwikiignore status
	rootDir := filepath.Join(cfg.server.dataDir, "root")
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
	if cfg.backup.gitBackup {
		if cfg.backup.gitBackupSSHKey != "" && os.Getenv("LEAFWIKI_GIT_BACKUP_SSH_KEY") == "" {
			slog.Warn("SSH private key passed via --git-backup-ssh-key flag is visible in process listings; prefer the LEAFWIKI_GIT_BACKUP_SSH_KEY environment variable")
		}
		if cfg.backup.gitBackupHTTPPassword != "" && os.Getenv("LEAFWIKI_GIT_BACKUP_HTTP_PASSWORD") == "" {
			slog.Warn("HTTP password passed via --git-backup-http-password flag is visible in process listings; prefer the LEAFWIKI_GIT_BACKUP_HTTP_PASSWORD environment variable")
		}
		if strings.HasPrefix(strings.ToLower(cfg.backup.gitBackupRemote), "http://") {
			slog.Warn("--git-backup-remote uses plain http://; git backup credentials and wiki content are transmitted unencrypted — use https:// unless the remote is on a trusted network")
		}
		backupRepo, err := backup.Init(backup.Config{
			Enabled:           true,
			RootDir:           filepath.Join(cfg.server.dataDir, "root"),
			AssetsDir:         filepath.Join(cfg.server.dataDir, "assets"),
			AuthorName:        cfg.backup.gitBackupAuthorName,
			AuthorEmail:       cfg.backup.gitBackupAuthorEmail,
			RemoteURL:         cfg.backup.gitBackupRemote,
			Branch:            cfg.backup.gitBackupBranch,
			SSHKeyPath:        cfg.backup.gitBackupSSHKeyPath,
			SSHKey:            cfg.backup.gitBackupSSHKey,
			SSHKnownHostsPath: cfg.backup.gitBackupSSHKnownHosts,
			HTTPUsername:      cfg.backup.gitBackupHTTPUsername,
			HTTPPassword:      cfg.backup.gitBackupHTTPPassword,
			Interval:          cfg.backup.gitBackupInterval,
		})
		if err != nil {
			fail("git backup init failed: %v", err)
		}
		backupScheduler = backup.NewScheduler(backupRepo)
		defer backupScheduler.Stop()
		w.SetBackupRoutes(wikibackup.NewRoutes(backupRepo, backupScheduler, w.AuthService()))
	}

	// Initialize full backup snapshots if enabled
	var writeGate *restore.WriteGate
	if cfg.backup.snapshot {
		snapshotsDir := cfg.backup.snapshotDir
		if snapshotsDir == "" {
			snapshotsDir = filepath.Join(cfg.server.dataDir, "snapshots")
		}
		snapshotManager := snapshot.NewManager(snapshot.Config{
			BackupsDir:         snapshotsDir,
			RootDir:            filepath.Join(cfg.server.dataDir, "root"),
			AssetsDir:          filepath.Join(cfg.server.dataDir, "assets"),
			BrandingDir:        filepath.Join(cfg.server.dataDir, "branding"),
			AvatarsDir:         filepath.Join(cfg.server.dataDir, "avatars"),
			BrandingConfigFile: filepath.Join(cfg.server.dataDir, "branding.json"),
			SchemaFile:         filepath.Join(cfg.server.dataDir, "schema.json"),
			UsersDBPath:        filepath.Join(cfg.server.dataDir, "users.db"),
			APIKeysDBPath:      filepath.Join(cfg.server.dataDir, "api_keys.db"),
			FavoritesDBPath:    filepath.Join(cfg.server.dataDir, "favorites.db"),
			UserSettingsDBPath: filepath.Join(cfg.server.dataDir, "usersettings.db"),
			WikiVersion:        Version,
			Interval:           cfg.backup.snapshotInterval,
			RetentionCount:     cfg.backup.snapshotRetention,
		})
		snapshotScheduler := snapshot.NewScheduler(snapshotManager)
		defer snapshotScheduler.Stop()
		w.SetSnapshotRoutes(wikisnapshot.NewRoutes(snapshotManager, snapshotScheduler, w.AuthService(), cfg.backup.snapshotRetention))

		writeGate = restore.NewWriteGate()
		restoreManager := restore.NewManager(restore.Config{
			SnapshotManager:    snapshotManager,
			DataDir:            cfg.server.dataDir,
			WikiVersion:        Version,
			WriteGate:          writeGate,
			AuthService:        w.AuthService(),
			APIKeyService:      w.APIKeyService(),
			Favorites:          w.Favorites(),
			UserSettings:       w.UserSettingsService(),
			BrandingService:    w.BrandingService(),
			PublicAccess:       publicAccessService,
			UserResolver:       w.UserResolver(),
			TriggerResync:      w.TriggerResyncAsync,
			MaxUploadSizeBytes: restoreUploadMaxSize,
		})
		// Registered after (so it runs before, defers are LIFO) the w.Close()
		// deferred above: an in-flight restore must finish before AuthService's
		// user/session stores get closed out from under it during shutdown.
		defer restoreManager.Wait()
		w.SetRestoreRoutes(wikirestore.NewRoutes(restoreManager, w.AuthService()))
	}

	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		PublicAccess:            publicAccessService,
		EditorLimit:             cfg.auth.editorLimit,
		InjectCodeInHeader:      cfg.frontend.injectCodeInHeader,
		CustomStylesheet:        cfg.frontend.customStylesheet,
		AllowInsecure:           cfg.server.allowInsecure,
		HideLinkMetadataSection: cfg.frontend.hideLinkMetadataSection,
		AccessTokenTimeout:      cfg.auth.accessTokenTimeout,
		RefreshTokenTimeout:     cfg.auth.refreshTokenTimeout,
		AuthDisabled:            cfg.auth.disableAuth,
		BasePath:                basePath,
		MaxAssetUploadSizeBytes: maxAssetUploadSize,
		EnableRevision:          cfg.frontend.enableRevision,
		EnableLinkRefactor:      cfg.frontend.enableLinkRefactor,
		EnableAPIKeyManagement:  cfg.frontend.enableAPIKeyManagement,
		Metrics:                 metrics,
		GitBackupEnabled:        cfg.backup.gitBackup,
		SnapshotEnabled:         cfg.backup.snapshot,
		SMTPEnabled:             smtpEnabled,
		TOTPAvailable:           w.TOTPService() != nil,
		HTTPRemoteUser: httpinternal.HTTPRemoteUserConfig{
			Enabled:         cfg.proxy.enableHTTPRemoteUser,
			HeaderName:      cfg.proxy.httpRemoteUserHeader,
			AutoCreate:      cfg.proxy.enableHTTPRemoteUserAutoCreate,
			EmailHeaderName: cfg.proxy.httpRemoteUserEmailHeader,
			DefaultRole:     cfg.proxy.httpRemoteUserDefaultRole,
			TrustedProxies:  trustedProxies,
			UserService:     w.UserService,
		},
		APIKeyService:     w.APIKeyService(),
		DisableRequestLog: cfg.server.disableRequestLog,
		UserManagementURL: cfg.proxy.userManagementURL,
		DefaultLanguage:   cfg.frontend.defaultLanguage,
		LoginURL:          cfg.proxy.loginURL,
		LogoutURL:         logoutURL,
		WriteGate:         writeGate,
	})

	reloadSignals := make(chan os.Signal, 1)
	notifyReloadSignals(reloadSignals)
	defer signal.Stop(reloadSignals)

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdownSignals)

	if err := runServer(router, cfg.server.host, cfg.server.port, cfg.server.unixSocket, cfg.server.dataDir, metrics, cfg.metrics.metricsHost, cfg.metrics.metricsPort, w.TriggerResyncAsync, reloadSignals, shutdownSignals); err != nil {
		slog.Default().Error("Failed to start server", "error", err)
		return errLogged
	}

	return nil
}

// buildPublicAccessService decides whether "public mode" is pinned by
// environment configuration or toggleable at runtime from Settings.
//
// It is env-managed when --public-access / LEAFWIKI_PUBLIC_ACCESS was supplied
// explicitly, or when --disable-auth forces public mode on (both cases keep
// today's behaviour and get a status-only Settings view). Otherwise it is
// settings-managed, reading its initial value from
// <data-dir>/public-access.json (absent ⇒ disabled).
func buildPublicAccessService(cmd *cli.Command, cfg *serverConfig) *publicaccess.Service {
	if cmd.IsSet("public-access") || cfg.auth.disableAuth {
		return publicaccess.NewEnvManaged(cfg.auth.publicAccess || cfg.auth.disableAuth)
	}
	svc, err := publicaccess.NewSettingsManaged(cfg.server.dataDir)
	if err != nil {
		fail("Failed to load public-access configuration", "error", err)
	}
	return svc
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

func parseLogFormat(s string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "text":
		return "text", true
	case "json":
		return "json", true
	}
	return "", false
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

func parseByteSize(raw string) (int64, error) {
	size, err := humanize.ParseBytes(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q: %w", raw, err)
	}
	if size == 0 {
		return 0, fmt.Errorf("byte size must be greater than zero, got %q", raw)
	}
	if size > math.MaxInt64 {
		return 0, fmt.Errorf("byte size %q is too large", raw)
	}
	return int64(size), nil
}

// mustParseByteSize is for values a flag validator has already accepted.
func mustParseByteSize(raw, label string) int64 {
	size, err := parseByteSize(raw)
	if err != nil {
		fail("Invalid byte size value", "setting", label, "error", err)
	}
	return size
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

// validateHTTPRemoteUserAutoCreateConfig guards the auto-create opt-in: it requires
// reverse-proxy auth to already be enabled, and forbids "admin" as the default role so
// a forged or misrouted proxy header can never mint an admin account by itself. An
// operator who wants remote-provisioned admins can promote a user manually afterward.
func validateHTTPRemoteUserAutoCreateConfig(autoCreateEnabled, remoteUserEnabled bool, defaultRole string) error {
	if !autoCreateEnabled {
		return nil
	}
	if !remoteUserEnabled {
		return fmt.Errorf("--enable-http-remote-user-auto-create requires --enable-http-remote-user to also be set")
	}
	if !auth.IsValidRole(defaultRole) {
		return fmt.Errorf("--http-remote-user-default-role %q is not a valid role", defaultRole)
	}
	if defaultRole == auth.RoleAdmin {
		return fmt.Errorf("--http-remote-user-default-role must not be %q; promote auto-created users manually instead", auth.RoleAdmin)
	}
	return nil
}

// validateSMTPConfig fails fast at startup, mirroring
// validateHTTPRemoteUserConfig, rather than silently disabling the feature or
// leaving it half-configured until the first send attempt fails: a bad
// --public-url in particular would only surface once already embedded in an
// email that's gone out.
func validateSMTPConfig(host, from, publicURL, security string) error {
	if host == "" {
		return nil
	}
	if from == "" {
		return fmt.Errorf("--smtp-from is required when --smtp-host is set")
	}
	if publicURL == "" {
		return fmt.Errorf("--public-url is required when --smtp-host is set")
	}
	u, err := url.Parse(publicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("--public-url must be an absolute http(s):// URL, got %q", publicURL)
	}
	switch email.Security(security) {
	case email.SecurityNone, email.SecurityStartTLS, email.SecurityTLS:
	default:
		return fmt.Errorf("--smtp-security must be one of none, starttls, tls, got %q", security)
	}
	return nil
}

// resolveLogoutURL resolves --logout-url/LEAFWIKI_LOGOUT_URL, falling back to
// the deprecated --http-remote-user-logout-url/LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL
// when the new option isn't set. usedDeprecated tells the caller whether to log
// a deprecation warning.
func resolveLogoutURL(logoutURL, deprecated string) (resolved string, usedDeprecated bool) {
	if logoutURL != "" {
		return logoutURL, false
	}
	if deprecated == "" {
		return "", false
	}
	return deprecated, true
}

// validateGitBackupRemote checks the git backup remote URL and that credentials
// matching its transport are configured: HTTP(S) remotes authenticate with a
// username + password/token (e.g. a repo-scoped access token), SSH remotes with
// a private key. An empty remote means local-only backup and needs no
// credentials at all.
func validateGitBackupRemote(remote, sshKey, sshKeyPath, httpUsername, httpPassword string) error {
	if remote == "" {
		return nil
	}
	lower := strings.ToLower(remote)

	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		if httpUsername == "" && httpPassword == "" {
			// Credentials may instead be embedded in the URL
			// (https://user:token@host/repo.git), which git supports natively.
			if parsed, err := url.Parse(remote); err == nil && parsed.User != nil {
				return nil
			}
			return fmt.Errorf("--git-backup-http-username and --git-backup-http-password are required when --git-backup-remote is an HTTP(S) URL. Use LEAFWIKI_GIT_BACKUP_HTTP_USERNAME or LEAFWIKI_GIT_BACKUP_HTTP_PASSWORD")
		}
		if httpUsername == "" || httpPassword == "" {
			return fmt.Errorf("--git-backup-http-username and --git-backup-http-password must both be set; got only one of them")
		}
		return nil

	case strings.HasPrefix(lower, "git@"), strings.HasPrefix(lower, "ssh://"):
		if sshKey == "" && sshKeyPath == "" {
			return fmt.Errorf("--git-backup-ssh-key or --git-backup-ssh-key-path is required when --git-backup-remote is set. Use LEAFWIKI_GIT_BACKUP_SSH_KEY or LEAFWIKI_GIT_BACKUP_SSH_KEY_PATH")
		}
		return nil

	default:
		return fmt.Errorf("--git-backup-remote must be an SSH URL (e.g. git@github.com:user/repo.git or ssh://...) or an HTTP(S) URL (e.g. https://github.com/user/repo.git)")
	}
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

func validateListenConfig(unixSocket string, hostSet, portSet bool) error {
	if unixSocket == "" {
		return nil
	}
	if hostSet || portSet {
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
