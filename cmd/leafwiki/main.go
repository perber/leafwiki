package main

import (
	"flag"
	"fmt"
	"log"
	"os"

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

	Environment variables:
	LEAFWIKI_HOST
	LEAFWIKI_PORT
	LEAFWIKI_DATA_DIR
	LEAFWIKI_JWT_SECRET
	LEAFWIKI_ADMIN_PASSWORD
	LEAFWIKI_PUBLIC_ACCESS
	`)
}

func main() {

	// flags
	hostFlag := flag.String("host", "", "host/IP address to bind the server to (e.g. 127.0.0.1 or 0.0.0.0)")
	portFlag := flag.String("port", "", "port to run the server on")
	dataDirFlag := flag.String("data-dir", "", "path to data directory")
	adminPasswordFlag := flag.String("admin-password", "", "initial admin password")
	jwtSecretFlag := flag.String("jwt-secret", "", "JWT secret for authentication")
	publicAccessFlag := flag.String("public-access", "false", "allow public access to the wiki with read access (default: false)")
	flag.Parse()

	port := getOrFallback(*portFlag, "LEAFWIKI_PORT", "8080")
	host := getOrFallback(*hostFlag, "LEAFWIKI_HOST", "0.0.0.0")
	dataDir := getOrFallback(*dataDirFlag, "LEAFWIKI_DATA_DIR", "./data")
	adminPassword := getOrFallback(*adminPasswordFlag, "LEAFWIKI_ADMIN_PASSWORD", "admin")
	jwtSecret := getOrFallback(*jwtSecretFlag, "LEAFWIKI_JWT_SECRET", "")
	publicAccess := getOrFallback(*publicAccessFlag, "LEAFWIKI_PUBLIC_ACCESS", "false")

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

	// needs to get injected by environment variable later
	w, err := wiki.NewWiki(dataDir, adminPassword, jwtSecret, true)
	if err != nil {
		log.Fatalf("Failed to initialize Wiki: %v", err)
	}
	defer w.Close()

	router := http.NewRouter(w, publicAccess == "true")

	// Start server - combine host and port
	listenAddr := host + ":" + port

	// Start server
	if err := router.Run(listenAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getOrFallback(flagVal, envVar, def string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv(envVar); env != "" {
		return env
	}
	return def
}
