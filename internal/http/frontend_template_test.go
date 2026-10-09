package http

import (
	"fmt"
	"html"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func frontendTemplateRouter(tb testing.TB, base string) *gin.Engine {
	tb.Helper()
	old := EmbedFrontend
	EmbedFrontend = "true"
	tb.Cleanup(func() { EmbedFrontend = old })
	return NewRouter(nil, FrontendConfig{}, RouterOptions{BasePath: base, DisableRequestLog: true})
}
func TestFrontendTemplate_DynamicBrandingAndDeepRoute_PreserveValues(t *testing.T) {
	if _, err := frontend.Open("dist/index.html"); err != nil {
		t.Skip("requires built frontend (make ui)")
	}
	old := EmbedFrontend
	EmbedFrontend = "true"
	defer func() { EmbedFrontend = old }()
	name, favicon := "First <Wiki>", "first.png"
	router := NewRouter(nil, FrontendConfig{GetSiteName: func() string { return name }, GetFaviconFile: func() string { return favicon }}, RouterOptions{BasePath: "/wiki", DisableRequestLog: true})
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", "/wiki/deep/page", nil))
		body := rec.Body.String()
		if rec.Code != 200 || !strings.Contains(body, html.EscapeString(name)) || !strings.Contains(body, "/wiki/branding/"+favicon) || strings.Contains(body, "{{__") {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		for _, match := range regexp.MustCompile(`(?:src|href)="([^"]+\.(?:js|css))"`).FindAllStringSubmatch(body, -1) {
			asset := httptest.NewRecorder()
			router.ServeHTTP(asset, httptest.NewRequest("GET", match[1], nil))
			if asset.Code != 200 {
				t.Fatalf("%s status %d", match[1], asset.Code)
			}
		}
		name = "Second Wiki"
		favicon = "second.png"
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/outside", nil))
	if rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}
func BenchmarkFrontendHTMLTemplate(b *testing.B) {
	if _, err := frontend.Open("dist/index.html"); err != nil {
		b.Skip("requires built frontend (make ui)")
	}
	for _, base := range []string{"", "/wiki"} {
		b.Run(fmt.Sprintf("base=%s", base), func(b *testing.B) {
			router := frontendTemplateRouter(b, base)
			b.ReportAllocs()
			for b.Loop() {
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest("GET", base+"/deep/page", nil))
				if rec.Code != 200 {
					b.Fatal(rec.Code)
				}
			}
		})
	}
}
