package main

import (
	"log"

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

	router := http.NewRouter(w)

	// Start server
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}
