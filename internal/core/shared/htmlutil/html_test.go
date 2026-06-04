package htmlutil

import "testing"

func TestEscapeText(t *testing.T) {
	got := EscapeText(`<script>alert("x")</script>`)
	want := "&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;"
	if got != want {
		t.Fatalf("escaped = %q, want %q", got, want)
	}
}

func TestEscapeTextWithAllowedMarkers(t *testing.T) {
	got := EscapeTextWithAllowedMarkers(
		`<script>alert(1)</script> `+"\u0002"+`Search`+"\u0003",
		AllowedMarker{Marker: "\u0002", HTML: "<b>"},
		AllowedMarker{Marker: "\u0003", HTML: "</b>"},
	)

	if got != "&lt;script&gt;alert(1)&lt;/script&gt; <b>Search</b>" {
		t.Fatalf("escaped = %q", got)
	}
}

func TestEscapeTextWithAllowedMarkers_SkipsEmptyMarkers(t *testing.T) {
	got := EscapeTextWithAllowedMarkers(`<b>title</b>`, AllowedMarker{})
	want := "&lt;b&gt;title&lt;/b&gt;"
	if got != want {
		t.Fatalf("escaped = %q, want %q", got, want)
	}
}
