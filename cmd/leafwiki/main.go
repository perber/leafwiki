package main

import (
	"fmt"
	"log"
	"os"

	"github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/wiki"
)

func main() {
	// needs to get injected by environment variable later
	storageDir := "./data"
	w, err := wiki.NewWiki(storageDir)
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
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func printUsage() {
	fmt.Println(`LeafWiki – lightweight selfhosted wiki 🌿

	Usage:
	leafwiki                   Start the web server on port 8080
	leafwiki reset-admin-password    Creates a new password for the admin user

	Examples:
	./leafwiki
	./leafwiki reset-admin-password
	`)
}
