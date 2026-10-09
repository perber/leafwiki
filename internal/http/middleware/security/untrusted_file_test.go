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
