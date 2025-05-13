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
	leafwiki [--port <PORT>] [--data-dir <DIR>] [--admin-password <PASSWORD>]
	leafwiki reset-admin-password
	leafwiki --help

	Options:
	--port             Port to run the server on (default: 8080)
	--data-dir         Path to storage directory (default: ./data)
	--admin-password   Initial admin password (used only if no admin exists)
	--jwt-secret       Secret for signing auth tokens (JWT) (required)

	Environment variables:
	LEAFWIKI_PORT
	LEAFWIKI_DATA_DIR
	LEAFWIKI_ADMIN_PASSWORD
	`)
}

func main() {

	// flags
	portFlag := flag.String("port", "", "port to run the server on")
	storageFlag := flag.String("data-dir", "", "path to data directory")
	adminPasswordFlag := flag.String("admin-password", "", "initial admin password")
	jwtSecretFlag := flag.String("jwt-secret", "", "JWT secret for authentication")
	flag.Parse()

	port := getOrFallback(*portFlag, "LEAFWIKI_PORT", "8080")
	storageDir := getOrFallback(*storageFlag, "LEAFWIKI_DATA_DIR", "./data")
	adminPassword := getOrFallback(*adminPasswordFlag, "LEAFWIKI_ADMIN_PASSWORD", "admin")
	jwtSecret := getOrFallback(*jwtSecretFlag, "LEAFWIKI_JWT_SECRET", "")

	// Check if storage directory exists
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		if err := os.MkdirAll(storageDir, 0755); err != nil {
			log.Fatalf("Failed to create storage directory: %v", err)
		}
	}

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "reset-admin-password":
			// Note: No JWT secret needed for this command
			w, err := wiki.NewWiki(storageDir, adminPassword, "")
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
			fmt.Printf("Unknown command: %s\n\n", args[1])
			printUsage()
			return
		}
	}

	if jwtSecret == "" {
		log.Fatal("JWT secret is required. Set it using --jwt-secret or LEAFWIKI_JWT_SECRET environment variable.")
	}

	// needs to get injected by environment variable later
	w, err := wiki.NewWiki(storageDir, adminPassword, jwtSecret)
	if err != nil {
		log.Fatalf("Failed to initialize Wiki: %v", err)
	}
	defer w.Close()

	router := http.NewRouter(w)

	// Start server
	if err := router.Run(":" + port); err != nil {
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
