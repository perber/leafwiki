package branding

import (
	"log"
	"os"
	"path/filepath"
)

func removeOtherMatches(glob string, keepPath string) {
	matches, err := filepath.Glob(glob)
	if err != nil {
		log.Printf("Failed to glob pattern %s: %v", glob, err)
		return
	}
	for _, p := range matches {
		if p == keepPath {
			continue
		}
		if err := os.Remove(p); err != nil {
			log.Printf("Failed to remove old file %s: %v", p, err)
		}
	}
}
