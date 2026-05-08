// Package e2eproxy contains integration tests that verify reverse-proxy
// authentication works end-to-end with a real nginx container in front of
// LeafWiki.  The tests expect the Docker Compose stack in this directory to
// be running before they are executed.
//
// Run with:
//
//	docker compose up -d --wait && go test ./... && docker compose down
package e2eproxy

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// proxyURL is the nginx entry point (all requests go through the proxy).
var proxyURL string

// directURL is the LeafWiki port exposed directly, used to verify that the
// same Remote-User header is rejected when it does not come from nginx.
// NOTE: the direct port is intentionally not exposed in docker-compose so
// that test #4 (bypass attempt) must go through the proxy.  We test the
// untrusted-IP scenario by sending to the proxy without X-Test-User instead.
var directURL string

func TestMain(m *testing.M) {
	proxyURL = envOr("E2E_PROXY_URL", "http://localhost:8095")
	directURL = envOr("E2E_DIRECT_URL", "")

	if err := waitReachable(proxyURL+"/api/config", 60*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "LeafWiki proxy stack not reachable at %s: %v\n", proxyURL, err)
		fmt.Fprintln(os.Stderr, "Start the stack first:  docker compose up -d --wait")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func waitReachable(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec,noctx
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}
