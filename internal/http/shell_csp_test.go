package http_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	wikihttp "github.com/perber/wiki/internal/http"
)

func TestShellCSP_Options_BuildsDocumentPolicy(t *testing.T) {
	const base = "object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"
	const withScript = "script-src 'self'; " + base

	for _, tc := range []struct {
		name string
		opts wikihttp.RouterOptions
		want string
	}{
		{"default", wikihttp.RouterOptions{}, withScript},
		{"base path does not change the policy", wikihttp.RouterOptions{BasePath: "/wiki"}, withScript},
		{"passive header code keeps the script restriction", wikihttp.RouterOptions{InjectCodeInHeader: `<meta name="google-site-verification" content="x">`}, withScript},
		{"active header code drops the script restriction", wikihttp.RouterOptions{InjectCodeInHeader: "<script>x()</script>"}, base},
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

func TestContainsActiveScript_HeaderCode_DetectsScript(t *testing.T) {
	for _, tc := range []struct {
		code string
		want bool
	}{
		{"", false},
		{"   ", false},
		{`<meta name="google-site-verification" content="abc">`, false},
		{`<link rel="stylesheet" href="https://fonts.example/css">`, false},
		{`<style>body{color:red}</style>`, false},
		{`<!-- analytics script goes here later -->`, false},
		{`<meta name="description" content="online documentation">`, false}, // "on" inside a word, no handler
		{`<script src="https://stats.example/a.js"></script>`, true},
		{`<SCRIPT>track()</SCRIPT>`, true},
		{"<script\n  async src=x></script>", true},
		{`<link rel="preload" href="/f.css" as="style" onload="this.rel='stylesheet'">`, true},
		{`<link rel=preload as=style ONLOAD = "x()">`, true},
		{`<img/onerror=alert(1) src=x>`, true},
		{`<a href="javascript:void(0)">x</a>`, true},
		{`<a href="JavaScript:x()">x</a>`, true},
		{"<img src=x\nonerror=alert(1)>", true},
		{"<script\tsrc=x></script>", true},
		{`<svg onload=alert(1)>`, true},
		{`<svg><animate onbegin=alert(1) attributeName=x>`, true},
		{`<div data-on-x="1" data-onclick="y">`, false},
		{`<noscript><img src="/pixel.gif"></noscript>`, false},
		{`<meta http-equiv="refresh" content="0;url=javascript:x()">`, true},
		{`<img src="x"onerror="f()">`, true},                     // no whitespace after the quote; browsers still create the attribute
		{`<script type="application/ld+json">{}</script>`, true}, // never executed, but errs toward true
		{`<a href="java&#x09;script:x()">x</a>`, false},          // known miss: entity-encoded; fails closed (the snippet is blocked visibly)
	} {
		if got := wikihttp.ContainsActiveScript(tc.code); got != tc.want {
			t.Errorf("ContainsActiveScript(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestShellHandler_HeaderCodeWithScript_WarnsAtStartup(t *testing.T) {
	for _, tc := range []struct {
		name     string
		opts     wikihttp.RouterOptions
		wantWarn bool
	}{
		{"script in header code", wikihttp.RouterOptions{InjectCodeInHeader: "<script>x()</script>"}, true},
		{"meta-only header code", wikihttp.RouterOptions{InjectCodeInHeader: `<meta name="x" content="y">`}, false},
		{"no header code", wikihttp.RouterOptions{}, false},
		{"headers disabled", wikihttp.RouterOptions{DisableSecurityHeaders: true, InjectCodeInHeader: "<script>x()</script>"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
			defer slog.SetDefault(previous)

			wikihttp.RenderShell(shellTestTemplate, wikihttp.FrontendConfig{}, tc.opts, "", "/")

			if got := strings.Contains(buf.String(), "inject-code-in-header contains script"); got != tc.wantWarn {
				t.Errorf("warning logged = %v, want %v; log: %s", got, tc.wantWarn, buf.String())
			}
		})
	}
}
