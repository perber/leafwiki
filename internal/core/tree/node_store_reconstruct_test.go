package tree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
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

func slugs(children []*PageNode) []string {
	out := make([]string, 0, len(children))
	for _, c := range children {
		out = append(out, c.Slug)
	}
	return out
}

// --- tests ---

func TestNodeStore_ReconstructTreeFromFS_EmptyStorage_ReturnsRoot(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	if tree == nil || tree.ID != "root" || tree.Kind != NodeKindSection {
		t.Fatalf("unexpected root: %#v", tree)
	}
	if tree.Parent != nil {
		t.Fatalf("expected root parent nil")
	}
	if len(tree.Children) != 0 {
		t.Fatalf("expected root to have no children, got %d", len(tree.Children))
	}
}

func TestNodeStore_ReconstructTreeFromFS_BuildsSectionsAndPages_SkipsIndexMdAsPage(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// FS layout:
	// <tmp>/docs/index.md (section content)
	// <tmp>/docs/intro.md (page)
	// <tmp>/readme.md (page at root)
	mustMkdir(t, filepath.Join(tmp, "root", "docs"))

	secIndex := `---
leafwiki_id: sec-docs
leafwiki_title: Documentation
---
# Section`
	mustWriteFile(t, filepath.Join(tmp, "root", "docs", "index.md"), secIndex, 0o644)

	pageIntro := `---
leafwiki_id: page-intro
leafwiki_title: Introduction
---
# Intro`
	mustWriteFile(t, filepath.Join(tmp, "root", "docs", "intro.md"), pageIntro, 0o644)

	rootPage := `---
leafwiki_id: page-readme
leafwiki_title: Readme
---
# Readme`
	mustWriteFile(t, filepath.Join(tmp, "root", "readme.md"), rootPage, 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	// root has: docs(section), readme(page)
	docs := findChildBySlug(t, tree, "docs")
	if docs.Kind != NodeKindSection {
		t.Fatalf("expected docs to be section, got %q", docs.Kind)
	}
	// section title/id from index frontmatter
	if docs.ID != "sec-docs" {
		t.Fatalf("expected docs.ID=sec-docs, got %q", docs.ID)
	}
	if docs.Title != "Documentation" {
		t.Fatalf("expected docs.Title=Documentation, got %q", docs.Title)
	}

	// ensure index.md wasn't turned into a page child
	for _, ch := range docs.Children {
		if ch.Slug == "index" {
			t.Fatalf("index.md must be skipped as page, but found slug index")
		}
	}

	intro := findChildBySlug(t, docs, "intro")
	if intro.Kind != NodeKindPage {
		t.Fatalf("expected intro to be page, got %q", intro.Kind)
	}
	// page title/id from frontmatter
	if intro.ID != "page-intro" {
		t.Fatalf("expected intro.ID=page-intro, got %q", intro.ID)
	}
	if intro.Title != "Introduction" {
		t.Fatalf("expected intro.Title=Introduction, got %q", intro.Title)
	}

	readme := findChildBySlug(t, tree, "readme")
	if readme.Kind != NodeKindPage {
		t.Fatalf("expected readme to be page, got %q", readme.Kind)
	}
	if readme.ID != "page-readme" {
		t.Fatalf("expected readme.ID=page-readme, got %q", readme.ID)
	}
	if readme.Title != "Readme" {
		t.Fatalf("expected readme.Title=Readme, got %q", readme.Title)
	}

	// parent pointers
	if docs.Parent == nil || docs.Parent.ID != "root" {
		t.Fatalf("expected docs parent root, got %#v", docs.Parent)
	}
	if intro.Parent == nil || intro.Parent.ID != docs.ID {
		t.Fatalf("expected intro parent docs, got %#v", intro.Parent)
	}
}

func TestNodeStore_ReconstructTreeFromFS_SectionWithoutIndex_UsesDirNameAsTitleAndMaterializesIndex(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustMkdir(t, filepath.Join(tmp, "root", "emptysec"))

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	sec := findChildBySlug(t, tree, "emptysec")
	if sec.Kind != NodeKindSection {
		t.Fatalf("expected section, got %q", sec.Kind)
	}
	if sec.Title != "emptysec" {
		t.Fatalf("expected title=emptysec, got %q", sec.Title)
	}
	if strings.TrimSpace(sec.ID) == "" {
		t.Fatalf("expected some generated id, got empty")
	}

	indexPath := filepath.Join(tmp, "root", "emptysec", "index.md")
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("expected reconstruct to materialize missing index.md: %v", err)
	}
	fm, body, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter in materialized index")
	}
	if fm.LeafWikiID != sec.ID || fm.LeafWikiTitle != sec.Title {
		t.Fatalf("unexpected frontmatter in materialized index: %#v", fm)
	}
	if strings.TrimSpace(body) != "" {
		t.Fatalf("expected empty body in materialized index, got %q", body)
	}
}

func TestNodeStore_ReconstructTreeFromFS_PageWithoutFrontmatter_FallsBackToHeadlineTitle(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// FS: <tmp>/plain.md (no fm)
	mustWriteFile(t, filepath.Join(tmp, "root", "plain.md"), "# hello\n", 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	p := findChildBySlug(t, tree, "plain")
	if p.Kind != NodeKindPage {
		t.Fatalf("expected page, got %q", p.Kind)
	}

	// title fallback should be headline
	if p.Title != "hello" {
		t.Fatalf("expected title fallback to slug 'plain', got %q", p.Title)
	}
	if strings.TrimSpace(p.ID) == "" {
		// should still have generated id (unless you later decide to keep empty)
		t.Fatalf("expected generated id, got empty")
	}
}

func TestNodeStore_ReconstructTreeFromFS_PositionsAreContiguous(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// Create several files/dirs
	mustWriteFile(t, filepath.Join(tmp, "root", "b.md"), "# b", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "a.md"), "# a", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "zsec"))

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	// Positions should be 0..n-1 regardless of order
	seen := make([]int, 0, len(tree.Children))
	for _, ch := range tree.Children {
		seen = append(seen, ch.Position)
	}
	sort.Ints(seen)
	for i := range seen {
		if seen[i] != i {
			t.Fatalf("expected contiguous positions 0..%d, got %v (slugs=%v)", len(seen)-1, seen, slugs(tree.Children))
		}
	}
}

func TestNodeStore_ReconstructTreeFromFS_OrderFileOverridesDefaultOrder(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustWriteFile(t, filepath.Join(tmp, "root", "a.md"), "---\nleafwiki_id: id-a\n---\n# A", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "b.md"), "---\nleafwiki_id: id-b\n---\n# B", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "c.md"), "---\nleafwiki_id: id-c\n---\n# C", 0o644)

	orderRaw, err := json.Marshal(map[string][]string{
		"ordered_ids": {"id-c", "id-a"},
	})
	if err != nil {
		t.Fatalf("marshal order file: %v", err)
	}
	mustWriteFile(t, filepath.Join(tmp, "root", ".order.json"), string(orderRaw), 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	got := slugs(tree.Children)
	want := []string{"c", "a", "b"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected child order: got %v want %v", got, want)
	}

	for i, child := range tree.Children {
		if child.Position != i {
			t.Fatalf("expected child %q position %d, got %d", child.Slug, i, child.Position)
		}
	}
}

func TestNodeStore_ReconstructTreeFromFS_OrderFileIgnoresUnknownIDsAndKeepsRemainingStable(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustWriteFile(t, filepath.Join(tmp, "root", "a.md"), "---\nleafwiki_id: id-a\n---\n# A", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "b.md"), "---\nleafwiki_id: id-b\n---\n# B", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "c.md"), "---\nleafwiki_id: id-c\n---\n# C", 0o644)

	orderRaw, err := json.Marshal(map[string][]string{
		"ordered_ids": {"missing-id", "id-b"},
	})
	if err != nil {
		t.Fatalf("marshal order file: %v", err)
	}
	mustWriteFile(t, filepath.Join(tmp, "root", ".order.json"), string(orderRaw), 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	got := slugs(tree.Children)
	want := []string{"b", "a", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected child order: got %v want %v", got, want)
	}
}

func TestNodeStore_ReconstructTreeFromFS_ReturnsErrorOnDuplicateLeafWikiIDs(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustWriteFile(t, filepath.Join(tmp, "root", "a.md"), `---
leafwiki_id: dup-id
leafwiki_title: A
---
# A`, 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "b.md"), `---
leafwiki_id: dup-id
leafwiki_title: B
---
# B`, 0o644)

	_, err := store.ReconstructTreeFromFS()
	if err == nil {
		t.Fatalf("expected duplicate ID error")
	}
	if !strings.Contains(err.Error(), "duplicate leafwiki_id") {
		t.Fatalf("expected duplicate ID error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "dup-id") {
		t.Fatalf("expected duplicate ID to be mentioned, got: %v", err)
	}
}

func TestNodeStore_ReconstructTreeFromFS_ReturnsErrorOnCaseInsensitiveDuplicateSlugs(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustWriteFile(t, filepath.Join(tmp, "root", "abc.md"), "# lower", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "ABC.md"), "# upper", 0o644)

	_, err := store.ReconstructTreeFromFS()
	if err == nil {
		t.Fatalf("expected duplicate slug error")
	}
	if !strings.Contains(err.Error(), "duplicate slug") {
		t.Fatalf("expected duplicate slug error, got: %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "abc") {
		t.Fatalf("expected conflicting slug to be mentioned, got: %v", err)
	}
}

func TestNodeStore_ReconstructTreeFromFS_WritesIDsBackToFiles(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// Create files without leafwiki_id in frontmatter
	mustWriteFile(t, filepath.Join(tmp, "root", "no-id.md"), "# No ID", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "section"))
	mustWriteFile(t, filepath.Join(tmp, "root", "section", "index.md"), "# Section No ID", 0o644)

	// Run reconstruction
	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	// Get the page and section nodes
	page := findChildBySlug(t, tree, "no-id")
	section := findChildBySlug(t, tree, "section")

	// Verify that IDs were generated
	if page.ID == "" {
		t.Fatalf("expected page to have generated ID, got empty")
	}
	if section.ID == "" {
		t.Fatalf("expected section to have generated ID, got empty")
	}

	// Now reload the files and check that IDs were written back
	pageMd, err := markdown.LoadMarkdownFile(filepath.Join(tmp, "root", "no-id.md"))
	if err != nil {
		t.Fatalf("failed to reload page: %v", err)
	}
	if pageMd.GetFrontmatter().LeafWikiID != page.ID {
		t.Fatalf("expected page frontmatter ID=%q, got %q", page.ID, pageMd.GetFrontmatter().LeafWikiID)
	}

	sectionMd, err := markdown.LoadMarkdownFile(filepath.Join(tmp, "root", "section", "index.md"))
	if err != nil {
		t.Fatalf("failed to reload section index: %v", err)
	}
	if sectionMd.GetFrontmatter().LeafWikiID != section.ID {
		t.Fatalf("expected section frontmatter ID=%q, got %q", section.ID, sectionMd.GetFrontmatter().LeafWikiID)
	}

	// Run reconstruction again and verify IDs are stable (deterministic)
	tree2, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("second ReconstructTreeFromFS: %v", err)
	}

	page2 := findChildBySlug(t, tree2, "no-id")
	section2 := findChildBySlug(t, tree2, "section")

	if page2.ID != page.ID {
		t.Fatalf("expected deterministic page ID on second run: first=%q, second=%q", page.ID, page2.ID)
	}
	if section2.ID != section.ID {
		t.Fatalf("expected deterministic section ID on second run: first=%q, second=%q", section.ID, section2.ID)
	}
}

func TestNodeStore_ReconstructTreeFromFS_SkipsInvalidSlugs(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// Create files and directories with invalid slug names
	mustWriteFile(t, filepath.Join(tmp, "root", "Valid Page.md"), "# Valid", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "UPPERCASE.md"), "# Upper", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "Valid Section"))
	mustWriteFile(t, filepath.Join(tmp, "root", "Valid Section", "index.md"), "# Section", 0o644)

	// Create a valid file to ensure the test still works
	mustWriteFile(t, filepath.Join(tmp, "root", "valid.md"), "# Valid", 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	// The valid file should be present with normalized slug
	findChildBySlug(t, tree, "valid")

	findChildBySlug(t, tree, "UPPERCASE")

	if len(tree.Children) != 2 {
		t.Fatalf("expected only invalid names with spaces to be skipped, got %v", slugs(tree.Children))
	}
}

func TestNodeStore_ReconstructTreeFromFS_PreservesMixedCaseSlugNames(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustWriteFile(t, filepath.Join(tmp, "root", "ABCD-efg.md"), "# Mixed Case", 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	findChildBySlug(t, tree, "ABCD-efg")
}
func TestNodeStore_ReconstructTreeFromFS_ReadsMetadataFromFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	mustWriteFile(t, filepath.Join(tmp, "root", "page.md"), `---
leafwiki_id: page-1
leafwiki_title: Page One
leafwiki_created_at: 2026-03-21T10:15:30Z
leafwiki_updated_at: 2026-03-21T11:16:31Z
leafwiki_creator_id: alice
leafwiki_last_author_id: bob
---
# Page One`, 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	page := findChildBySlug(t, tree, "page")
	if page.ID != "page-1" {
		t.Fatalf("expected page ID from frontmatter, got %q", page.ID)
	}
	if got := page.Metadata.CreatedAt.UTC().Format(time.RFC3339); got != "2026-03-21T10:15:30Z" {
		t.Fatalf("expected created_at from frontmatter, got %q", got)
	}
	if got := page.Metadata.UpdatedAt.UTC().Format(time.RFC3339); got != "2026-03-21T11:16:31Z" {
		t.Fatalf("expected updated_at from frontmatter, got %q", got)
	}
	if page.Metadata.CreatorID != "alice" || page.Metadata.LastAuthorID != "bob" {
		t.Fatalf("expected author metadata from frontmatter, got %#v", page.Metadata)
	}
}

func TestNodeStore_ReconstructTreeFromFS_MissingMetadataFallsBackToMtimeAndSystem(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	pagePath := filepath.Join(tmp, "root", "page.md")
	mustWriteFile(t, pagePath, `# Page One`, 0o644)

	wantTime := time.Date(2026, time.March, 21, 12, 34, 56, 0, time.UTC)
	if err := os.Chtimes(pagePath, wantTime, wantTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	page := findChildBySlug(t, tree, "page")
	if strings.TrimSpace(page.ID) == "" {
		t.Fatalf("expected generated ID")
	}
	if got := page.Metadata.CreatedAt.UTC().Format(time.RFC3339); got != wantTime.Format(time.RFC3339) {
		t.Fatalf("expected created_at fallback from mtime, got %q", got)
	}
	if got := page.Metadata.UpdatedAt.UTC().Format(time.RFC3339); got != wantTime.Format(time.RFC3339) {
		t.Fatalf("expected updated_at fallback from mtime, got %q", got)
	}
	if page.Metadata.CreatorID != reconstructSystemUserID || page.Metadata.LastAuthorID != reconstructSystemUserID {
		t.Fatalf("expected system user fallback, got %#v", page.Metadata)
	}

	mdFile, err := markdown.LoadMarkdownFile(pagePath)
	if err != nil {
		t.Fatalf("LoadMarkdownFile: %v", err)
	}
	fm := mdFile.GetFrontmatter()
	if fm.LeafWikiID != page.ID {
		t.Fatalf("expected generated ID to be written back, got %q want %q", fm.LeafWikiID, page.ID)
	}
	if fm.LeafWikiCreatedAt != "" || fm.LeafWikiUpdatedAt != "" || fm.LeafWikiCreatorID != "" || fm.LeafWikiLastAuthorID != "" {
		t.Fatalf("expected reconstruct fallback metadata to stay out of frontmatter during read-only reconstruct, got %#v", fm)
	}
}

func TestNodeStore_ReconstructTreeFromFS_InvalidMetadataTimestampFallsBackToMtime(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	pagePath := filepath.Join(tmp, "root", "page.md")
	mustWriteFile(t, pagePath, `---
leafwiki_id: page-1
leafwiki_title: Page One
leafwiki_created_at: not-a-timestamp
leafwiki_updated_at: 2026-03-21T11:16:31Z
leafwiki_creator_id: alice
leafwiki_last_author_id: bob
---
# Page One`, 0o644)

	wantTime := time.Date(2026, time.March, 21, 12, 34, 56, 0, time.UTC)
	if err := os.Chtimes(pagePath, wantTime, wantTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	page := findChildBySlug(t, tree, "page")
	if got := page.Metadata.CreatedAt.UTC().Format(time.RFC3339); got != wantTime.Format(time.RFC3339) {
		t.Fatalf("expected invalid created_at to fall back to mtime, got %q", got)
	}
	if got := page.Metadata.UpdatedAt.UTC().Format(time.RFC3339); got != "2026-03-21T11:16:31Z" {
		t.Fatalf("expected valid updated_at to be preserved, got %q", got)
	}
	if page.Metadata.CreatorID != "alice" || page.Metadata.LastAuthorID != "bob" {
		t.Fatalf("expected author metadata to be preserved, got %#v", page.Metadata)
	}
}
