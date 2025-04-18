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
	leafwiki [--port <PORT>] [--storage <DIR>] [--admin-password <PASSWORD>]
	leafwiki reset-admin-password
	leafwiki --help

	Options:
	--port             Port to run the server on (default: 8080)
	--storage          Path to storage directory (default: ./data)
	--admin-password   Initial admin password (used only if no admin exists)

	Environment variables:
	LEAFWIKI_PORT
	LEAFWIKI_STORAGE_DIR
	LEAFWIKI_ADMIN_PASSWORD
	`)
}

func main() {

	// flags
	portFlag := flag.String("port", "", "port to run the server on")
	storageFlag := flag.String("storage", "", "path to storage directory")
	adminPasswordFlag := flag.String("admin-password", "", "initial admin password")
	flag.Parse()

	port := getOrFallback(*portFlag, "LEAFWIKI_PORT", "8080")
	storageDir := getOrFallback(*storageFlag, "LEAFWIKI_STORAGE_DIR", "./data")
	adminPassword := getOrFallback(*adminPasswordFlag, "LEAFWIKI_ADMIN_PASSWORD", "admin")

	// needs to get injected by environment variable later
	w, err := wiki.NewWiki(storageDir, adminPassword)
	if err != nil {
		log.Fatalf("Failed to initialize Wiki: %v", err)
	}
	defer w.Close()

	args := os.Args
	if len(args) > 1 {
		switch args[1] {
		case "reset-admin-password":
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
