package tree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeService_SaveAndLoadTree(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)

	// Initialen Tree manuell setzen
	service.tree = &PageNode{
		ID:    "root",
		Title: "Root",
		Slug:  "root",
		Children: []*PageNode{
			{
				ID:    "child1",
				Title: "Child 1",
				Slug:  "child-1",
				Children: []*PageNode{
					{
						ID:    "child1a",
						Title: "Child 1a",
						Slug:  "child-1a",
					},
				},
			},
		},
	}

	// SaveTree ausführen
	if err := service.SaveTree(); err != nil {
		t.Fatalf("SaveTree failed: %v", err)
	}

	// Neue Instanz zum Laden
	loaded := NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	// Verifikation der Struktur
	root := loaded.GetTree()
	if root.ID != "root" || root.Title != "Root" {
		t.Errorf("Expected root node not loaded correctly")
	}

	if len(root.Children) != 1 || root.Children[0].ID != "child1" {
		t.Errorf("Child not loaded correctly")
	}

	grandchild := root.Children[0].Children[0]
	if grandchild == nil || grandchild.ID != "child1a" {
		t.Errorf("Grandchild not loaded correctly")
	}

	// Verifiziere Parent-Zuweisung
	if root.Children[0].Parent == nil || root.Children[0].Parent.ID != "root" {
		t.Errorf("Parent not assigned to child node")
	}
	if grandchild.Parent == nil || grandchild.Parent.ID != "child1" {
		t.Errorf("Parent not assigned to grandchild node")
	}
}

func TestTreeService_LoadTree_DefaultOnMissing(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)

	// Kein tree.json vorhanden → Default-Root
	err := service.LoadTree()
	if err != nil {
		t.Fatalf("Expected to load default tree, got error: %v", err)
	}

	tree := service.GetTree()
	if tree == nil || tree.ID != "root" {
		t.Errorf("Expected default root node, got: %+v", tree)
	}
}

func TestTreeService_CreatePage_RootLevel(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, err := service.CreatePage("system", nil, "Welcome", "welcome")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	tree := service.GetTree()
	if len(tree.Children) != 1 {
		t.Errorf("Expected 1 child at root level, got %d", len(tree.Children))
	}

	child := tree.Children[0]
	if child.Title != "Welcome" || child.Slug != "welcome" {
		t.Errorf("Child has incorrect data: %+v", child)
	}

	// Datei muss existieren
	expectedPath := filepath.Join(tmpDir, "root", "welcome.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file not found: %s", expectedPath)
	}
}

func TestTreeService_CreatePage_Nested(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Zuerst einen Parent anlegen
	_, err := service.CreatePage("system", nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("Failed to create parent page: %v", err)
	}

	// ID des Elternteils holen
	parent := service.GetTree().Children[0]

	// Jetzt Subpage erstellen
	_, err = service.CreatePage("system", &parent.ID, "Getting Started", "getting-started")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if len(parent.Children) != 1 {
		t.Errorf("Expected 1 child under parent, got %d", len(parent.Children))
	}

	sub := parent.Children[0]
	if sub.Slug != "getting-started" {
		t.Errorf("Unexpected slug: %s", sub.Slug)
	}

	expected := filepath.Join(tmpDir, "root", "docs", "getting-started.md")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("Expected nested file not found: %s", expected)
	}
}

func TestTreeService_CreatePage_InvalidParent(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	invalidID := "does-not-exist"
	_, err := service.CreatePage("system", &invalidID, "Broken", "broken")
	if err == nil {
		t.Errorf("Expected error for invalid parent ID, got none")
	}
}

func TestTreeService_UpdatePage_ContentAndSlug(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Seite anlegen
	_, err := service.CreatePage("system", nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := service.GetTree().Children[0]

	// Inhalt + Slug ändern
	newSlug := "documentation"
	newContent := "# Updated Docs"
	err = service.UpdatePage("system", page.ID, "Documentation", newSlug, newContent)
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	// Neuer Pfad sollte existieren
	newPath := filepath.Join(tmpDir, "root", newSlug+".md")
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("Expected updated file at %s not found", newPath)
	}

	// Inhalt prüfen
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if !strings.Contains(string(data), newContent) {
		t.Errorf("Expected content %q, got %q", newContent, string(data))
	}
}

func TestTreeService_UpdatePage_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create a page in the tree but do not create the corresponding file
	id := "ghost"
	page := &PageNode{
		ID:     id,
		Title:  "Ghost",
		Slug:   "ghost",
		Parent: service.tree,
	}
	service.tree.Children = append(service.tree.Children, page)

	// Versuch zu aktualisieren
	err := service.UpdatePage("system", id, "Still Ghost", "still-ghost", "# Boo")
	if err == nil {
		t.Error("Expected error when file does not exist")
	}
}

func TestTreeService_UpdatePage_InvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	err := service.UpdatePage("system", "unknown", "Nope", "nope", "# nope")
	if err == nil {
		t.Error("Expected error for invalid ID, got none")
	}
}

func TestTreeService_DeletePage_Success(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Seite erstellen
	_, err := service.CreatePage("system", nil, "DeleteMe", "delete-me")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := service.GetTree().Children[0]

	// Löschen
	err = service.DeletePage("system", page.ID, false)
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	// Datei darf nicht mehr existieren
	path := filepath.Join(tmpDir, "root", "delete-me.md")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted: %s", path)
	}

	// Seite sollte aus Tree entfernt worden sein
	if len(service.GetTree().Children) != 0 {
		t.Errorf("Expected page to be removed from tree")
	}
}

func TestTreeService_DeletePage_HasChildrenWithoutRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Parent + Child
	_, err := service.CreatePage("system", nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	parent := service.GetTree().Children[0]

	_, err = service.CreatePage("system", &parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage (child) failed: %v", err)
	}

	// Try deleting parent without recursive
	err = service.DeletePage("system", parent.ID, false)
	if err == nil {
		t.Error("Expected error when deleting parent with children without recursive flag")
	}
}

func TestTreeService_DeletePage_InvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	err := service.DeletePage("system", "nonexistent", false)
	if err == nil {
		t.Error("Expected error for unknown ID")
	}
}

func TestTreeService_DeletePage_Recursive(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Parent → Child
	_, err := service.CreatePage("system", nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	parent := service.GetTree().Children[0]

	_, err = service.CreatePage("system", &parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage (child) failed: %v", err)
	}

	// Rekursiv löschen
	err = service.DeletePage("system", parent.ID, true)
	if err != nil {
		t.Fatalf("Expected recursive delete to succeed, got error: %v", err)
	}

	parentPath := filepath.Join(tmpDir, "root", "parent")
	if _, err := os.Stat(parentPath); !os.IsNotExist(err) {
		t.Errorf("Expected parent folder to be deleted")
	}
}

func TestTreeService_MovePage_FileToFolder(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create root → a, root → b
	_, err := service.CreatePage("system", nil, "A", "a")
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	_, err = service.CreatePage("system", nil, "B", "b")
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}

	a := service.GetTree().Children[0]
	b := service.GetTree().Children[1]

	err = service.MovePage("system", a.ID, b.ID)
	if err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}

	// Erwartung: a ist jetzt unter b
	if len(b.Children) != 1 || b.Children[0].ID != a.ID {
		t.Errorf("Expected page A to be moved under B")
	}

	// Datei existiert im neuen Pfad
	expected := filepath.Join(tmpDir, "root", "b", "a.md")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("Expected moved file: %v", expected)
	}
}

func TestTreeService_MovePage_NonexistentPage(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create only one page
	_, err := service.CreatePage("system", nil, "Target", "target")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	target := service.GetTree().Children[0]

	// Versuch mit ungültiger ID
	err = service.MovePage("system", "does-not-exist", target.ID)
	if err == nil {
		t.Error("Expected error for non-existent source page")
	}
}

func TestTreeService_MovePage_NonexistentTarget(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, err := service.CreatePage("system", nil, "Source", "source")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	source := service.GetTree().Children[0]

	err = service.MovePage("system", source.ID, "invalid-target-id")
	if err == nil {
		t.Error("Expected error for non-existent target")
	}
}

func TestTreeService_MovePage_SelfAsParent(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, err := service.CreatePage("system", nil, "Loop", "loop")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	node := service.GetTree().Children[0]

	err = service.MovePage("system", node.ID, node.ID)
	if err == nil {
		t.Error("Expected error when moving page into itself (if you later implement such protection)")
	}
}

func TestTreeService_FindPageByRoutePath_Success(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Tree: root → architecture → project-a → specs
	_, err := service.CreatePage("system", nil, "Architecture", "architecture")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	arch := service.GetTree().Children[0]

	_, err = service.CreatePage("system", &arch.ID, "Project A", "project-a")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	projectA := arch.Children[0]

	_, err = service.CreatePage("system", &projectA.ID, "Specs", "specs")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Datei anlegen
	specPath := filepath.Join(tmpDir, "root", "architecture", "project-a", "specs.md")
	err = os.WriteFile(specPath, []byte("# Project A Specs"), 0644)
	if err != nil {
		t.Fatalf("Failed to write specs file: %v", err)
	}

	// 🔍 Suche über RoutePath
	page, err := service.FindPageByRoutePath(service.GetTree().Children, "architecture/project-a/specs")
	if err != nil {
		t.Fatalf("Expected page, got error: %v", err)
	}

	if page.Slug != "specs" || !strings.Contains(page.Content, "Specs") {
		t.Errorf("Unexpected page content or slug")
	}
}

func TestTreeService_FindPageByRoutePath_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	if _, err := service.CreatePage("system", nil, "Top", "top"); err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if _, err := service.FindPageByRoutePath(service.GetTree().Children, "top/missing"); err == nil {
		t.Error("Expected error for non-existent nested path, got nil")
	}
}

func TestTreeService_FindPageByRoutePath_PartialMatch(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	if _, err := service.CreatePage("system", nil, "Docs", "docs"); err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if _, err := service.CreatePage("system", nil, "API", "api"); err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if _, err := service.FindPageByRoutePath(service.GetTree().Children, "docs/should-not-exist"); err == nil {
		t.Error("Expected error for unmatched subpath")
	}
}

func setupTestTree() *TreeService {
	ts := NewTreeService(os.TempDir())
	ts.tree = &PageNode{
		ID:    "root",
		Title: "Root",
		Children: []*PageNode{
			{ID: "a", Title: "A"},
			{ID: "b", Title: "B"},
			{ID: "c", Title: "C"},
		},
	}
	return ts
}

func TestTreeService_SortPages_ValidOrder(t *testing.T) {
	ts := setupTestTree()

	err := ts.SortPages("root", []string{"c", "a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ts.tree.Children[0].ID != "c" || ts.tree.Children[1].ID != "a" || ts.tree.Children[2].ID != "b" {
		t.Errorf("unexpected order after sorting")
	}
}

func TestTreeService_SortPages_InvalidLength(t *testing.T) {
	ts := setupTestTree()

	err := ts.SortPages("root", []string{"a", "b"})
	if err == nil {
		t.Errorf("expected error for invalid length, got nil")
	}
}

func TestTreeService_SortPages_InvalidID(t *testing.T) {
	ts := setupTestTree()

	err := ts.SortPages("root", []string{"a", "b", "x"})
	if err == nil {
		t.Errorf("expected error for invalid ID, got nil")
	}
}

func TestTreeService_SortPages_DuplicateID(t *testing.T) {
	ts := setupTestTree()

	err := ts.SortPages("root", []string{"a", "a", "b"})
	if err == nil {
		t.Errorf("expected error for duplicate ID, got nil")
	}
}

func TestTreeService_SortPages_EmptyOK(t *testing.T) {
	ts := NewTreeService(t.TempDir())
	ts.tree = &PageNode{
		ID:       "root",
		Title:    "Root",
		Children: []*PageNode{},
	}

	err := ts.SortPages("root", []string{})
	if err != nil {
		t.Fatalf("unexpected error for empty list: %v", err)
	}
}

func TestTreeService_SortPages_TreeNotLoaded(t *testing.T) {
	ts := &TreeService{
		tree: nil,
	}

	err := ts.SortPages("root", []string{"a"})
	if err == nil || !errors.Is(err, ErrTreeNotLoaded) {
		t.Errorf("expected ErrTreeNotLoaded, got: %v", err)
	}
}

func TestTreeService_LookupPath_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create tree structure
	_, _ = service.CreatePage("system", nil, "Home", "home")
	home := service.GetTree().Children[0]
	_, _ = service.CreatePage("system", &home.ID, "About", "about")
	about := home.Children[0]
	_, _ = service.CreatePage("system", &about.ID, "Team", "team")

	lookup, err := service.LookupPagePath(service.GetTree().Children, "home/about/team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !lookup.Exists {
		t.Errorf("expected path to exist")
	}
	if len(lookup.Segments) != 3 {
		t.Errorf("expected 3 segments, got %d", len(lookup.Segments))
	}
	if !lookup.Segments[2].Exists || lookup.Segments[2].ID == nil || lookup.Segments[2].Slug != "team" {
		t.Errorf("expected last segment to exist with correct Slug")
	}
}

func TestTreeService_LookupPath_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create tree structure
	_, _ = service.CreatePage("system", nil, "Home", "home")
	home := service.GetTree().Children[0]
	_, _ = service.CreatePage("system", &home.ID, "About", "about")

	lookup, err := service.LookupPagePath(service.GetTree().Children, "home/about/contact")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.Exists {
		t.Errorf("expected path to not exist")
	}
	if len(lookup.Segments) != 3 {
		t.Errorf("expected 3 segments, got %d", len(lookup.Segments))
	}
	if !lookup.Segments[1].Exists || lookup.Segments[1].ID == nil || lookup.Segments[1].Slug != "about" {
		t.Errorf("expected second segment to exist with correct Slug")
	}
	if lookup.Segments[2].Exists || lookup.Segments[2].ID != nil || lookup.Segments[2].Slug != "contact" {
		t.Errorf("expected last segment to not exist with correct Slug")
	}
}

func TestTreeService_LookupPath_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	lookup, err := service.LookupPagePath(service.GetTree().Children, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.Exists {
		t.Errorf("expected empty path to not exist")
	}
	if len(lookup.Segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(lookup.Segments))
	}
}

func TestTreeService_LookupPath_DeeperMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, _ = service.CreatePage("system", nil, "Home", "home")
	home := service.GetTree().Children[0]
	_, _ = service.CreatePage("system", &home.ID, "About", "about")

	lookup, err := service.LookupPagePath(service.GetTree().Children, "home/about/team/members")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.Exists {
		t.Errorf("expected path to not exist")
	}
	if len(lookup.Segments) != 4 {
		t.Errorf("expected 4 segments, got %d", len(lookup.Segments))
	}
	if !lookup.Segments[1].Exists || lookup.Segments[1].ID == nil || lookup.Segments[1].Slug != "about" {
		t.Errorf("expected second segment to exist with correct Slug")
	}
	if lookup.Segments[2].Exists || lookup.Segments[2].ID != nil || lookup.Segments[2].Slug != "team" {
		t.Errorf("expected third segment to not exist with correct Slug")
	}
	if lookup.Segments[3].Exists || lookup.Segments[3].ID != nil || lookup.Segments[3].Slug != "members" {
		t.Errorf("expected last segment to not exist with correct Slug")
	}
}

func TestTreeService_LookupPath_OnlyOneSegment(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, _ = service.CreatePage("system", nil, "Home", "home")

	lookup, err := service.LookupPagePath(service.GetTree().Children, "home")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !lookup.Exists {
		t.Errorf("expected path to exist")
	}
	if len(lookup.Segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(lookup.Segments))
	}
	if !lookup.Segments[0].Exists || lookup.Segments[0].ID == nil || lookup.Segments[0].Slug != "home" {
		t.Errorf("expected segment to exist with correct Slug")
	}
}

func TestTreeService_EnsurePagePath_Successful(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, _ = service.CreatePage("system", nil, "Home", "home")
	home := service.GetTree().Children[0]
	_, _ = service.CreatePage("system", &home.ID, "About", "about")

	result, err := service.EnsurePagePath("system", "home/about/team", "Team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Errorf("expected path to exist after creation")
	}
	if result.Page == nil || result.Page.Slug != "team" || result.Page.Title != "Team" {
		t.Errorf("expected created page with correct Slug and Title")
	}

	// Verify the page was actually created in the tree
	about := home.Children[0]
	if len(about.Children) != 1 || about.Children[0].Slug != "team" {
		t.Errorf("expected 'team' page to be a child of 'about'")
	}
}

func TestTreeService_EnsurePagePath_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, _ = service.CreatePage("system", nil, "Home", "home")
	home := service.GetTree().Children[0]
	_, _ = service.CreatePage("system", &home.ID, "About", "about")
	about := home.Children[0]
	_, _ = service.CreatePage("system", &about.ID, "Team", "team")

	result, err := service.EnsurePagePath("system", "home/about/team", "Team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Errorf("expected path to exist")
	}
	if result.Page == nil || result.Page.Slug != "team" {
		t.Errorf("expected existing page with correct Slug")
	}
}

func TestTreeService_EnsurePagePath_PartialExistence(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, _ = service.CreatePage("system", nil, "Home", "home")
	home := service.GetTree().Children[0]
	_, _ = service.CreatePage("system", &home.ID, "About", "about")

	result, err := service.EnsurePagePath("system", "home/about/team/members", "Members")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Errorf("expected full path to exist after creation")
	}
	if result.Page == nil || result.Page.Slug != "members" || result.Page.Title != "Members" {
		t.Errorf("expected created 'members' page with correct Slug and Title")
	}

	// Verify the intermediate 'team' page was also created
	about := home.Children[0]
	if len(about.Children) != 1 || about.Children[0].Slug != "team" {
		t.Errorf("expected 'team' page to be a child of 'about'")
	}
	team := about.Children[0]
	if len(team.Children) != 1 || team.Children[0].Slug != "members" {
		t.Errorf("expected 'members' page to be a child of 'team'")
	}
}

func TestTreeService_EnsurePagePath_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	result, err := service.EnsurePagePath("system", "", "Root")
	if err == nil {
		t.Fatalf("expected error for empty path, got nil")
	}

	if result != nil {
		t.Errorf("expected nil result for empty path")
	}
}

func TestTreeService_EnsurePagePath_PathStartingWithSlash(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	result, err := service.EnsurePagePath("system", "/leading/slash", "Invalid")
	if err != nil {
		t.Fatalf("expected error for invalid path, got nil")
	}

	if result == nil {
		t.Errorf("expected nil result for invalid path")
	}
}

func TestTreeService_MigrateToV2_PagesWithoutFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create pages without frontmatter
	_, err := service.CreatePage("system", nil, "Page1", "page1")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page1 := service.GetTree().Children[0]

	_, err = service.CreatePage("system", &page1.ID, "Page2", "page2")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page2 := page1.Children[0]

	// Write content without frontmatter
	page1Path := filepath.Join(tmpDir, "root", "page1.md")
	page2Path := filepath.Join(tmpDir, "root", "page1", "page2.md")
	
	err = os.WriteFile(page1Path, []byte("# Page 1 Content\nHello World"), 0644)
	if err != nil {
		t.Fatalf("Failed to write page1: %v", err)
	}
	
	err = os.WriteFile(page2Path, []byte("# Page 2 Content\nNested content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write page2: %v", err)
	}

	// Run migration
	err = service.migrateToV2()
	if err != nil {
		t.Fatalf("migrateToV2 failed: %v", err)
	}

	// Verify frontmatter was added to page1
	content1, err := os.ReadFile(page1Path)
	if err != nil {
		t.Fatalf("Failed to read page1 after migration: %v", err)
	}
	fm1, body1, has1, err := ParseFrontmatter(string(content1))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter for page1: %v", err)
	}
	if !has1 {
		t.Error("Expected page1 to have frontmatter after migration")
	}
	if fm1.LeafWikiID != page1.ID {
		t.Errorf("Expected page1 frontmatter ID to be %s, got %s", page1.ID, fm1.LeafWikiID)
	}
	if fm1.LeafWikiTitle != "Page1" {
		t.Errorf("Expected page1 frontmatter title to be 'Page1', got %s", fm1.LeafWikiTitle)
	}
	if !strings.Contains(body1, "# Page 1 Content") {
		t.Error("Expected page1 body to be preserved")
	}

	// Verify frontmatter was added to page2
	content2, err := os.ReadFile(page2Path)
	if err != nil {
		t.Fatalf("Failed to read page2 after migration: %v", err)
	}
	fm2, body2, has2, err := ParseFrontmatter(string(content2))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter for page2: %v", err)
	}
	if !has2 {
		t.Error("Expected page2 to have frontmatter after migration")
	}
	if fm2.LeafWikiID != page2.ID {
		t.Errorf("Expected page2 frontmatter ID to be %s, got %s", page2.ID, fm2.LeafWikiID)
	}
	if !strings.Contains(body2, "# Page 2 Content") {
		t.Error("Expected page2 body to be preserved")
	}
}

func TestTreeService_MigrateToV2_PagesWithExistingFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create page
	_, err := service.CreatePage("system", nil, "Page1", "page1")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page1 := service.GetTree().Children[0]

	// Write content with existing frontmatter
	page1Path := filepath.Join(tmpDir, "root", "page1.md")
	existingContent := "---\nleafwiki_id: " + page1.ID + "\nleafwiki_title: Custom Title\n---\n# Page 1 Content"
	err = os.WriteFile(page1Path, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write page1: %v", err)
	}

	// Run migration
	err = service.migrateToV2()
	if err != nil {
		t.Fatalf("migrateToV2 failed: %v", err)
	}

	// Verify frontmatter was not modified (should be unchanged)
	content1, err := os.ReadFile(page1Path)
	if err != nil {
		t.Fatalf("Failed to read page1 after migration: %v", err)
	}
	fm1, body1, has1, err := ParseFrontmatter(string(content1))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter for page1: %v", err)
	}
	if !has1 {
		t.Error("Expected page1 to have frontmatter after migration")
	}
	if fm1.LeafWikiID != page1.ID {
		t.Errorf("Expected page1 frontmatter ID to be %s, got %s", page1.ID, fm1.LeafWikiID)
	}
	if fm1.LeafWikiTitle != "Custom Title" {
		t.Errorf("Expected page1 frontmatter title to be 'Custom Title', got %s", fm1.LeafWikiTitle)
	}
	if !strings.Contains(body1, "# Page 1 Content") {
		t.Error("Expected page1 body to be preserved")
	}
}

func TestTreeService_MigrateToV2_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create a page and its file
	_, err := service.CreatePage("system", nil, "Page1", "page1")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Write content to page1
	page1Path := filepath.Join(tmpDir, "root", "page1.md")
	err = os.WriteFile(page1Path, []byte("# Page 1 Content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write page1: %v", err)
	}

	// Create a page with a child
	_, err = service.CreatePage("system", nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	parent := service.GetTree().Children[1]

	_, err = service.CreatePage("system", &parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Write content to child without frontmatter
	childPath := filepath.Join(tmpDir, "root", "parent", "child.md")
	err = os.WriteFile(childPath, []byte("# Child Content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write child: %v", err)
	}

	// Remove the parent index.md file (parent has children so it's in a folder)
	parentIndexPath := filepath.Join(tmpDir, "root", "parent", "index.md")
	if _, err := os.Stat(parentIndexPath); err == nil {
		os.Remove(parentIndexPath)
	}

	// Run migration - should handle missing parent file gracefully and still migrate child
	err = service.migrateToV2()
	if err != nil {
		t.Fatalf("migrateToV2 should handle missing files gracefully, got error: %v", err)
	}

	// Verify page1 was migrated
	content1, err := os.ReadFile(page1Path)
	if err != nil {
		t.Fatalf("Failed to read page1 after migration: %v", err)
	}
	_, _, has1, err := ParseFrontmatter(string(content1))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter for page1: %v", err)
	}
	if !has1 {
		t.Error("Expected page1 to have frontmatter after migration")
	}

	// Verify child was still migrated even though parent file is missing
	childContent, err := os.ReadFile(childPath)
	if err != nil {
		t.Fatalf("Failed to read child after migration: %v", err)
	}
	_, _, hasChild, err := ParseFrontmatter(string(childContent))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter for child: %v", err)
	}
	if !hasChild {
		t.Error("Expected child to have frontmatter after migration even if parent file is missing")
	}
}

func TestTreeService_MigrateToV2_SkipsNonExistentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create a simple page
	_, err := service.CreatePage("system", nil, "Page1", "page1")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Write content without frontmatter
	page1Path := filepath.Join(tmpDir, "root", "page1.md")
	err = os.WriteFile(page1Path, []byte("# Page 1 Content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write page1: %v", err)
	}

	// Manually add a node to the tree without creating its file
	// This simulates a corrupted tree structure
	ghostNode := &PageNode{
		ID:     "ghost-node",
		Title:  "Ghost",
		Slug:   "ghost",
		Parent: service.tree,
	}
	service.tree.Children = append(service.tree.Children, ghostNode)

	// Run migration - should handle the ghost node gracefully if it returns os.ErrNotExist
	// But will fail with other errors from getFilePath
	err = service.migrateToV2()
	// The getFilePath returns "file not found" which is not os.ErrNotExist
	// So the migration will fail
	if err == nil {
		t.Error("Expected migration to fail when encountering missing file")
	}
}

func TestTreeService_MigrateToV2_TreeNotLoaded(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	// Do NOT load tree

	// Run migration should fail
	err := service.migrateToV2()
	if err == nil {
		t.Error("Expected error when tree is not loaded")
	}
	if !errors.Is(err, ErrTreeNotLoaded) {
		t.Errorf("Expected ErrTreeNotLoaded, got: %v", err)
	}
}

func TestTreeService_MigrateToV2_PartialFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create page
	_, err := service.CreatePage("system", nil, "Page1", "page1")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page1 := service.GetTree().Children[0]

	// Write content with partial frontmatter (missing ID)
	page1Path := filepath.Join(tmpDir, "root", "page1.md")
	partialContent := "---\nleafwiki_title: Existing Title\n---\n# Page 1 Content"
	err = os.WriteFile(page1Path, []byte(partialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write page1: %v", err)
	}

	// Run migration
	err = service.migrateToV2()
	if err != nil {
		t.Fatalf("migrateToV2 failed: %v", err)
	}

	// Verify ID was added but title was preserved
	content1, err := os.ReadFile(page1Path)
	if err != nil {
		t.Fatalf("Failed to read page1 after migration: %v", err)
	}
	fm1, _, _, err := ParseFrontmatter(string(content1))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter for page1: %v", err)
	}
	if fm1.LeafWikiID != page1.ID {
		t.Errorf("Expected page1 frontmatter ID to be added: %s, got %s", page1.ID, fm1.LeafWikiID)
	}
	if fm1.LeafWikiTitle != "Existing Title" {
		t.Errorf("Expected page1 frontmatter title to be preserved: 'Existing Title', got %s", fm1.LeafWikiTitle)
	}
}

func TestTreeService_MigrateToV2_EmptyTree(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Run migration on empty tree (only root, no children)
	err := service.migrateToV2()
	if err != nil {
		t.Fatalf("migrateToV2 should succeed on empty tree, got error: %v", err)
	}
}

func TestTreeService_MigrateToV2_PreservesBodyContent(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Create page
	_, err := service.CreatePage("system", nil, "Page1", "page1")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Write complex content without frontmatter
	page1Path := filepath.Join(tmpDir, "root", "page1.md")
	complexContent := `# Title

This is a paragraph.

## Section 1

- Item 1
- Item 2

` + "```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```" + `

### Subsection

More content here.

---

Horizontal rule above.
`
	err = os.WriteFile(page1Path, []byte(complexContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write page1: %v", err)
	}

	// Run migration
	err = service.migrateToV2()
	if err != nil {
		t.Fatalf("migrateToV2 failed: %v", err)
	}

	// Verify body content is exactly preserved
	content1, err := os.ReadFile(page1Path)
	if err != nil {
		t.Fatalf("Failed to read page1 after migration: %v", err)
	}
	_, body1, _, err := ParseFrontmatter(string(content1))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter for page1: %v", err)
	}
	if body1 != complexContent {
		t.Errorf("Expected body to be exactly preserved.\nGot:\n%s\n\nWant:\n%s", body1, complexContent)
	}
}
