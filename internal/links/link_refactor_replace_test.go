package links

import (
	"strings"
	"testing"
)

func TestMarkdownRefactorEngine_Rewrite_KeepsQueryAndFragment(t *testing.T) {
	content := `[Link](/docs/b?mode=1#intro)`

	result := NewMarkdownRefactorEngine().Rewrite(content, "/docs/a", []RewriteRule{{
		OldPath: "/docs/b",
		NewPath: "/guides/b",
	}})

	if result.Count() != 1 {
		t.Fatalf("expected 1 rewrite, got %d", result.Count())
	}
	if result.Content != `[Link](/guides/b?mode=1#intro)` {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestMarkdownRefactorEngine_Rewrite_SupportsParenthesesInDestination(t *testing.T) {
	content := `[Draft](../page_(draft))`

	result := NewMarkdownRefactorEngine().Rewrite(content, "/docs/a", []RewriteRule{{
		OldPath: "/docs/page_(draft)",
		NewPath: "/guides/page_(draft)",
	}})

	if result.Count() != 1 {
		t.Fatalf("expected 1 rewrite, got %d", result.Count())
	}
	if result.Content != `[Draft](../../guides/page_(draft))` {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestMarkdownRefactorEngine_Rewrite_OnlyChangesDestinationSegment(t *testing.T) {
	content := `[Label with (parens)](/docs/b "Title")`

	result := NewMarkdownRefactorEngine().Rewrite(content, "/docs/a", []RewriteRule{{
		OldPath: "/docs/b",
		NewPath: "/guides/b",
	}})

	if result.Count() != 1 {
		t.Fatalf("expected 1 rewrite, got %d", result.Count())
	}
	if !strings.Contains(result.Content, `[Label with (parens)](/guides/b "Title")`) {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestMarkdownRefactorEngine_RewriteRelativeLinksForPathChange_UsesFolderSemanticsForCrossTreeMove(t *testing.T) {
	content := `[Target](../seite-a)`

	result := NewMarkdownRefactorEngine().RewriteRelativeLinksForPathChange(
		content,
		"/test-link-refactoring/seite-b",
		"/patrick/techtalk/seite-b",
		[]RewriteRule{
			{
				OldPath: "/test-link-refactoring/seite-b",
				NewPath: "/patrick/techtalk/seite-b",
			},
		},
	)

	if result.Count() != 1 {
		t.Fatalf("expected 1 rewrite, got %d", result.Count())
	}
	if result.Content != `[Target](../../../test-link-refactoring/seite-a)` {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestMarkdownRefactorEngine_RewriteRelativeLinksForPathChange_IgnoresRelativeAssetLinks(t *testing.T) {
	content := `[Asset](assets/abc/manual.pdf)`

	result := NewMarkdownRefactorEngine().RewriteRelativeLinksForPathChange(
		content,
		"/docs/a",
		"/guides/a",
		[]RewriteRule{
			{
				OldPath: "/docs/a",
				NewPath: "/guides/a",
			},
		},
	)

	if result.Count() != 0 {
		t.Fatalf("expected no rewrite for asset link, got %d", result.Count())
	}
	if result.Content != content {
		t.Fatalf("asset link should remain unchanged, got %q", result.Content)
	}
}
