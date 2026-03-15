package tree

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/markdown"
)

func mustWriteFile(t *testing.T, path string, data string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), perm); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func newRoot() *PageNode {
	return &PageNode{
		ID:    "root",
		Slug:  "root",
		Title: "root",
		Kind:  NodeKindSection,
	}
}

func newSection(id, slug, title string, parent *PageNode) *PageNode {
	return &PageNode{
		ID:     id,
		Slug:   slug,
		Title:  title,
		Kind:   NodeKindSection,
		Parent: parent,
	}
}

func newPage(id, slug, title string, parent *PageNode) *PageNode {
	return &PageNode{
		ID:     id,
		Slug:   slug,
		Title:  title,
		Kind:   NodeKindPage,
		Parent: parent,
	}
}

func TestNodeStore_CreateSection_CreatesFolderAndIndex(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	sec := newSection("sec1", "docs", "Docs", root)

	if err := store.CreateSection(root, sec); err != nil {
		t.Fatalf("CreateSection: %v", err)
	}

	dir := filepath.Join(tmp, "root", "docs")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("expected section folder at %s", dir)
	}

	index := filepath.Join(dir, "index.md")
	raw, err := os.ReadFile(index)
	if err != nil {
		t.Fatalf("expected index.md to exist: %v", err)
	}

	fm, body, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter in index.md")
	}
	if fm.LeafWikiID != "sec1" {
		t.Fatalf("expected leafwiki_id sec1, got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "Docs" {
		t.Fatalf("expected leafwiki_title Docs, got %q", fm.LeafWikiTitle)
	}
	if !strings.Contains(body, "# Docs") {
		t.Fatalf("expected H1 title in body, got %q", body)
	}
}

func TestNodeStore_CreateSection_Guards(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	tests := []struct {
		name   string
		parent *PageNode
		entry  *PageNode
	}{
		{
			name:   "parent must be section",
			parent: &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindPage},
			entry:  &PageNode{ID: "sec1", Slug: "docs", Title: "Docs", Kind: NodeKindSection},
		},
		{
			name:   "entry must be section",
			parent: newRoot(),
			entry:  &PageNode{ID: "p1", Slug: "x", Title: "X", Kind: NodeKindPage},
		},
		{
			name:   "entry must not be nil",
			parent: newRoot(),
			entry:  nil,
		},
		{
			name:   "parent must not be nil",
			parent: nil,
			entry:  &PageNode{ID: "sec1", Slug: "docs", Title: "Docs", Kind: NodeKindSection},
		},
		{
			name:   "entry must not be root",
			parent: newRoot(),
			entry:  &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.CreateSection(tc.parent, tc.entry); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestNodeStore_CreatePage_CreatesMarkdownWithFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	page := newPage("p1", "hello", "Hello World", root)

	if err := store.CreatePage(root, page); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	path := filepath.Join(tmp, "root", "hello.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created page: %v", err)
	}

	fm, body, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if strings.TrimSpace(fm.LeafWikiID) != "p1" {
		t.Fatalf("expected leafwiki_id p1, got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "Hello World" {
		t.Fatalf("expected leafwiki_title Hello World, got %q", fm.LeafWikiTitle)
	}
	if !strings.Contains(body, "# Hello World") {
		t.Fatalf("expected H1 title in body, got: %q", body)
	}
}

func TestNodeStore_CreatePage_GuardsAndCollisions(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)
	root := newRoot()

	tests := []struct {
		name  string
		setup func(t *testing.T)
		entry *PageNode
	}{
		{
			name:  "entry must not be nil",
			entry: nil,
		},
		{
			name:  "entry must be page",
			entry: newSection("s1", "docs", "Docs", root),
		},
		{
			name: "slug collides with file",
			setup: func(t *testing.T) {
				mustWriteFile(t, filepath.Join(tmp, "root", "dup.md"), "x", 0o644)
			},
			entry: newPage("p1", "dup", "Dup", root),
		},
		{
			name: "slug collides with dir",
			setup: func(t *testing.T) {
				mustMkdir(t, filepath.Join(tmp, "root", "dupdir"))
			},
			entry: newPage("p2", "dupdir", "DupDir", root),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			if err := store.CreatePage(root, tc.entry); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}

	t.Run("parent must be section", func(t *testing.T) {
		parent := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindPage}
		entry := newPage("p3", "page", "Page", parent)

		if err := store.CreatePage(parent, entry); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("parent must not be nil", func(t *testing.T) {
		entry := &PageNode{ID: "p4", Slug: "page", Title: "Page", Kind: NodeKindPage}
		if err := store.CreatePage(nil, entry); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestNodeStore_UpsertContent_Page_CreatesOrUpdates_PreservesMode(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	page := newPage("p1", "p", "My Page", root)

	path := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, path, "# old", 0o600)

	if err := store.UpsertContent(page, "# new"); err != nil {
		t.Fatalf("UpsertContent: %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("expected perm 0600, got %o", st.Mode().Perm())
	}

	raw := string(mustRead(t, path))
	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if fm.LeafWikiID != "p1" {
		t.Fatalf("expected id p1, got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "My Page" {
		t.Fatalf("expected title My Page, got %q", fm.LeafWikiTitle)
	}
	if strings.TrimSpace(body) != "# new" {
		t.Fatalf("expected body '# new', got %q", body)
	}
}

func TestNodeStore_UpsertContent_Section_WritesIndexAndCreatesDir(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	sec := newSection("s1", "docs", "Docs", root)

	if err := store.UpsertContent(sec, "# docs"); err != nil {
		t.Fatalf("UpsertContent: %v", err)
	}

	index := filepath.Join(tmp, "root", "docs", "index.md")
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("expected index.md to exist: %v", err)
	}
}

func TestNodeStore_MoveNode_Page_MovesFileStrict(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	secA := newSection("a", "a", "A", root)
	secB := newSection("b", "b", "B", root)
	page := newPage("p1", "p", "P", secA)

	src := filepath.Join(tmp, "root", "a", "p.md")
	mustWriteFile(t, src, "# hi", 0o644)

	if err := store.MoveNode(page, secB); err != nil {
		t.Fatalf("MoveNode: %v", err)
	}

	dst := filepath.Join(tmp, "root", "b", "p.md")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected dest file: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected src removed")
	}
}

func TestNodeStore_MoveNode_DriftWhenMissingSource(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	sec := newSection("s", "s", "S", root)
	page := newPage("p1", "p", "P", sec)

	err := store.MoveNode(page, root)
	if err == nil {
		t.Fatalf("expected DriftError, got nil")
	}

	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("expected DriftError, got %T: %v", err, err)
	}
}

func TestNodeStore_MoveNode_Guards(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	pageParent := &PageNode{ID: "p", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}
	page := newPage("p1", "child", "Child", root)

	tests := []struct {
		name   string
		entry  *PageNode
		parent *PageNode
	}{
		{name: "entry required", entry: nil, parent: root},
		{name: "parent required", entry: page, parent: nil},
		{name: "cannot move root", entry: newRoot(), parent: root},
		{name: "parent must be section", entry: page, parent: pageParent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.MoveNode(tc.entry, tc.parent); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestNodeStore_DeletePage_RemovesFile_OrDriftIfMissing(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	page := newPage("p1", "p", "P", root)

	path := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, path, "# x", 0o644)

	if err := store.DeletePage(page); err != nil {
		t.Fatalf("DeletePage: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file deleted")
	}

	err := store.DeletePage(page)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
}

func TestNodeStore_DeletePage_Guards(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()

	tests := []struct {
		name  string
		entry *PageNode
	}{
		{name: "entry required", entry: nil},
		{name: "cannot delete root", entry: newRoot()},
		{name: "entry must be page", entry: newSection("s1", "docs", "Docs", root)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.DeletePage(tc.entry); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestNodeStore_DeleteSection_RemovesFolderRecursive_OrDriftIfMissing(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	sec := newSection("s1", "docs", "Docs", root)

	dir := filepath.Join(tmp, "root", "docs")
	mustMkdir(t, dir)
	mustWriteFile(t, filepath.Join(dir, "index.md"), "# hi", 0o644)
	mustWriteFile(t, filepath.Join(dir, "nested.txt"), "x", 0o644)

	if err := store.DeleteSection(sec); err != nil {
		t.Fatalf("DeleteSection: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected folder deleted")
	}

	err := store.DeleteSection(sec)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
}

func TestNodeStore_DeleteSection_Guards(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()

	tests := []struct {
		name  string
		entry *PageNode
	}{
		{name: "entry required", entry: nil},
		{name: "cannot delete root", entry: newRoot()},
		{name: "entry must be section", entry: newPage("p1", "p", "P", root)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.DeleteSection(tc.entry); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestNodeStore_RenameNode_PageAndSection(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()

	t.Run("page", func(t *testing.T) {
		page := newPage("p1", "old", "P", root)
		oldFile := filepath.Join(tmp, "root", "old.md")
		mustWriteFile(t, oldFile, "# x", 0o644)

		if err := store.RenameNode(page, "new"); err != nil {
			t.Fatalf("RenameNode(page): %v", err)
		}
		if _, err := os.Stat(filepath.Join(tmp, "root", "new.md")); err != nil {
			t.Fatalf("expected new page file")
		}
	})

	t.Run("section", func(t *testing.T) {
		sec := newSection("s1", "docs", "Docs", root)
		secDir := filepath.Join(tmp, "root", "docs")
		mustMkdir(t, secDir)
		mustWriteFile(t, filepath.Join(secDir, "index.md"), "# y", 0o644)

		if err := store.RenameNode(sec, "docs2"); err != nil {
			t.Fatalf("RenameNode(section): %v", err)
		}
		if st, err := os.Stat(filepath.Join(tmp, "root", "docs2")); err != nil || !st.IsDir() {
			t.Fatalf("expected renamed section dir")
		}
	})
}

func TestNodeStore_RenameNode_Guards(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()

	tests := []struct {
		name    string
		entry   *PageNode
		newSlug string
	}{
		{name: "entry required", entry: nil, newSlug: "x"},
		{name: "new slug required", entry: newPage("p1", "old", "P", root), newSlug: ""},
		{name: "cannot rename root", entry: newRoot(), newSlug: "x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.RenameNode(tc.entry, tc.newSlug); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestNodeStore_ReadPageRaw_Section_NoIndex_ReturnsEmptyNil(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	sec := newSection("s1", "docs", "Docs", root)

	mustMkdir(t, filepath.Join(tmp, "root", "docs"))

	raw, err := store.ReadPageRaw(sec)
	if err != nil {
		t.Fatalf("ReadPageRaw: %v", err)
	}
	if raw != "" {
		t.Fatalf("expected empty raw for section without index, got %q", raw)
	}
}

func TestNodeStore_ReadPageRaw_Page_Missing_IsDrift(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	page := newPage("p1", "p", "P", root)

	_, err := store.ReadPageRaw(page)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
}

func TestNodeStore_WriteNodeFrontmatter_PageAndSection(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()

	t.Run("page", func(t *testing.T) {
		page := newPage("p1", "p", "Title A", root)
		path := filepath.Join(tmp, "root", "p.md")
		mustWriteFile(t, path, "# Body\nHello", 0o644)

		if err := store.WriteNodeFrontmatter(page); err != nil {
			t.Fatalf("WriteNodeFrontmatter(page): %v", err)
		}

		raw := string(mustRead(t, path))
		fm, body, has, err := markdown.ParseFrontmatter(raw)
		if err != nil {
			t.Fatalf("ParseFrontmatter: %v", err)
		}
		if !has {
			t.Fatalf("expected frontmatter")
		}
		if fm.LeafWikiID != "p1" || fm.LeafWikiTitle != "Title A" {
			t.Fatalf("unexpected frontmatter: %#v", fm)
		}
		if strings.TrimSpace(body) != "# Body\nHello" {
			t.Fatalf("body changed unexpectedly: %q", body)
		}
	})

	t.Run("section creates index when missing", func(t *testing.T) {
		sec := newSection("s1", "docs", "Docs", root)

		if err := store.WriteNodeFrontmatter(sec); err != nil {
			t.Fatalf("WriteNodeFrontmatter(section): %v", err)
		}

		index := filepath.Join(tmp, "root", "docs", "index.md")
		raw := string(mustRead(t, index))
		fm, _, has, err := markdown.ParseFrontmatter(raw)
		if err != nil {
			t.Fatalf("ParseFrontmatter: %v", err)
		}
		if !has {
			t.Fatalf("expected frontmatter")
		}
		if fm.LeafWikiID != "s1" || fm.LeafWikiTitle != "Docs" {
			t.Fatalf("unexpected frontmatter: %#v", fm)
		}
	})
}

func TestNodeStore_WriteNodeFrontmatter_GuardsAndDrift(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	page := newPage("p1", "p", "P", root)

	t.Run("entry required", func(t *testing.T) {
		if err := store.WriteNodeFrontmatter(nil); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("root is noop", func(t *testing.T) {
		if err := store.WriteNodeFrontmatter(newRoot()); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("page missing file is drift", func(t *testing.T) {
		err := store.WriteNodeFrontmatter(page)
		if err == nil {
			t.Fatalf("expected DriftError")
		}
		var de *DriftError
		if !errors.As(err, &de) {
			t.Fatalf("expected DriftError, got %T: %v", err, err)
		}
	})
}

func TestNodeStore_ConvertNode_PageToSection_MovesToIndex(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()
	entry := newPage("p1", "p", "P", root)

	file := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, file, "# hi", 0o644)

	if err := store.ConvertNode(entry, NodeKindSection); err != nil {
		t.Fatalf("ConvertNode(page->section): %v", err)
	}

	index := filepath.Join(tmp, "root", "p", "index.md")
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("expected index at %s", index)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("expected old file removed")
	}
}

func TestNodeStore_ConvertNode_SectionToPage(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := newRoot()

	t.Run("rejects non-empty folder", func(t *testing.T) {
		entry := newSection("s1", "docs", "Docs", root)

		dir := filepath.Join(tmp, "root", "docs")
		mustMkdir(t, dir)
		mustWriteFile(t, filepath.Join(dir, "index.md"), "# idx", 0o644)
		mustWriteFile(t, filepath.Join(dir, "other.txt"), "nope", 0o644)

		err := store.ConvertNode(entry, NodeKindPage)
		if err == nil {
			t.Fatalf("expected ConvertNotAllowedError")
		}
		var cna *ConvertNotAllowedError
		if !errors.As(err, &cna) {
			t.Fatalf("expected ConvertNotAllowedError, got %T: %v", err, err)
		}
	})

	t.Run("with index moves and removes folder", func(t *testing.T) {
		tmp2 := t.TempDir()
		store2 := NewNodeStore(tmp2)
		root2 := newRoot()
		entry := newSection("s1", "docs", "Docs", root2)

		dir := filepath.Join(tmp2, "root", "docs")
		mustMkdir(t, dir)
		mustWriteFile(t, filepath.Join(dir, "index.md"), "# idx", 0o644)

		if err := store2.ConvertNode(entry, NodeKindPage); err != nil {
			t.Fatalf("ConvertNode(section->page): %v", err)
		}

		pageFile := filepath.Join(tmp2, "root", "docs.md")
		if _, err := os.Stat(pageFile); err != nil {
			t.Fatalf("expected page file: %v", err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("expected folder removed")
		}
	})

	t.Run("no index creates empty page with frontmatter", func(t *testing.T) {
		tmp2 := t.TempDir()
		store2 := NewNodeStore(tmp2)
		root2 := newRoot()
		entry := newSection("s1", "docs", "Docs", root2)

		dir := filepath.Join(tmp2, "root", "docs")
		mustMkdir(t, dir)

		if err := store2.ConvertNode(entry, NodeKindPage); err != nil {
			t.Fatalf("ConvertNode(section->page no index): %v", err)
		}

		pageFile := filepath.Join(tmp2, "root", "docs.md")
		raw := string(mustRead(t, pageFile))
		fm, _, has, err := markdown.ParseFrontmatter(raw)
		if err != nil {
			t.Fatalf("ParseFrontmatter: %v", err)
		}
		if !has || fm.LeafWikiID != "s1" || fm.LeafWikiTitle != "Docs" {
			t.Fatalf("unexpected fm: %#v", fm)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("expected folder removed")
		}
	})
}

func TestNodeStore_ConvertNode_Guards(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	tests := []struct {
		name   string
		entry  *PageNode
		target NodeKind
	}{
		{name: "entry required", entry: nil, target: NodeKindPage},
		{name: "unknown target kind", entry: newRoot(), target: NodeKind("weird")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.ConvertNode(tc.entry, tc.target); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}
