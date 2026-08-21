package main

import (
	"os"
	"strings"
)

// lookupEnv resolves a process environment value with Docker-style *_FILE
// support. Precedence for callers that also check CLI flags is:
//
//	CLI > NAME_FILE > NAME > default
//
// When NAME_FILE is set, the file contents are used (trailing whitespace trimmed)
// and NAME is ignored. A missing/unreadable file fails startup.
func lookupEnv(name string) string {
	if path := strings.TrimSpace(os.Getenv(name + "_FILE")); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			fail("Failed to read environment file", "variable", name+"_FILE", "path", path, "error", err)
		}
		return strings.TrimSpace(string(b))
	}
	return strings.TrimSpace(os.Getenv(name))
}
