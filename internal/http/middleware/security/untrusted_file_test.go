package security

import (
	"net/http"
	"strings"
	"testing"
)

func TestSetUntrustedFileHeaders_ActiveContent_IsSandboxedDownload(t *testing.T) {
	for _, name := range []string{"poc.html", "dir/poc.SVG", "poc.xml", "noext", "/assets/p1/"} {
		h := http.Header{}
		SetUntrustedFileHeaders(h, name)
		if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", name, got)
		}
		if got := h.Get("Content-Security-Policy"); got != "sandbox" {
			t.Errorf("%s: Content-Security-Policy = %q, want sandbox", name, got)
		}
		if got := h.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
			t.Errorf("%s: Content-Disposition = %q, want attachment", name, got)
		}
	}
}

func TestSetUntrustedFileHeaders_Media_IsInlineAndSandboxed(t *testing.T) {
	h := http.Header{}
	SetUntrustedFileHeaders(h, "/assets/p1/Pic.PNG")
	if got := h.Get("Content-Disposition"); got != "inline; filename=Pic.PNG" {
		t.Errorf("Content-Disposition = %q, want inline with filename", got)
	}
	if got := h.Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("Content-Security-Policy = %q, want sandbox", got)
	}
}

func TestSetUntrustedFileHeaders_PDF_IsInlineWithoutSandbox(t *testing.T) {
	h := http.Header{}
	SetUntrustedFileHeaders(h, "doc.pdf")
	if got := h.Get("Content-Disposition"); got != "inline; filename=doc.pdf" {
		t.Errorf("Content-Disposition = %q, want inline with filename", got)
	}
	if got := h.Get("Content-Security-Policy"); got != "" {
		t.Errorf("Content-Security-Policy = %q, want none (Chrome won't render sandboxed PDFs)", got)
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

// A script type would let a same-origin <script src> load an upload; with
// nosniff, application/octet-stream makes the browser refuse it as a script.
func TestSetUntrustedFileHeaders_ScriptTypes_AreServedAsOctetStream(t *testing.T) {
	for _, tc := range []struct{ name, preset string }{
		{"x.js", ""},
		{"/assets/p1/X.JS", ""},
		{"x.mjs", ""},
		{"x.cjs", ""},
		{"x.js", "text/javascript; charset=utf-8"},
		{"x.txt", "text/javascript"},
		{"x.bin", "application/x-javascript"},
		{"x.bin", "application/ecmascript"},
	} {
		h := http.Header{}
		if tc.preset != "" {
			h.Set("Content-Type", tc.preset)
		}
		SetUntrustedFileHeaders(h, tc.name)
		if got := h.Values("Content-Type"); len(got) != 1 || got[0] != "application/octet-stream" {
			t.Errorf("%s (preset %q): Content-Type = %q, want [application/octet-stream]", tc.name, tc.preset, got)
		}
	}
}

// Embedded images and media keep their type; SVG in particular only renders
// in <img> as image/svg+xml.
func TestSetUntrustedFileHeaders_NonScriptTypes_KeepTheirContentType(t *testing.T) {
	for _, tc := range []struct{ name, preset string }{
		{"x.svg", ""},
		{"x.svg", "image/svg+xml"},
		{"x.png", "image/png"},
		{"doc.pdf", "application/pdf"},
		{"note.json", "application/json"},
		{"style.css", "text/css; charset=utf-8"},
	} {
		h := http.Header{}
		if tc.preset != "" {
			h.Set("Content-Type", tc.preset)
		}
		SetUntrustedFileHeaders(h, tc.name)
		if got := h.Get("Content-Type"); got != tc.preset {
			t.Errorf("%s: Content-Type = %q, want %q unchanged", tc.name, got, tc.preset)
		}
	}
}
