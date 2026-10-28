package seo_test

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	wikihttp "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/wiki/seo"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func shellRequest(basePath, routePath string) wikihttp.ShellRequest {
	return wikihttp.ShellRequest{
		Request:   httptest.NewRequest(http.MethodGet, basePath+routePath, nil),
		RoutePath: routePath,
		BasePath:  basePath,
	}
}

func rootRedirect(reader *fakeFirstPageReader) wikihttp.ShellResponder {
	return seo.NewRootRedirect(seo.NewGetRootTargetUseCase(reader), discardLogger())
}

func TestPublicOnly_Responder_RunsOnlyWithPublicAccess(t *testing.T) {
	tests := []struct {
		name        string
		gate        publicaccess.ReadGate
		wantHandled bool
		wantReads   int
	}{
		{name: "public access off: zero reads", gate: publicaccess.Fixed(false), wantHandled: false, wantReads: 0},
		{name: "no gate counts as off", gate: nil, wantHandled: false, wantReads: 0},
		{name: "public access on", gate: publicaccess.Fixed(true), wantHandled: true, wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := firstPage("docs")
			responder := seo.NewPublicOnly(tc.gate).Responder(rootRedirect(reader))

			rec := httptest.NewRecorder()
			handled := responder.RespondShell(rec, shellRequest("", "/"))

			if handled != tc.wantHandled {
				t.Errorf("handled = %v, want %v", handled, tc.wantHandled)
			}
			if reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", reader.reads, tc.wantReads)
			}
			if !tc.wantHandled && (len(rec.Header()) != 0 || rec.Body.Len() != 0) {
				t.Errorf("declining responder wrote headers %v / body %q", rec.Header(), rec.Body.String())
			}
		})
	}
}

func TestRootRedirect_Root_RedirectsToFirstPage(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		headers  map[string]string
		want     string
	}{
		{name: "no base path", want: "/docs"},
		{name: "base path", basePath: "/wiki", want: "/wiki/docs"},
		{
			name:     "forwarded host and proto have no effect",
			basePath: "/wiki",
			headers:  map[string]string{"X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "https", "Host": "evil.example"},
			want:     "/wiki/docs",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := shellRequest(tc.basePath, "/")
			for k, v := range tc.headers {
				req.Request.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			if !rootRedirect(firstPage("docs")).RespondShell(rec, req) {
				t.Fatal("root request was not handled")
			}
			if rec.Code != http.StatusFound {
				t.Errorf("status = %d, want 302", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != tc.want {
				t.Errorf("Location = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRootRedirect_NoTarget_Declines(t *testing.T) {
	tests := []struct {
		name      string
		reader    *fakeFirstPageReader
		routePath string
		wantReads int
	}{
		{name: "not the root", reader: firstPage("docs"), routePath: "/docs", wantReads: 0},
		{name: "empty wiki", reader: &fakeFirstPageReader{}, routePath: "/", wantReads: 1},
		{name: "read error", reader: &fakeFirstPageReader{err: errors.New("boom")}, routePath: "/", wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			if rootRedirect(tc.reader).RespondShell(rec, shellRequest("/wiki", tc.routePath)) {
				t.Fatal("responder handled a request without a target")
			}
			if len(rec.Header()) != 0 || rec.Body.Len() != 0 {
				t.Errorf("declining responder wrote headers %v / body %q", rec.Header(), rec.Body.String())
			}
			if tc.reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", tc.reader.reads, tc.wantReads)
			}
		})
	}
}
