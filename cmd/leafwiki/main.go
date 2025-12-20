package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/wiki"
)

func printUsage() {
	fmt.Println(`LeafWiki – lightweight selfhosted wiki 🌿

	Usage:
	leafwiki [--host <HOST>] [--port <PORT>] [--data-dir <DIR>] [--admin-password <PASSWORD>]
	leafwiki reset-admin-password
	leafwiki --help

	Options:
	--host             Host/IP address to bind the server to (default: 0.0.0.0)
	--port             Port to run the server on (default: 8080)
	--data-dir         Path to data directory (default: ./data)
	--admin-password   Initial admin password (used only if no admin exists)
	--jwt-secret       Secret for signing auth tokens (JWT) (required)
	--public-access    Allow public access to the wiki only with read access (default: false)
	--inject-code-in-header  Raw HTML/JS code injected into <head> tag (e.g., analytics, custom CSS) (default: "")
	                         WARNING: Use only with trusted code to avoid XSS vulnerabilities. No sanitization is performed.
	                         

	Environment variables:
	LEAFWIKI_HOST
	LEAFWIKI_PORT
	LEAFWIKI_DATA_DIR
	LEAFWIKI_JWT_SECRET
	LEAFWIKI_ADMIN_PASSWORD
	LEAFWIKI_PUBLIC_ACCESS
	LEAFWIKI_INJECT_CODE_IN_HEADER
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
	injectCodeInHeaderFlag := flag.String("inject-code-in-header", "", "raw string injected into <head> (default: \"\")")
	flag.Parse()

	// Track which flags were explicitly set on CLI
	visited := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { visited[f.Name] = true })

	host := resolveString("host", *hostFlag, visited, "LEAFWIKI_HOST", "0.0.0.0")
	port := resolveString("port", *portFlag, visited, "LEAFWIKI_PORT", "8080")
	dataDir := resolveString("data-dir", *dataDirFlag, visited, "LEAFWIKI_DATA_DIR", "./data")
	adminPassword := resolveString("admin-password", *adminPasswordFlag, visited, "LEAFWIKI_ADMIN_PASSWORD", "")
	jwtSecret := resolveString("jwt-secret", *jwtSecretFlag, visited, "LEAFWIKI_JWT_SECRET", "")
	injectCodeInHeader := resolveString("inject-code-in-header", *injectCodeInHeaderFlag, visited, "LEAFWIKI_INJECT_CODE_IN_HEADER", "")

	publicAccess := resolveBool("public-access", *publicAccessFlag, visited, "LEAFWIKI_PUBLIC_ACCESS")

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Fatalf("Failed to create data directory: %v", err)
		}
	}

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "reset-admin-password":
			// Note: No JWT secret needed for this command
			w, err := wiki.NewWiki(dataDir, adminPassword, "", false)
			if err != nil {
				log.Fatalf("Failed to initialize Wiki: %v", err)
			}
			defer w.Close()
			user, err := w.ResetAdminUserPassword()
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

	if jwtSecret == "" {
		log.Fatal("JWT secret is required. Set it using --jwt-secret or LEAFWIKI_JWT_SECRET environment variable.")
	}

	enableSearchIndexing := true
	w, err := wiki.NewWiki(dataDir, adminPassword, jwtSecret, enableSearchIndexing)
	if err != nil {
		log.Fatalf("Failed to initialize Wiki: %v", err)
	}
	defer w.Close()

	router := http.NewRouter(w, publicAccess, injectCodeInHeader)

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
	if flagVal != "" {
		return flagVal
	}
	// If flagVal is empty, return provided default
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
