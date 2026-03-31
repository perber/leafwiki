package importer

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

func TestContentTransformer_TransformContent_TableDriven(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Note.md", "# Note")
	writeTmp(t, tmp, "Three laws of motion.md", "# Three laws")
	writeTmp(t, tmp, "foo/bar.md", "# Bar")
	writeTmp(t, tmp, "Document.pdf", "pdf-bytes")
	writeTmp(t, tmp, "Image.png", "png-bytes")
	writeTmp(t, tmp, "img.png", "png-bytes")

	transformer := newContentTransformer(&PlanResult{
		Items: []PlanItem{
			{SourcePath: "Note.md", TargetPath: "note", Kind: tree.NodeKindPage},
			{SourcePath: "Three laws of motion.md", TargetPath: "three-laws-of-motion", Kind: tree.NodeKindPage},
			{SourcePath: "foo/bar.md", TargetPath: "foo/bar", Kind: tree.NodeKindPage},
		},
	}, tmp, 1234)

	page := &tree.Page{PageNode: &tree.PageNode{ID: "p1", Kind: tree.NodeKindPage}}
	wiki := &fakeExecWiki{}

	tests := []struct {
		name       string
		sourcePath string
		content    string
		want       string
	}{
		{
			name:       "normal markdown link",
			sourcePath: "docs/current.md",
			content:    "[Note](../Note.md)",
			want:       "[Note](/note)",
		},
		{
			name:       "markdown link with title",
			sourcePath: "docs/current.md",
			content:    "[Note](../Note.md \"Tooltip\")",
			want:       "[Note](/note \"Tooltip\")",
		},
		{
			name:       "wiki link",
			sourcePath: "docs/current.md",
			content:    "[[Note]]",
			want:       "[Note](/note)",
		},
		{
			name:       "wiki link alias",
			sourcePath: "docs/current.md",
			content:    "[[Note|Alias]]",
			want:       "[Alias](/note)",
		},
		{
			name:       "wiki link anchor",
			sourcePath: "docs/current.md",
			content:    "[[Note#Heading]]",
			want:       "[Note](/note#Heading)",
		},
		{
			name:       "wiki link anchor alias",
			sourcePath: "docs/current.md",
			content:    "[[Note#Heading|Alias]]",
			want:       "[Alias](/note#Heading)",
		},
		{
			name:       "wiki link block reference",
			sourcePath: "docs/current.md",
			content:    "[[Note#^block-id]]",
			want:       "[Note](/note#^block-id)",
		},
		{
			name:       "asset embed",
			sourcePath: "docs/current.md",
			content:    "![[../img.png]]",
			want:       "![img.png](/assets/p1/img.png)",
		},
		{
			name:       "non image wiki asset stays a link",
			sourcePath: "docs/current.md",
			content:    "[[../Document.pdf]]",
			want:       "[Document.pdf](/assets/p1/Document.pdf)",
		},
		{
			name:       "image wiki asset renders as image",
			sourcePath: "docs/current.md",
			content:    "[[../Image.png]]",
			want:       "![Image.png](/assets/p1/Image.png)",
		},
		{
			name:       "relative markdown link",
			sourcePath: "docs/current.md",
			content:    "[Bar](../foo/bar.md)",
			want:       "[Bar](/foo/bar)",
		},
		{
			name:       "percent encoded markdown link",
			sourcePath: "docs/current.md",
			content:    "[Three laws](../Three%20laws%20of%20motion.md)",
			want:       "[Three laws](/three-laws-of-motion)",
		},
		{
			name:       "inline code stays unchanged",
			sourcePath: "docs/current.md",
			content:    "`[[Note]]`",
			want:       "`[[Note]]`",
		},
		{
			name:       "fenced code stays unchanged",
			sourcePath: "docs/current.md",
			content:    "```md\n[x](../Note.md)\n```",
			want:       "```md\n[x](../Note.md)\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformer.TransformContent(tt.sourcePath, page, tt.content, wiki)
			if err != nil {
				t.Fatalf("TransformContent err: %v", err)
			}
			if got != tt.want {
				t.Fatalf("TransformContent = %q, want %q", got, tt.want)
			}
		})
	}
}
