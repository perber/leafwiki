package links

import "testing"

// ─── RewriteWikiLinks ────────────────────────────────────────────────────────

func TestRewriteWikiLinks_TitleRename(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "See [[Project Plan]] and [[Project Plan|our plan]] for details."

	result := engine.RewriteWikiLinks(content, []RewriteRule{{
		OldPath:  "/docs/project-plan",
		NewPath:  "/docs/project-overview",
		OldTitle: "Project Plan",
		NewTitle: "Project Overview",
	}})

	want := "See [[Project Overview]] and [[Project Overview|our plan]] for details."
	if result.Content != want {
		t.Errorf("got %q, want %q", result.Content, want)
	}
	if result.Count() != 2 {
		t.Errorf("expected 2 replacements, got %d", result.Count())
	}
}

func TestRewriteWikiLinks_TitleRenameCaseInsensitive(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "See [[project plan]] here."

	result := engine.RewriteWikiLinks(content, []RewriteRule{{
		OldTitle: "Project Plan",
		NewTitle: "Project Overview",
	}})

	want := "See [[Project Overview]] here."
	if result.Content != want {
		t.Errorf("got %q, want %q", result.Content, want)
	}
}

func TestRewriteWikiLinks_PathHintRewrite(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "See [[docs/intro]] and [[docs/intro|Introduction]]."

	result := engine.RewriteWikiLinks(content, []RewriteRule{{
		OldPath: "/docs/intro",
		NewPath: "/guides/intro",
	}})

	want := "See [[guides/intro]] and [[guides/intro|Introduction]]."
	if result.Content != want {
		t.Errorf("got %q, want %q", result.Content, want)
	}
	if result.Count() != 2 {
		t.Errorf("expected 2 replacements, got %d", result.Count())
	}
}

func TestRewriteWikiLinks_SkipsCodeBlocks(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "```\n[[Old Title]]\n```\n\n[[Old Title]] outside"

	result := engine.RewriteWikiLinks(content, []RewriteRule{
		{OldTitle: "Old Title", NewTitle: "New Title"},
	})

	want := "```\n[[Old Title]]\n```\n\n[[New Title]] outside"
	if result.Content != want {
		t.Errorf("got %q, want %q", result.Content, want)
	}
	if result.Count() != 1 {
		t.Errorf("expected 1 replacement (outside code block), got %d", result.Count())
	}
}

func TestRewriteWikiLinks_NoMatchReturnsUnchanged(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "See [[Other Page]] here."

	result := engine.RewriteWikiLinks(content, []RewriteRule{
		{OldTitle: "Project Plan", NewTitle: "Project Overview"},
	})

	if result.Content != content {
		t.Errorf("content should be unchanged, got %q", result.Content)
	}
	if result.Count() != 0 {
		t.Errorf("expected 0 replacements, got %d", result.Count())
	}
}

func TestRewriteWikiLinks_EmptyRulesNoOp(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "[[Some Page]]"
	result := engine.RewriteWikiLinks(content, nil)
	if result.Content != content {
		t.Errorf("expected unchanged content, got %q", result.Content)
	}
}

// ─── FindWikiLinksForPath ────────────────────────────────────────────────────

func TestFindWikiLinksForPath_FindsPathHint(t *testing.T) {
	content := "See [[docs/intro]] for details."
	found := FindWikiLinksForPath(content, "/docs/intro", "Intro")
	if len(found) == 0 {
		t.Fatal("expected to find path hint [[docs/intro]]")
	}
	if found[0] != "[[docs/intro]]" {
		t.Errorf("got %q, want [[docs/intro]]", found[0])
	}
}

func TestFindWikiLinksForPath_FindsTitleLink(t *testing.T) {
	content := "See [[Project Plan]] for details."
	found := FindWikiLinksForPath(content, "/docs/project-plan", "Project Plan")
	if len(found) == 0 {
		t.Fatal("expected to find title link [[Project Plan]]")
	}
	if found[0] != "[[Project Plan]]" {
		t.Errorf("got %q, want [[Project Plan]]", found[0])
	}
}

func TestFindWikiLinksForPath_TitleMatchCaseInsensitive(t *testing.T) {
	content := "See [[project plan]] for details."
	found := FindWikiLinksForPath(content, "/docs/project-plan", "Project Plan")
	if len(found) == 0 {
		t.Fatal("expected case-insensitive title match")
	}
}

func TestFindWikiLinksForPath_EmptyContentReturnsNil(t *testing.T) {
	found := FindWikiLinksForPath("", "/docs/intro", "Intro")
	if len(found) != 0 {
		t.Errorf("expected nil for empty content, got %v", found)
	}
}

func TestFindWikiLinksForPath_NoMatchReturnsEmpty(t *testing.T) {
	content := "No wiki links here, just [regular](/links)."
	found := FindWikiLinksForPath(content, "/docs/intro", "Intro")
	if len(found) != 0 {
		t.Errorf("expected no matches, got %v", found)
	}
}
