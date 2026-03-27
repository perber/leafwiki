package tree

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

func writeLegacyTreeJSON(t *testing.T, storageDir string, tree *PageNode) {
	t.Helper()
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal legacy tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "tree.json"), raw, 0o644); err != nil {
		t.Fatalf("write legacy tree: %v", err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func TestNodeStore_LoadTree_MissingFile_ReturnsDefaultRoot(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	tree, err := store.LoadTree("missing.json")
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	if tree == nil {
		t.Fatalf("expected tree, got nil")
	}
	if tree.ID != "root" || tree.Slug != "root" || tree.Title != "root" {
		t.Fatalf("unexpected default root: %#v", tree)
	}
	if tree.Kind != NodeKindSection {
		t.Fatalf("expected root kind %q, got %q", NodeKindSection, tree.Kind)
	}
	if tree.Parent != nil {
		t.Fatalf("expected root parent nil")
	}
	if len(tree.Children) != 0 {
		t.Fatalf("expected no children")
	}
}

func TestNodeStore_SaveTree_ThenLoadTree_AssignsParents(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	tree := &PageNode{
		ID:    "root",
		Slug:  "root",
		Title: "root",
		Kind:  NodeKindSection,
		Children: []*PageNode{
			{
				ID:    "s1",
				Slug:  "sec",
				Title: "Section",
				Kind:  NodeKindSection,
				Children: []*PageNode{
					{
						ID:    "p1",
						Slug:  "page",
						Title: "Page",
						Kind:  NodeKindPage,
					},
				},
			},
		},
	}

	writeLegacyTreeJSON(t, tmp, tree)

	loaded, err := store.LoadTree("tree.json")
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	sec := loaded.Children[0]
	p := sec.Children[0]

	if sec.Parent == nil || sec.Parent.ID != "root" {
		t.Fatalf("expected section parent root, got %#v", sec.Parent)
	}
	if p.Parent == nil || p.Parent.ID != "s1" {
		t.Fatalf("expected page parent s1, got %#v", p.Parent)
	}
}

func TestNodeStore_SaveChildOrder_Root_WritesOrderFile(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{
		ID:    "root",
		Slug:  "root",
		Title: "root",
		Kind:  NodeKindSection,
		Children: []*PageNode{
			{ID: "a"},
			{ID: "b"},
		},
	}

	if err := store.SaveChildOrder(root); err != nil {
		t.Fatalf("SaveChildOrder root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "root", ".order.json")); err != nil {
		t.Fatalf("expected root order file: %v", err)
	}
}

func TestNodeStore_SaveChildOrder_Page_ReturnsErrorWithoutCreatingDirectory(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "page1", Slug: "docs", Title: "Docs", Kind: NodeKindPage, Parent: root}

	err := store.SaveChildOrder(page)
	if err == nil {
		t.Fatalf("expected SaveChildOrder to reject page nodes")
	}
	var opErr *InvalidOpError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected InvalidOpError, got %T (%v)", err, err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "root", "docs")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no stray page directory, got err=%v", err)
	}
}

func TestNodeStore_CreateSection_CreatesFolderAndIndexWithFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{
		ID:     "sec1",
		Slug:   "docs",
		Title:  "Docs",
		Kind:   NodeKindSection,
		Parent: root,
		Metadata: PageMetadata{
			CreatedAt:    time.Date(2026, time.March, 22, 10, 15, 30, 0, time.UTC),
			UpdatedAt:    time.Date(2026, time.March, 22, 11, 16, 31, 0, time.UTC),
			CreatorID:    "alice",
			LastAuthorID: "bob",
		},
	}

	if err := store.CreateSection(root, sec); err != nil {
		t.Fatalf("CreateSection: %v", err)
	}

	// expected folder: <tmp>/root/docs
	dir := filepath.Join(tmp, "root", "docs")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("expected section folder at %s", dir)
	}

	index := filepath.Join(dir, "index.md")
	raw := string(mustRead(t, index))
	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter in section index")
	}
	if fm.LeafWikiID != "sec1" || fm.LeafWikiTitle != "Docs" {
		t.Fatalf("unexpected section frontmatter: %#v", fm)
	}
	if fm.LeafWikiCreatedAt != "2026-03-22T10:15:30Z" || fm.LeafWikiUpdatedAt != "2026-03-22T11:16:31Z" {
		t.Fatalf("unexpected section timestamp metadata: %#v", fm)
	}
	if fm.LeafWikiCreatorID != "alice" || fm.LeafWikiLastAuthorID != "bob" {
		t.Fatalf("unexpected section author metadata: %#v", fm)
	}
	if strings.TrimSpace(body) != "" {
		t.Fatalf("expected empty section body, got %q", body)
	}
}

func TestNodeStore_CreateSection_KindGuards(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	rootPageWrong := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindPage}
	sec := &PageNode{ID: "sec1", Slug: "docs", Title: "Docs", Kind: NodeKindSection}

	if err := store.CreateSection(rootPageWrong, sec); err == nil {
		t.Fatalf("expected error when parent is not a section")
	}

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	pageWrong := &PageNode{ID: "x", Slug: "x", Title: "X", Kind: NodeKindPage}
	if err := store.CreateSection(root, pageWrong); err == nil {
		t.Fatalf("expected error when new entry is not a section")
	}
}

func TestNodeStore_CreatePage_CreatesMarkdownWithFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "hello", Title: "Hello World", Kind: NodeKindPage, Parent: root}

	if err := store.CreatePage(root, page); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	p := filepath.Join(tmp, "root", "hello.md")
	raw, err := os.ReadFile(p)
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
		t.Fatalf("expected leafwiki_title 'Hello World', got %q", fm.LeafWikiTitle)
	}
	if !strings.Contains(body, "# Hello World") {
		t.Fatalf("expected H1 title in body, got: %q", body)
	}
}

func TestNodeStore_CreatePage_SetsLeafWikiTitleInitially(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "hello", Title: "Hello World", Kind: NodeKindPage, Parent: root}

	if err := store.CreatePage(root, page); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	raw := string(mustRead(t, filepath.Join(tmp, "root", "hello.md")))
	fm, _, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if fm.LeafWikiID != "p1" {
		t.Fatalf("expected leafwiki_id p1, got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "Hello World" {
		t.Fatalf("expected CreatePage to set leafwiki_title, got %q", fm.LeafWikiTitle)
	}
}

func TestNodeStore_CreatePage_RejectsCollision_FileOrDir(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}

	// collision as file
	mustWriteFile(t, filepath.Join(tmp, "root", "dup.md"), "x", 0o644)
	page := &PageNode{ID: "p1", Slug: "dup", Title: "Dup", Kind: NodeKindPage, Parent: root}
	if err := store.CreatePage(root, page); err == nil {
		t.Fatalf("expected PageAlreadyExistsError for existing file")
	}

	// collision as dir
	mustMkdir(t, filepath.Join(tmp, "root", "dupdir"))
	page2 := &PageNode{ID: "p2", Slug: "dupdir", Title: "DupDir", Kind: NodeKindPage, Parent: root}
	if err := store.CreatePage(root, page2); err == nil {
		t.Fatalf("expected PageAlreadyExistsError for existing dir")
	}
}

func TestNodeStore_UpsertContent_Page_CreatesOrUpdates_PreservesMode(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "My Page", Kind: NodeKindPage, Parent: root}

	// create with custom mode
	path := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, path, "# old", 0o600)

	if err := store.UpsertContent(page, "# new"); err != nil {
		t.Fatalf("UpsertContent: %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// permissions should stay (best-effort; Windows behaves differently sometimes)
	if runtime.GOOS != "windows" {
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("expected perm 0600, got %o", st.Mode().Perm())
		}
	}

	raw, _ := os.ReadFile(path)
	fm, body, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected FM to exist")
	}
	if fm.LeafWikiID != "p1" {
		t.Fatalf("expected id p1, got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "My Page" {
		t.Fatalf("expected title 'My Page', got %q", fm.LeafWikiTitle)
	}
	if strings.TrimSpace(body) != "# new" {
		t.Fatalf("expected body '# new', got %q", body)
	}
}

func TestNodeStore_UpsertContent_PreservesExistingCustomFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "My Page", Kind: NodeKindPage, Parent: root}

	path := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, path, `---
custom_key: keep-me
tags:
  - alpha
leafwiki_id: old-id
leafwiki_title: Old Title
---
# old
`, 0o644)

	if err := store.UpsertContent(page, "# new"); err != nil {
		t.Fatalf("UpsertContent: %v", err)
	}

	raw := string(mustRead(t, path))
	if !strings.Contains(raw, "custom_key: keep-me") {
		t.Fatalf("expected custom frontmatter to be preserved, got: %q", raw)
	}
	if !strings.Contains(raw, "- alpha") {
		t.Fatalf("expected custom list frontmatter to be preserved, got: %q", raw)
	}

	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected FM to exist")
	}
	if fm.LeafWikiID != "p1" {
		t.Fatalf("expected id p1, got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "My Page" {
		t.Fatalf("expected title 'My Page', got %q", fm.LeafWikiTitle)
	}
	if strings.TrimSpace(body) != "# new" {
		t.Fatalf("expected body '# new', got %q", body)
	}
}

func TestNodeStore_UpsertContent_RawFrontmatter_MergesExtrasIntoWrittenFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "My Page", Kind: NodeKindPage, Parent: root}

	if err := store.CreatePage(root, page); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	rawContent := "---\naliases:\n  - alpha\ncustom_key: keep-me\nleafwiki_id: source-id\nleafwiki_title: Source Title\ntitle: Imported Title\n---\n\n# Imported Title\nBody"
	if err := store.UpsertContent(page, rawContent); err != nil {
		t.Fatalf("UpsertContent: %v", err)
	}

	path := filepath.Join(tmp, "root", "p.md")
	raw := string(mustRead(t, path))
	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter in written file")
	}
	if body != "\n# Imported Title\nBody" {
		t.Fatalf("unexpected body: %q", body)
	}
	if got := fm.ExtraFields["custom_key"]; got != "keep-me" {
		t.Fatalf("expected custom_key to be preserved, got %#v", got)
	}
	if got := fm.ExtraFields["title"]; got != "Imported Title" {
		t.Fatalf("expected title to be preserved, got %#v", got)
	}
	aliases, ok := fm.ExtraFields["aliases"].([]interface{})
	if !ok || len(aliases) != 1 || aliases[0] != "alpha" {
		t.Fatalf("expected aliases to be preserved, got %#v", fm.ExtraFields["aliases"])
	}
	if strings.Contains(raw, "leafwiki_id: source-id") {
		t.Fatalf("expected source leafwiki_id to be dropped, got: %q", raw)
	}
	if strings.Contains(raw, "leafwiki_title: Source Title") {
		t.Fatalf("expected source leafwiki_title to be dropped, got: %q", raw)
	}
	if fm.LeafWikiID != "p1" {
		t.Fatalf("expected managed leafwiki_id, got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "My Page" {
		t.Fatalf("expected managed leafwiki_title, got %q", fm.LeafWikiTitle)
	}
}

func TestNodeStore_UpsertContent_Section_WritesIndexAndCreatesDir(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

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

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	secA := &PageNode{ID: "a", Slug: "a", Title: "A", Kind: NodeKindSection, Parent: root}
	secB := &PageNode{ID: "b", Slug: "b", Title: "B", Kind: NodeKindSection, Parent: root}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: secA}

	// create source file at old location (tree-based path)
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

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s", Slug: "s", Title: "S", Kind: NodeKindSection, Parent: root}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: sec}

	err := store.MoveNode(page, root)
	if err == nil {
		t.Fatalf("expected DriftError, got nil")
	}
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("expected DriftError, got %T: %v", err, err)
	}
}

func TestNodeStore_DeletePage_RemovesFile_OrDriftIfMissing(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}

	path := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, path, "# x", 0o644)

	if err := store.DeletePage(page); err != nil {
		t.Fatalf("DeletePage: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file deleted")
	}

	// delete again -> drift
	err := store.DeletePage(page)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
}

func TestNodeStore_DeleteSection_RemovesFolderRecursive_OrDriftIfMissing(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

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

func TestNodeStore_RenameNode_PageAndSection(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}

	// page rename
	page := &PageNode{ID: "p1", Slug: "old", Title: "P", Kind: NodeKindPage, Parent: root}
	oldFile := filepath.Join(tmp, "root", "old.md")
	mustWriteFile(t, oldFile, "# x", 0o644)

	if err := store.RenameNode(page, "new"); err != nil {
		t.Fatalf("RenameNode(page): %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "root", "new.md")); err != nil {
		t.Fatalf("expected new page file")
	}

	// section rename
	sec := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}
	secDir := filepath.Join(tmp, "root", "docs")
	mustMkdir(t, secDir)
	mustWriteFile(t, filepath.Join(secDir, "index.md"), "# y", 0o644)

	if err := store.RenameNode(sec, "docs2"); err != nil {
		t.Fatalf("RenameNode(section): %v", err)
	}
	if st, err := os.Stat(filepath.Join(tmp, "root", "docs2")); err != nil || !st.IsDir() {
		t.Fatalf("expected renamed section dir")
	}
}

func TestNodeStore_RenameNode_RejectsEmptySlugAndRoot(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "old", Title: "P", Kind: NodeKindPage, Parent: root}

	if err := store.RenameNode(page, "   "); err == nil {
		t.Fatalf("expected empty slug to be rejected")
	}
	if err := store.RenameNode(root, "new-root"); err == nil {
		t.Fatalf("expected root rename to be rejected")
	}
}

func TestNodeStore_RenameNode_ReturnsNilWhenSlugUnchanged(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "same", Title: "P", Kind: NodeKindPage, Parent: root}
	file := filepath.Join(tmp, "root", "same.md")
	mustWriteFile(t, file, "# x", 0o644)

	if err := store.RenameNode(page, "same"); err != nil {
		t.Fatalf("expected unchanged slug rename to be a no-op, got %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("expected original file to remain: %v", err)
	}
}

func TestNodeStore_RenameNode_RejectsDestinationCollision(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "old", Title: "P", Kind: NodeKindPage, Parent: root}
	mustWriteFile(t, filepath.Join(tmp, "root", "old.md"), "# x", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "new.md"), "# y", 0o644)

	err := store.RenameNode(page, "new")
	if err == nil {
		t.Fatalf("expected PageAlreadyExistsError")
	}
	var existsErr *PageAlreadyExistsError
	if !errors.As(err, &existsErr) {
		t.Fatalf("expected PageAlreadyExistsError, got %T: %v", err, err)
	}
}

func TestNodeStore_RenameNode_Page_DriftWhenSourceIsFolder(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "old", Title: "P", Kind: NodeKindPage, Parent: root}
	mustMkdir(t, filepath.Join(tmp, "root", "old.md"))

	err := store.RenameNode(page, "new")
	if err == nil {
		t.Fatalf("expected DriftError")
	}
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("expected DriftError, got %T: %v", err, err)
	}
}

func TestNodeStore_RenameNode_Section_DriftWhenSourceIsFile(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}
	mustWriteFile(t, filepath.Join(tmp, "root", "docs"), "not a dir", 0o644)

	err := store.RenameNode(sec, "docs2")
	if err == nil {
		t.Fatalf("expected DriftError")
	}
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("expected DriftError, got %T: %v", err, err)
	}
}

func TestNodeStore_RenameNode_RejectsUnknownKind(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "x1", Slug: "weird", Title: "Weird", Kind: NodeKind("mystery"), Parent: root}

	err := store.RenameNode(entry, "other")
	if err == nil {
		t.Fatalf("expected InvalidOpError")
	}
	var opErr *InvalidOpError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected InvalidOpError, got %T: %v", err, err)
	}
}

func TestNodeStore_ReadPageRaw_Section_NoIndex_ReturnsEmptyNil(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

	mustMkdir(t, filepath.Join(tmp, "root", "docs"))

	raw, err := store.ReadPageRaw(sec)
	if err != nil {
		t.Fatalf("ReadPageRaw: %v", err)
	}
	if raw != "" {
		t.Fatalf("expected empty raw for section without index, got %q", raw)
	}

	if _, err := os.Stat(filepath.Join(tmp, "root", "docs", "index.md")); err == nil {
		t.Fatalf("expected no index.md side effect on read")
	}
}

func TestNodeStore_ReadPageRaw_Page_Missing_IsDrift(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}

	_, err := store.ReadPageRaw(page)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
}

func TestNodeStore_ReadPageContent_StripsFrontmatterAndPreservesBody(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}

	path := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, path, `---
custom_key: keep-me
leafwiki_id: p1
leafwiki_title: Existing Title
---
# Body
Hello
`, 0o644)

	content, err := store.ReadPageContent(page)
	if err != nil {
		t.Fatalf("ReadPageContent: %v", err)
	}
	if content != `# Body
Hello
` {
		t.Fatalf("expected body without frontmatter, got %q", content)
	}
}

func TestNodeStore_ReadPageContent_InvalidFrontmatterReturnsRaw(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}

	path := filepath.Join(tmp, "root", "p.md")
	raw := `---
leafwiki_id: [broken
---
# Body
Hello
`
	mustWriteFile(t, path, raw, 0o644)

	content, err := store.ReadPageContent(page)
	if err == nil {
		t.Fatalf("expected parse error for invalid frontmatter")
	}
	if content != raw {
		t.Fatalf("expected raw content fallback on parse error, got %q", content)
	}
}

func TestNodeStore_SyncFrontmatterIfExists_Page_UpdatesOrAddsFM(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{
		ID:     "p1",
		Slug:   "p",
		Title:  "Title A",
		Kind:   NodeKindPage,
		Parent: root,
		Metadata: PageMetadata{
			CreatedAt:    time.Date(2026, time.March, 21, 10, 15, 30, 0, time.UTC),
			UpdatedAt:    time.Date(2026, time.March, 21, 11, 16, 31, 0, time.UTC),
			CreatorID:    "alice",
			LastAuthorID: "bob",
		},
	}

	path := filepath.Join(tmp, "root", "p.md")

	// file without FM
	mustWriteFile(t, path, "# Body\nHello", 0o644)

	if err := store.SyncFrontmatterIfExists(page); err != nil {
		t.Fatalf("SyncFrontmatterIfExists: %v", err)
	}

	raw := string(mustRead(t, path))
	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected fm after sync")
	}
	if fm.LeafWikiID != "p1" || fm.LeafWikiTitle != "Title A" {
		t.Fatalf("unexpected fm: %#v", fm)
	}
	if fm.LeafWikiCreatedAt != "2026-03-21T10:15:30Z" || fm.LeafWikiUpdatedAt != "2026-03-21T11:16:31Z" {
		t.Fatalf("unexpected timestamp metadata: %#v", fm)
	}
	if fm.LeafWikiCreatorID != "alice" || fm.LeafWikiLastAuthorID != "bob" {
		t.Fatalf("unexpected author metadata: %#v", fm)
	}
	if strings.TrimSpace(body) != "# Body\nHello" {
		t.Fatalf("body changed unexpectedly: %q", body)
	}

	// update title and id
	page.Title = "Title B"
	page.ID = "p1b"
	page.Metadata.UpdatedAt = time.Date(2026, time.March, 21, 12, 17, 32, 0, time.UTC)
	page.Metadata.LastAuthorID = "carol"
	if err := store.SyncFrontmatterIfExists(page); err != nil {
		t.Fatalf("SyncFrontmatterIfExists(update): %v", err)
	}
	raw2 := string(mustRead(t, path))
	fm2, body2, has2, err := markdown.ParseFrontmatter(raw2)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has2 || fm2.LeafWikiID != "p1b" || fm2.LeafWikiTitle != "Title B" {
		t.Fatalf("expected updated fm, got %#v", fm2)
	}
	if fm2.LeafWikiCreatedAt != "2026-03-21T10:15:30Z" || fm2.LeafWikiUpdatedAt != "2026-03-21T12:17:32Z" {
		t.Fatalf("expected updated timestamps in fm, got %#v", fm2)
	}
	if fm2.LeafWikiCreatorID != "alice" || fm2.LeafWikiLastAuthorID != "carol" {
		t.Fatalf("expected updated author metadata in fm, got %#v", fm2)
	}
	if strings.TrimSpace(body2) != "# Body\nHello" {
		t.Fatalf("body changed unexpectedly on update: %q", body2)
	}
}

func TestNodeStore_SyncFrontmatterIfExists_PreservesExistingCustomFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "Title A", Kind: NodeKindPage, Parent: root}

	path := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, path, `---
custom_key: keep-me
aliases:
  - one
leafwiki_id: old-id
leafwiki_title: Old Title
---
# Body
Hello
`, 0o644)

	if err := store.SyncFrontmatterIfExists(page); err != nil {
		t.Fatalf("SyncFrontmatterIfExists: %v", err)
	}

	raw := string(mustRead(t, path))
	if !strings.Contains(raw, "custom_key: keep-me") {
		t.Fatalf("expected custom frontmatter to be preserved, got: %q", raw)
	}
	if !strings.Contains(raw, "- one") {
		t.Fatalf("expected custom list frontmatter to be preserved, got: %q", raw)
	}

	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected FM to exist")
	}
	if fm.LeafWikiID != "p1" || fm.LeafWikiTitle != "Title A" {
		t.Fatalf("unexpected fm: %#v", fm)
	}
	if strings.TrimSpace(body) != `# Body
Hello` {
		t.Fatalf("body changed unexpectedly: %q", body)
	}
}

func TestNodeStore_SyncFrontmatterIfExists_Section_NoIndex_NoSideEffects(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

	// Do NOT create folder: sync must not mkdir via write-path; should return nil.
	if err := store.SyncFrontmatterIfExists(sec); err != nil {
		t.Fatalf("SyncFrontmatterIfExists(section): %v", err)
	}
	// Ensure no folder created implicitly
	if _, err := os.Stat(filepath.Join(tmp, "root", "docs")); err == nil {
		t.Fatalf("expected no side effects (folder created), but folder exists")
	}
}

func TestNodeStore_resolveNode_FileVsFolder(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}

	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}
	mustWriteFile(t, filepath.Join(tmp, "root", "p.md"), "# x", 0o644)

	r1, err := store.resolveNode(page)
	if err != nil {
		t.Fatalf("resolveNode(page): %v", err)
	}
	if r1.Kind != NodeKindPage || !r1.HasContent || !strings.HasSuffix(r1.FilePath, "p.md") {
		t.Fatalf("unexpected resolved: %#v", r1)
	}

	sec := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}
	secDir := filepath.Join(tmp, "root", "docs")
	mustMkdir(t, secDir)

	r2, err := store.resolveNode(sec)
	if err != nil {
		t.Fatalf("resolveNode(sec without index): %v", err)
	}
	if r2.Kind != NodeKindSection || r2.HasContent {
		t.Fatalf("expected section without content: %#v", r2)
	}

	mustWriteFile(t, filepath.Join(secDir, "index.md"), "# idx", 0o644)
	r3, err := store.resolveNode(sec)
	if err != nil {
		t.Fatalf("resolveNode(sec with index): %v", err)
	}
	if r3.Kind != NodeKindSection || !r3.HasContent || !strings.HasSuffix(r3.FilePath, "index.md") {
		t.Fatalf("unexpected resolved: %#v", r3)
	}
}

func TestNodeStore_ConvertNode_PageToSection_MovesToIndex(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}

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

func TestNodeStore_ConvertNode_PageToSection_PreservesExistingMetadataAndBody(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{
		ID:     "p1",
		Slug:   "p",
		Title:  "Section Title",
		Kind:   NodeKindPage,
		Parent: root,
		Metadata: PageMetadata{
			CreatedAt:    time.Date(2026, time.March, 22, 10, 15, 30, 0, time.UTC),
			UpdatedAt:    time.Date(2026, time.March, 22, 11, 16, 31, 0, time.UTC),
			CreatorID:    "alice",
			LastAuthorID: "bob",
		},
	}

	file := filepath.Join(tmp, "root", "p.md")
	mustWriteFile(t, file, `---
custom_key: keep-me
leafwiki_id: legacy-id
leafwiki_title: Legacy Title
---
# hi
`, 0o644)

	if err := store.ConvertNode(entry, NodeKindSection); err != nil {
		t.Fatalf("ConvertNode(page->section): %v", err)
	}

	index := filepath.Join(tmp, "root", "p", "index.md")
	raw := string(mustRead(t, index))
	if !strings.Contains(raw, "custom_key: keep-me") {
		t.Fatalf("expected custom frontmatter to be preserved, got %q", raw)
	}

	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after conversion")
	}
	if fm.LeafWikiID != "p1" || fm.LeafWikiTitle != "Section Title" {
		t.Fatalf("expected managed frontmatter from tree metadata, got %#v", fm)
	}
	if fm.LeafWikiCreatedAt != "2026-03-22T10:15:30Z" || fm.LeafWikiUpdatedAt != "2026-03-22T11:16:31Z" {
		t.Fatalf("expected timestamps from tree metadata, got %#v", fm)
	}
	if fm.LeafWikiCreatorID != "alice" || fm.LeafWikiLastAuthorID != "bob" {
		t.Fatalf("expected author metadata from tree metadata, got %#v", fm)
	}
	if strings.TrimSpace(body) != "# hi" {
		t.Fatalf("expected body to be preserved, got %q", body)
	}
}

func TestNodeStore_ConvertNode_SectionToPage_RejectsNonEmptyFolder(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

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
}

func TestNodeStore_ConvertNode_SectionToPage_WithIndex_MovesAndRemovesFolder(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

	dir := filepath.Join(tmp, "root", "docs")
	mustMkdir(t, dir)
	mustWriteFile(t, filepath.Join(dir, "index.md"), "# idx", 0o644)

	if err := store.ConvertNode(entry, NodeKindPage); err != nil {
		t.Fatalf("ConvertNode(section->page): %v", err)
	}

	pageFile := filepath.Join(tmp, "root", "docs.md")
	if _, err := os.Stat(pageFile); err != nil {
		t.Fatalf("expected page file: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected folder removed")
	}
}

func TestNodeStore_ConvertNode_SectionToPage_NoIndex_CreatesEmptyPageWithFM(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

	dir := filepath.Join(tmp, "root", "docs")
	mustMkdir(t, dir)
	// empty folder, no index.md

	if err := store.ConvertNode(entry, NodeKindPage); err != nil {
		t.Fatalf("ConvertNode(section->page no index): %v", err)
	}

	pageFile := filepath.Join(tmp, "root", "docs.md")
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
}

func TestNodeStore_ConvertNode_SectionToPage_WithOrderMetadata_PreservesIndexContent(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

	dir := filepath.Join(tmp, "root", "docs")
	mustMkdir(t, dir)
	indexPath := filepath.Join(dir, "index.md")
	mustWriteFile(t, indexPath, `---
leafwiki_id: existing
leafwiki_title: Existing
custom: keep
---
# idx
`, 0o644)
	mustWriteFile(t, filepath.Join(dir, orderFilename), `{"ordered_ids":[]}`, 0o644)

	if err := store.ConvertNode(entry, NodeKindPage); err != nil {
		t.Fatalf("ConvertNode(section->page with order metadata): %v", err)
	}

	pageFile := filepath.Join(tmp, "root", "docs.md")
	raw := string(mustRead(t, pageFile))
	if !strings.Contains(raw, "custom: keep") || !strings.Contains(raw, "# idx") {
		t.Fatalf("expected converted page to keep index content, got: %s", raw)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected folder removed")
	}
}

func TestNodeStore_MoveNode_Section_MovesFolderStrict(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	secA := &PageNode{ID: "a", Slug: "a", Title: "A", Kind: NodeKindSection, Parent: root}
	secB := &PageNode{ID: "b", Slug: "b", Title: "B", Kind: NodeKindSection, Parent: root}
	entry := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: secA}

	srcDir := filepath.Join(tmp, "root", "a", "docs")
	mustMkdir(t, srcDir)
	mustWriteFile(t, filepath.Join(srcDir, "index.md"), "# hi", 0o644)

	if err := store.MoveNode(entry, secB); err != nil {
		t.Fatalf("MoveNode(section): %v", err)
	}

	dstDir := filepath.Join(tmp, "root", "b", "docs")
	if st, err := os.Stat(dstDir); err != nil || !st.IsDir() {
		t.Fatalf("expected moved section dir, err=%v", err)
	}
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Fatalf("expected old section dir removed")
	}
}

func TestNodeStore_MoveNode_Page_DriftWhenSourceIsFolder(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s", Slug: "s", Title: "S", Kind: NodeKindSection, Parent: root}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: sec}

	mustMkdir(t, filepath.Join(tmp, "root", "s", "p.md"))

	err := store.MoveNode(page, root)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("expected DriftError, got %T: %v", err, err)
	}
}

func TestNodeStore_MoveNode_Section_DriftWhenSourceIsFile(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s", Slug: "s", Title: "S", Kind: NodeKindSection, Parent: root}
	entry := &PageNode{ID: "p1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: sec}

	mustWriteFile(t, filepath.Join(tmp, "root", "s", "docs"), "not a dir", 0o644)

	err := store.MoveNode(entry, root)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("expected DriftError, got %T: %v", err, err)
	}
}

func TestNodeStore_MoveNode_RejectsDestinationCollision(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	secA := &PageNode{ID: "a", Slug: "a", Title: "A", Kind: NodeKindSection, Parent: root}
	secB := &PageNode{ID: "b", Slug: "b", Title: "B", Kind: NodeKindSection, Parent: root}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: secA}

	src := filepath.Join(tmp, "root", "a", "p.md")
	dst := filepath.Join(tmp, "root", "b", "p.md")
	mustWriteFile(t, src, "# hi", 0o644)
	mustWriteFile(t, dst, "# existing", 0o644)

	err := store.MoveNode(page, secB)
	if err == nil {
		t.Fatalf("expected PageAlreadyExistsError")
	}
	var existsErr *PageAlreadyExistsError
	if !errors.As(err, &existsErr) {
		t.Fatalf("expected PageAlreadyExistsError, got %T: %v", err, err)
	}
}

func TestNodeStore_ConvertNode_PageToSection_CreatesIndexWhenPageMissing(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}

	if err := store.ConvertNode(entry, NodeKindSection); err != nil {
		t.Fatalf("ConvertNode(page->section missing page): %v", err)
	}

	index := filepath.Join(tmp, "root", "p", "index.md")
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("expected materialized section index: %v", err)
	}
}

func TestNodeStore_ConvertNode_RejectsUnknownTarget(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}

	err := store.ConvertNode(entry, NodeKind("weird"))
	if err == nil {
		t.Fatalf("expected InvalidOpError")
	}
	var opErr *InvalidOpError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected InvalidOpError, got %T: %v", err, err)
	}
}

func TestNodeStore_ConvertNode_SectionToPage_DriftWhenPathIsFile(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	entry := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

	mustWriteFile(t, filepath.Join(tmp, "root", "docs"), "not a dir", 0o644)

	err := store.ConvertNode(entry, NodeKindPage)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("expected DriftError, got %T: %v", err, err)
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
