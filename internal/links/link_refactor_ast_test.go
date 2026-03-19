package links

import "testing"

func TestMarkdownRefactorEngine_Rewrite_SkipsInlineCodeAndCodeBlocks(t *testing.T) {
	content := "`[code](/docs/b)`\n\n```md\n[block](/docs/b)\n```\n\n[real](/docs/b)"

	result := NewMarkdownRefactorEngine().Rewrite(content, "/docs/a", []RewriteRule{{
		OldPath: "/docs/b",
		NewPath: "/guides/b",
	}})

	if result.Count() != 1 {
		t.Fatalf("expected 1 rewrite for real markdown link, got %d", result.Count())
	}
	if result.Content != "`[code](/docs/b)`\n\n```md\n[block](/docs/b)\n```\n\n[real](/guides/b)" {
		t.Fatalf("unexpected rewritten content:\n%s", result.Content)
	}
}

func TestMarkdownRefactorEngine_Rewrite_WarnsForReferenceLinks(t *testing.T) {
	content := "[Ref][docs]\n\n[docs]: /docs/b"

	result := NewMarkdownRefactorEngine().Rewrite(content, "/docs/a", []RewriteRule{{
		OldPath: "/docs/b",
		NewPath: "/guides/b",
	}})

	if result.Count() != 0 {
		t.Fatalf("expected no inline rewrite for reference links, got %d", result.Count())
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected warning for unsupported reference link syntax")
	}
}
