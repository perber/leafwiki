package http_test

import (
	"testing"

	wikihttp "github.com/perber/wiki/internal/http"
)

func TestShellCSP(t *testing.T) {
	const base = "object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"

	for _, tc := range []struct {
		name string
		opts wikihttp.RouterOptions
		want string
	}{
		{"default", wikihttp.RouterOptions{}, base},
		{"base path does not change the policy", wikihttp.RouterOptions{BasePath: "/wiki"}, base},
		{"headers disabled", wikihttp.RouterOptions{DisableSecurityHeaders: true}, ""},
		{"headers disabled with header code", wikihttp.RouterOptions{DisableSecurityHeaders: true, InjectCodeInHeader: "<script>x()</script>"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := wikihttp.ShellCSP(tc.opts); got != tc.want {
				t.Errorf("ShellCSP() = %q, want %q", got, tc.want)
			}
		})
	}
}
