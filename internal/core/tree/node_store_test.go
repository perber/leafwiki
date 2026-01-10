package tree

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

	if err := store.SaveTree("tree.json", tree); err != nil {
		t.Fatalf("SaveTree: %v", err)
	}

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

func TestNodeStore_SaveTree_NilTree_Error(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	if err := store.SaveTree("tree.json", nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestNodeStore_CreateSection_CreatesFolder_NoIndexByDefault(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "sec1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

	if err := store.CreateSection(root, sec); err != nil {
		t.Fatalf("CreateSection: %v", err)
	}

	// expected folder: <tmp>/root/docs
	dir := filepath.Join(tmp, "root", "docs")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("expected section folder at %s", dir)
	}

	// no index.md by default
	index := filepath.Join(dir, "index.md")
	if _, err := os.Stat(index); err == nil {
		t.Fatalf("did not expect index.md to exist by default: %s", index)
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

	fm, body, has, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if strings.TrimSpace(fm.LeafWikiID) != "p1" {
		t.Fatalf("expected leafwiki_id p1, got %q", fm.LeafWikiID)
	}
	// CreatePage setzt nur ID im FM, Title kommt in den Body als H1
	if !strings.Contains(body, "# Hello World") {
		t.Fatalf("expected H1 title in body, got: %q", body)
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
	fm, body, has, err := ParseFrontmatter(string(raw))
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

func TestNodeStore_ReadPageRaw_Section_NoIndex_ReturnsEmptyNil(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	sec := &PageNode{ID: "s1", Slug: "docs", Title: "Docs", Kind: NodeKindSection, Parent: root}

	// folder exists, but no index.md
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

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "P", Kind: NodeKindPage, Parent: root}

	_, err := store.ReadPageRaw(page)
	if err == nil {
		t.Fatalf("expected DriftError")
	}
}

func TestNodeStore_SyncFrontmatterIfExists_Page_UpdatesOrAddsFM(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	root := &PageNode{ID: "root", Slug: "root", Title: "root", Kind: NodeKindSection}
	page := &PageNode{ID: "p1", Slug: "p", Title: "Title A", Kind: NodeKindPage, Parent: root}

	path := filepath.Join(tmp, "root", "p.md")

	// file without FM
	mustWriteFile(t, path, "# Body\nHello", 0o644)

	if err := store.SyncFrontmatterIfExists(page); err != nil {
		t.Fatalf("SyncFrontmatterIfExists: %v", err)
	}

	raw := string(mustRead(t, path))
	fm, body, has, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected fm after sync")
	}
	if fm.LeafWikiID != "p1" || fm.LeafWikiTitle != "Title A" {
		t.Fatalf("unexpected fm: %#v", fm)
	}
	if strings.TrimSpace(body) != "# Body\nHello" {
		t.Fatalf("body changed unexpectedly: %q", body)
	}

	// update title and id
	page.Title = "Title B"
	page.ID = "p1b"
	if err := store.SyncFrontmatterIfExists(page); err != nil {
		t.Fatalf("SyncFrontmatterIfExists(update): %v", err)
	}
	raw2 := string(mustRead(t, path))
	fm2, body2, has2, err := ParseFrontmatter(raw2)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has2 || fm2.LeafWikiID != "p1b" || fm2.LeafWikiTitle != "Title B" {
		t.Fatalf("expected updated fm, got %#v", fm2)
	}
	if strings.TrimSpace(body2) != "# Body\nHello" {
		t.Fatalf("body changed unexpectedly on update: %q", body2)
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
	fm, _, has, err := ParseFrontmatter(raw)
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

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
