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

	cases := []struct{ input, want string }{
		{"See [[project plan]] here.", "See [[Project Overview]] here."},
		{"See [[ project plan ]] here.", "See [[Project Overview]] here."},
		{"See [[ Project Plan  ]] here.", "See [[Project Overview]] here."},
	}
	for _, tc := range cases {
		result := engine.RewriteWikiLinks(tc.input, []RewriteRule{{
			OldTitle: "Project Plan",
			NewTitle: "Project Overview",
		}})
		if result.Content != tc.want {
			t.Errorf("input %q: got %q, want %q", tc.input, result.Content, tc.want)
		}
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
	content := "`[[Old Title]]` inline\n\n```\n[[Old Title]]\n```\n\n[[Old Title]] outside"

	result := engine.RewriteWikiLinks(content, []RewriteRule{
		{OldTitle: "Old Title", NewTitle: "New Title"},
	})

	want := "`[[Old Title]]` inline\n\n```\n[[Old Title]]\n```\n\n[[New Title]] outside"
	if result.Content != want {
		t.Errorf("got %q, want %q", result.Content, want)
	}
	if result.Count() != 1 {
		t.Errorf("expected 1 replacement (outside code), got %d", result.Count())
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
	engine := NewMarkdownRefactorEngine()
	content := "See [[docs/intro]] for details."
	found := engine.FindWikiLinksForPath(content, "/docs/intro", "Intro")
	if len(found) == 0 {
		t.Fatal("expected to find path hint [[docs/intro]]")
	}
	if found[0] != "[[docs/intro]]" {
		t.Errorf("got %q, want [[docs/intro]]", found[0])
	}
}

func TestFindWikiLinksForPath_FindsTitleLink(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "See [[Project Plan]] for details."
	found := engine.FindWikiLinksForPath(content, "/docs/project-plan", "Project Plan")
	if len(found) == 0 {
		t.Fatal("expected to find title link [[Project Plan]]")
	}
	if found[0] != "[[Project Plan]]" {
		t.Errorf("got %q, want [[Project Plan]]", found[0])
	}
}

func TestFindWikiLinksForPath_TitleMatchCaseInsensitive(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "See [[project plan]] for details."
	found := engine.FindWikiLinksForPath(content, "/docs/project-plan", "Project Plan")
	if len(found) == 0 {
		t.Fatal("expected case-insensitive title match")
	}
}

func TestFindWikiLinksForPath_SkipsCodeBlocks(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	// Fenced code block: no occurrence outside code → nothing reported.
	fenced := "```\n[[Project Plan]]\n```"
	if found := engine.FindWikiLinksForPath(fenced, "/docs/project-plan", "Project Plan"); len(found) != 0 {
		t.Errorf("fenced block: expected no match, got %v", found)
	}
	// Inline code: also excluded.
	inline := "`[[Project Plan]]`"
	if found := engine.FindWikiLinksForPath(inline, "/docs/project-plan", "Project Plan"); len(found) != 0 {
		t.Errorf("inline code: expected no match, got %v", found)
	}
	// Outside code: should be found.
	outside := "See [[Project Plan]] here."
	if found := engine.FindWikiLinksForPath(outside, "/docs/project-plan", "Project Plan"); len(found) == 0 {
		t.Errorf("outside code: expected match, got none")
	}
}

func TestFindWikiLinksForPath_EmptyContentReturnsNil(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	found := engine.FindWikiLinksForPath("", "/docs/intro", "Intro")
	if len(found) != 0 {
		t.Errorf("expected nil for empty content, got %v", found)
	}
}

func TestFindWikiLinksForPath_NoMatchReturnsEmpty(t *testing.T) {
	engine := NewMarkdownRefactorEngine()
	content := "No wiki links here, just [regular](/links)."
	found := engine.FindWikiLinksForPath(content, "/docs/intro", "Intro")
	if len(found) != 0 {
		t.Errorf("expected no matches, got %v", found)
	}
}
