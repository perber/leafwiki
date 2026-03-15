package tree

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func findChildBySlug(t *testing.T, parent *PageNode, slug string) *PageNode {
	t.Helper()
	for _, ch := range parent.Children {
		if ch.Slug == slug {
			return ch
		}
	}
	t.Fatalf("child with slug %q not found under %q", slug, parent.Slug)
	return nil
}

func childSlugs(children []*PageNode) []string {
	out := make([]string, 0, len(children))
	for _, c := range children {
		out = append(out, c.Slug)
	}
	return out
}

func assertRootNode(t *testing.T, tree *PageNode) {
	t.Helper()

	if tree == nil {
		t.Fatalf("expected root tree, got nil")
	}
	if tree.ID != "root" {
		t.Fatalf("unexpected root id: got %q", tree.ID)
	}
	if tree.Kind != NodeKindSection {
		t.Fatalf("unexpected root kind: got %q", tree.Kind)
	}
	if tree.Parent != nil {
		t.Fatalf("expected root parent nil")
	}
}

func assertNode(t *testing.T, n *PageNode, wantID, wantTitle string, wantKind NodeKind) {
	t.Helper()

	if n == nil {
		t.Fatalf("expected node, got nil")
	}
	if n.ID != wantID {
		t.Fatalf("unexpected id: want=%q got=%q", wantID, n.ID)
	}
	if n.Title != wantTitle {
		t.Fatalf("unexpected title: want=%q got=%q", wantTitle, n.Title)
	}
	if n.Kind != wantKind {
		t.Fatalf("unexpected kind: want=%q got=%q", wantKind, n.Kind)
	}
}

func assertSyntheticID(t *testing.T, n *PageNode) {
	t.Helper()

	if n == nil {
		t.Fatalf("expected node, got nil")
	}
	if strings.TrimSpace(n.ID) == "" {
		t.Fatalf("expected non-empty id")
	}
	if !strings.HasPrefix(n.ID, "missing-id:") {
		t.Fatalf("expected synthetic id with prefix %q, got %q", "missing-id:", n.ID)
	}
}

func TestNodeStore_ReconstructTreeFromFS_EmptyStorage_ReturnsRoot(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	assertRootNode(t, tree)

	if len(tree.Children) != 0 {
		t.Fatalf("expected root to have no children, got %d", len(tree.Children))
	}
}

func TestNodeStore_ReconstructTreeFromFS_BuildsSectionsAndPages(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustMkdir(t, filepath.Join(tmp, "root", "docs"))

	mustWriteFile(t, filepath.Join(tmp, "root", "docs", "index.md"), `---
leafwiki_id: sec-docs
leafwiki_title: Documentation
---
# Section`, 0o644)

	mustWriteFile(t, filepath.Join(tmp, "root", "docs", "intro.md"), `---
leafwiki_id: page-intro
leafwiki_title: Introduction
---
# Intro`, 0o644)

	mustWriteFile(t, filepath.Join(tmp, "root", "readme.md"), `---
leafwiki_id: page-readme
leafwiki_title: Readme
---
# Readme`, 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	assertRootNode(t, tree)

	if got, want := childSlugs(tree.Children), []string{"docs", "readme"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected root children: want=%v got=%v", want, got)
	}

	docs := findChildBySlug(t, tree, "docs")
	assertNode(t, docs, "sec-docs", "Documentation", NodeKindSection)

	for _, ch := range docs.Children {
		if ch.Slug == "index" {
			t.Fatalf("index.md must not appear as child page")
		}
	}

	intro := findChildBySlug(t, docs, "intro")
	assertNode(t, intro, "page-intro", "Introduction", NodeKindPage)

	readme := findChildBySlug(t, tree, "readme")
	assertNode(t, readme, "page-readme", "Readme", NodeKindPage)

	if docs.Parent == nil || docs.Parent.ID != "root" {
		t.Fatalf("expected docs parent=root, got %#v", docs.Parent)
	}
	if intro.Parent == nil || intro.Parent.ID != docs.ID {
		t.Fatalf("expected intro parent=%q, got %#v", docs.ID, intro.Parent)
	}
}

func TestNodeStore_ReconstructTreeFromFS_FallbacksAndRepairFlags(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(t *testing.T, root string)
		slug            string
		wantTitle       string
		wantKind        NodeKind
		wantRepair      bool
		wantSyntheticID bool
	}{
		{
			name: "section without index uses directory name and is marked for repair",
			setup: func(t *testing.T, root string) {
				mustMkdir(t, filepath.Join(root, "emptysec"))
			},
			slug:            "emptysec",
			wantTitle:       "emptysec",
			wantKind:        NodeKindSection,
			wantRepair:      true,
			wantSyntheticID: true,
		},
		{
			name: "page without frontmatter uses headline title and is marked for repair",
			setup: func(t *testing.T, root string) {
				mustWriteFile(t, filepath.Join(root, "plain.md"), "# hello\n", 0o644)
			},
			slug:            "plain",
			wantTitle:       "hello",
			wantKind:        NodeKindPage,
			wantRepair:      true,
			wantSyntheticID: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			store := NewNodeStore(tmp)
			root := filepath.Join(tmp, "root")

			tc.setup(t, root)

			tree, err := store.ReconstructTreeFromFS()
			if err != nil {
				t.Fatalf("ReconstructTreeFromFS: %v", err)
			}

			node := findChildBySlug(t, tree, tc.slug)

			if node.Title != tc.wantTitle {
				t.Fatalf("unexpected title: want=%q got=%q", tc.wantTitle, node.Title)
			}
			if node.Kind != tc.wantKind {
				t.Fatalf("unexpected kind: want=%q got=%q", tc.wantKind, node.Kind)
			}
			if node.RepairNeeded != tc.wantRepair {
				t.Fatalf("unexpected repairNeeded: want=%v got=%v", tc.wantRepair, node.RepairNeeded)
			}

			if tc.wantSyntheticID {
				assertSyntheticID(t, node)
			}
		})
	}
}

func TestNodeStore_ReconstructTreeFromFS_NormalizesSlugs(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustWriteFile(t, filepath.Join(tmp, "root", "Valid Page.md"), "# Valid", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "UPPERCASE.md"), "# Upper", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "Valid Section"))
	mustWriteFile(t, filepath.Join(tmp, "root", "Valid Section", "index.md"), "# Section", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "valid.md"), "# Valid", 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	for _, slug := range []string{"valid", "valid-page", "uppercase", "valid-section"} {
		findChildBySlug(t, tree, slug)
	}
}

func TestNodeStore_ReconstructTreeFromFS_UsesDeterministicOrderWithoutOrderJSON(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustWriteFile(t, filepath.Join(tmp, "root", "b.md"), "# b", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "a.md"), "# a", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "zsec"))

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	got := childSlugs(tree.Children)
	want := []string{"a", "b", "zsec"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected child order: want=%v got=%v", want, got)
	}
}
