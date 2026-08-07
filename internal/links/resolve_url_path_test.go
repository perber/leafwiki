package links

import "testing"

func TestResolveURLPath_StripsMarkdownSuffix(t *testing.T) {
	got, err := resolveURLPath("/docs/guide", "setup.md")
	if err != nil {
		t.Fatalf("resolveURLPath: %v", err)
	}
	if got != "/docs/guide/setup" {
		t.Fatalf("got %q, want /docs/guide/setup", got)
	}

	got, err = resolveURLPath("/docs/guide", "../other.md")
	if err != nil {
		t.Fatalf("resolveURLPath: %v", err)
	}
	if got != "/docs/other" {
		t.Fatalf("got %q, want /docs/other", got)
	}
}