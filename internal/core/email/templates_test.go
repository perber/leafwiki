package email

import (
	"strings"
	"testing"
)

func TestRenderPasswordReset_ProducesLinkInBothBodies(t *testing.T) {
	html, text, err := renderPasswordReset("https://wiki.example.com/reset-password?token=abc.def")
	if err != nil {
		t.Fatalf("renderPasswordReset returned error: %v", err)
	}
	if !strings.Contains(html, "https://wiki.example.com/reset-password?token=abc.def") {
		t.Fatalf("expected html body to contain the link, got: %s", html)
	}
	if !strings.Contains(text, "https://wiki.example.com/reset-password?token=abc.def") {
		t.Fatalf("expected text body to contain the link, got: %s", text)
	}
}

func TestRenderInvite_ProducesLinkInBothBodies(t *testing.T) {
	html, text, err := renderInvite("https://wiki.example.com/accept-invite?token=abc.def")
	if err != nil {
		t.Fatalf("renderInvite returned error: %v", err)
	}
	if !strings.Contains(html, "https://wiki.example.com/accept-invite?token=abc.def") {
		t.Fatalf("expected html body to contain the link, got: %s", html)
	}
	if !strings.Contains(text, "https://wiki.example.com/accept-invite?token=abc.def") {
		t.Fatalf("expected text body to contain the link, got: %s", text)
	}
}

// TestRenderPasswordReset_EscapesHTMLInLink guards the html/template
// auto-escaping layer: even though Link only ever comes from
// EmailTokenService.buildLink (never raw user input) today, a crafted value
// containing HTML metacharacters must not be emitted unescaped into the html
// body.
func TestRenderPasswordReset_EscapesHTMLInLink(t *testing.T) {
	html, _, err := renderPasswordReset(`https://wiki.example.com/reset-password?token="><script>alert(1)</script>`)
	if err != nil {
		t.Fatalf("renderPasswordReset returned error: %v", err)
	}
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected script tag to be escaped, got: %s", html)
	}
}
