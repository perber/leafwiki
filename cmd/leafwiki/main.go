package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/tools"
	"github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/wiki"
)

func printUsage() {
	fmt.Println(`LeafWiki – lightweight selfhosted wiki 🌿

	Usage:
	leafwiki --jwt-secret <SECRET> --admin-password <PASSWORD> [--host <HOST>] [--port <PORT>] [--data-dir <DIR>]
	leafwiki --disable-auth [--host <HOST>] [--port <PORT>] [--data-dir <DIR>]
	leafwiki reset-admin-password
	leafwiki --help

	Options:
	--host             Host/IP address to bind the server to (default: 127.0.0.1)
	--port             Port to run the server on (default: 8080)
	--data-dir         Path to data directory (default: ./data)
	--admin-password   Initial admin password (used only if no admin exists)
	--jwt-secret       Secret for signing auth tokens (JWT) (required)
	--public-access    Allow public access to the wiki only with read access (default: false)
	--allow-insecure   Allow insecure HTTP connections (default: false)                      
	--access-token-timeout  Access token timeout duration (e.g. 24h, 15m) (default: 15m)
	--refresh-token-timeout Refresh token timeout duration (e.g. 168h, 7d) (default: 7d)
	--inject-code-in-header  Raw HTML/JS code injected into <head> tag (e.g., analytics, custom CSS) (default: "")
	                         WARNING: Use only with trusted code to avoid XSS vulnerabilities. No sanitization is performed.
	--disable-auth                Disable authentication completely (default: false) (WARNING: only use in trusted networks!)
	--hide-link-metadata-section  Hide link metadata section in the frontend UI (default: false)

	Environment variables:
	LEAFWIKI_HOST
	LEAFWIKI_PORT
	LEAFWIKI_DATA_DIR
	LEAFWIKI_JWT_SECRET
	LEAFWIKI_ADMIN_PASSWORD
	LEAFWIKI_PUBLIC_ACCESS
	LEAFWIKI_ALLOW_INSECURE
	LEAFWIKI_INJECT_CODE_IN_HEADER
	LEAFWIKI_ACCESS_TOKEN_TIMEOUT
	LEAFWIKI_REFRESH_TOKEN_TIMEOUT
	LEAFWIKI_DISABLE_AUTH
	LEAFWIKI_HIDE_LINK_METADATA_SECTION
	`)
}

func main() {

	// flags
	hostFlag := flag.String("host", "", "host/IP address to bind the server to (e.g. 127.0.0.1 or 0.0.0.0)")
	portFlag := flag.String("port", "", "port to run the server on")
	dataDirFlag := flag.String("data-dir", "", "path to data directory")
	adminPasswordFlag := flag.String("admin-password", "", "initial admin password")
	jwtSecretFlag := flag.String("jwt-secret", "", "JWT secret for authentication")
	publicAccessFlag := flag.Bool("public-access", false, "allow public access to the wiki with read access (default: false)")
	allowInsecureFlag := flag.Bool("allow-insecure", false, "allow insecure HTTP connections (default: false)")
	injectCodeInHeaderFlag := flag.String("inject-code-in-header", "", "raw string injected into <head> (default: \"\")")
	disableAuthFlag := flag.Bool("disable-auth", false, "disable authentication completely (default: false) (WARNING: only use in trusted networks!)")
	hideLinkMetadataSectionFlag := flag.Bool("hide-link-metadata-section", false, "hide link metadata section (default: false)")
	accessTokenTimeoutFlag := flag.Duration("access-token-timeout", 15*time.Minute, "access token timeout duration (e.g. 24h, 15m) (default: 15m)")
	refreshTokenTimeoutFlag := flag.Duration("refresh-token-timeout", 7*24*time.Hour, "refresh token timeout duration (e.g. 168h, 7d) (default: 7d)")
	flag.Parse()

	// Track which flags were explicitly set on CLI
	visited := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { visited[f.Name] = true })

	host := resolveString("host", *hostFlag, visited, "LEAFWIKI_HOST", "127.0.0.1")
	port := resolveString("port", *portFlag, visited, "LEAFWIKI_PORT", "8080")
	dataDir := resolveString("data-dir", *dataDirFlag, visited, "LEAFWIKI_DATA_DIR", "./data")
	adminPassword := resolveString("admin-password", *adminPasswordFlag, visited, "LEAFWIKI_ADMIN_PASSWORD", "")
	jwtSecret := resolveString("jwt-secret", *jwtSecretFlag, visited, "LEAFWIKI_JWT_SECRET", "")
	injectCodeInHeader := resolveString("inject-code-in-header", *injectCodeInHeaderFlag, visited, "LEAFWIKI_INJECT_CODE_IN_HEADER", "")
	allowInsecure := resolveBool("allow-insecure", *allowInsecureFlag, visited, "LEAFWIKI_ALLOW_INSECURE")
	publicAccess := resolveBool("public-access", *publicAccessFlag, visited, "LEAFWIKI_PUBLIC_ACCESS")
	hideLinkMetadataSection := resolveBool("hide-link-metadata-section", *hideLinkMetadataSectionFlag, visited, "LEAFWIKI_HIDE_LINK_METADATA_SECTION")
	accessTokenTimeout := resolveDuration("access-token-timeout", *accessTokenTimeoutFlag, visited, "LEAFWIKI_ACCESS_TOKEN_TIMEOUT")
	refreshTokenTimeout := resolveDuration("refresh-token-timeout", *refreshTokenTimeoutFlag, visited, "LEAFWIKI_REFRESH_TOKEN_TIMEOUT")
	// If disable-auth is set, later logic will override publicAccess accordingly
	disableAuth := resolveBool("disable-auth", *disableAuthFlag, visited, "LEAFWIKI_DISABLE_AUTH")

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "reset-admin-password":
			user, err := tools.ResetAdminPassword(dataDir)
			if err != nil {
				log.Fatalf("Password reset failed: %v", err)
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
		log.Printf("WARNING: Authentication disabled. Wiki is publicly accessible without authentication.")
	}

	if allowInsecure {
		log.Printf("WARNING: allow-insecure enabled. Auth cookies may be transmitted over plain HTTP (INSECURE).")
	}

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Fatalf("Failed to create data directory: %v", err)
		}
	}

	if !disableAuth {
		if jwtSecret == "" {
			log.Fatal("JWT secret is required. Set it using --jwt-secret or LEAFWIKI_JWT_SECRET environment variable.")
		}

		if adminPassword == "" {
			log.Fatalf("admin password is required. Set it using --admin-password or LEAFWIKI_ADMIN_PASSWORD environment variable.")
		}
	}

	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          dataDir,
		AdminPassword:       adminPassword,
		JWTSecret:           jwtSecret,
		AccessTokenTimeout:  accessTokenTimeout,
		RefreshTokenTimeout: refreshTokenTimeout,
		AuthDisabled:        disableAuth,
	})
	if err != nil {
		log.Fatalf("Failed to initialize Wiki: %v", err)
	}
	defer w.Close()

	router := http.NewRouter(w, http.RouterOptions{
		PublicAccess:            publicAccess,
		InjectCodeInHeader:      injectCodeInHeader,
		AllowInsecure:           allowInsecure,
		HideLinkMetadataSection: hideLinkMetadataSection,
		AccessTokenTimeout:      accessTokenTimeout,
		RefreshTokenTimeout:     refreshTokenTimeout,
		AuthDisabled:            disableAuth,
	})

	// Start server - combine host and port
	listenAddr := host + ":" + port

	// Start server
	if err := router.Run(listenAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
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
		log.Fatalf("Invalid value for %s: %q (expected true/false/1/0/yes/no)", envVar, env)
	}
	return flagVal // default from flag
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
		log.Fatalf("Invalid value for %s: %q (expected duration like 24h, 15m)", envVar, env)
	}
	return flagVal // default from flag
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
