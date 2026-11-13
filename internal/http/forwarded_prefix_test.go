package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/test_utils"
)

// gin prepends X-Forwarded-Prefix to the Location of its trailing-slash
// redirect. LeafWiki has --base-path for that, so a client must not be able to
// steer the redirect target with the header, with or without a base path.
func TestHandler_TrailingSlashRedirect_IgnoresForwardedPrefix(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	for _, tc := range []struct {
		name, basePath, path, wantLocation string
	}{
		{name: "no base path", path: "/api/config/", wantLocation: "/api/config"},
		{name: "base path", basePath: "/wiki", path: "/wiki/api/config/", wantLocation: "/wiki/api/config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := httpinternal.Handler(newTestRouter(w, func(o *httpinternal.RouterOptions) {
				o.BasePath = tc.basePath
			}))
			for _, prefix := range []string{"", "/evil", "//evil.example"} {
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				if prefix != "" {
					req.Header.Set("X-Forwarded-Prefix", prefix)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusMovedPermanently {
					t.Fatalf("prefix %q: status = %d, want 301", prefix, rec.Code)
				}
				if got := rec.Header().Get("Location"); got != tc.wantLocation {
					t.Errorf("prefix %q: Location = %q, want %q", prefix, got, tc.wantLocation)
				}
			}
		})
	}
}
