package branding

import (
	"log"
	"os"
	"path/filepath"
)

func removeOtherMatches(glob string, keepPath string) {
	matches, _ := filepath.Glob(glob)
	for _, p := range matches {
		if p == keepPath {
			continue
		}
		if err := os.Remove(p); err != nil {
			log.Printf("Failed to remove old file %s: %v", p, err)
		}
	}
}
