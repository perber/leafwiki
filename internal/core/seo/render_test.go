package seo_test

import (
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/seo"
)

func TestRenderHTML_BasicMarkdown(t *testing.T) {
	html, err := seo.RenderHTML("# Hello\n\nWorld")
	if err != nil {
		t.Fatalf("RenderHTML returned error: %v", err)
	}
	if !strings.Contains(html, "<h1") || !strings.Contains(html, "Hello") {
		t.Fatalf("expected rendered heading, got %q", html)
	}
	if !strings.Contains(html, "<p>World</p>") {
		t.Fatalf("expected rendered paragraph, got %q", html)
	}
}

func TestRenderHTML_GFMTable(t *testing.T) {
	md := "| A | B |\n|---|---|\n| 1 | 2 |\n"
	html, err := seo.RenderHTML(md)
	if err != nil {
		t.Fatalf("RenderHTML returned error: %v", err)
	}
	if !strings.Contains(html, "<table") {
		t.Fatalf("expected GFM table extension to render <table>, got %q", html)
	}
}

func TestRenderHTML_StripsScriptTags(t *testing.T) {
	md := "Hello\n\n<script>alert(1)</script>\n\nWorld"
	html, err := seo.RenderHTML(md)
	if err != nil {
		t.Fatalf("RenderHTML returned error: %v", err)
	}
	if strings.Contains(html, "<script") {
		t.Fatalf("expected <script> to be stripped by sanitizer, got %q", html)
	}
	if strings.Contains(html, "alert(1)") {
		t.Fatalf("expected script body to be stripped, got %q", html)
	}
}

func TestRenderHTML_StripsEventHandlerAttributes(t *testing.T) {
	md := `<img src="x" onerror="alert(1)">`
	html, err := seo.RenderHTML(md)
	if err != nil {
		t.Fatalf("RenderHTML returned error: %v", err)
	}
	if strings.Contains(html, "onerror") {
		t.Fatalf("expected onerror attribute to be stripped, got %q", html)
	}
}
