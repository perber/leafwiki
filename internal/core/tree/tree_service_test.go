package tree

import (
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

	_, err := service.CreatePage(nil, "Welcome", "welcome")
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
	_, err := service.CreatePage(nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("Failed to create parent page: %v", err)
	}

	// ID des Elternteils holen
	parent := service.GetTree().Children[0]

	// Jetzt Subpage erstellen
	_, err = service.CreatePage(&parent.ID, "Getting Started", "getting-started")
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
	_, err := service.CreatePage(&invalidID, "Broken", "broken")
	if err == nil {
		t.Errorf("Expected error for invalid parent ID, got none")
	}
}

func TestTreeService_UpdatePage_ContentAndSlug(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Seite anlegen
	_, err := service.CreatePage(nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := service.GetTree().Children[0]

	// Inhalt + Slug ändern
	newSlug := "documentation"
	newContent := "# Updated Docs"
	err = service.UpdatePage(page.ID, "Documentation", newSlug, newContent)
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
	if string(data) != newContent {
		t.Errorf("Expected content %q, got %q", newContent, string(data))
	}
}

func TestTreeService_UpdatePage_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Seite im Baum erzeugen, aber Datei nicht schreiben
	id := "ghost"
	page := &PageNode{
		ID:     id,
		Title:  "Ghost",
		Slug:   "ghost",
		Parent: service.tree,
	}
	service.tree.Children = append(service.tree.Children, page)

	// Versuch zu aktualisieren
	err := service.UpdatePage(id, "Still Ghost", "still-ghost", "# Boo")
	if err == nil {
		t.Error("Expected error when file does not exist")
	}
}

func TestTreeService_UpdatePage_InvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	err := service.UpdatePage("unknown", "Nope", "nope", "# nope")
	if err == nil {
		t.Error("Expected error for invalid ID, got none")
	}
}

func TestTreeService_DeletePage_Success(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Seite erstellen
	_, err := service.CreatePage(nil, "DeleteMe", "delete-me")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	page := service.GetTree().Children[0]

	// Löschen
	err = service.DeletePage(page.ID, false)
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
	_, err := service.CreatePage(nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	parent := service.GetTree().Children[0]

	_, err = service.CreatePage(&parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage (child) failed: %v", err)
	}

	// Versuch ohne Rekursion
	err = service.DeletePage(parent.ID, false)
	if err == nil {
		t.Error("Expected error when deleting parent with children without recursive flag")
	}
}

func TestTreeService_DeletePage_InvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	err := service.DeletePage("nonexistent", false)
	if err == nil {
		t.Error("Expected error for unknown ID")
	}
}

func TestTreeService_DeletePage_Recursive(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Parent → Child
	_, err := service.CreatePage(nil, "Parent", "parent")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	parent := service.GetTree().Children[0]

	_, err = service.CreatePage(&parent.ID, "Child", "child")
	if err != nil {
		t.Fatalf("CreatePage (child) failed: %v", err)
	}

	// Rekursiv löschen
	err = service.DeletePage(parent.ID, true)
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
	_, err := service.CreatePage(nil, "A", "a")
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	_, err = service.CreatePage(nil, "B", "b")
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}

	a := service.GetTree().Children[0]
	b := service.GetTree().Children[1]

	err = service.MovePage(a.ID, b.ID)
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
	_, err := service.CreatePage(nil, "Target", "target")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	target := service.GetTree().Children[0]

	// Versuch mit ungültiger ID
	err = service.MovePage("does-not-exist", target.ID)
	if err == nil {
		t.Error("Expected error for non-existent source page")
	}
}

func TestTreeService_MovePage_NonexistentTarget(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, err := service.CreatePage(nil, "Source", "source")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	source := service.GetTree().Children[0]

	err = service.MovePage(source.ID, "invalid-target-id")
	if err == nil {
		t.Error("Expected error for non-existent target")
	}
}

func TestTreeService_MovePage_SelfAsParent(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	_, err := service.CreatePage(nil, "Loop", "loop")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	node := service.GetTree().Children[0]

	err = service.MovePage(node.ID, node.ID)
	if err == nil {
		t.Error("Expected error when moving page into itself (if you later implement such protection)")
	}
}

func TestTreeService_FindPageByRoutePath_Success(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	// Tree: root → architecture → project-a → specs
	service.CreatePage(nil, "Architecture", "architecture")
	arch := service.GetTree().Children[0]

	service.CreatePage(&arch.ID, "Project A", "project-a")
	projectA := arch.Children[0]

	service.CreatePage(&projectA.ID, "Specs", "specs")

	// Datei anlegen
	specPath := filepath.Join(tmpDir, "root", "architecture", "project-a", "specs.md")
	os.WriteFile(specPath, []byte("# Project A Specs"), 0644)

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

	service.CreatePage(nil, "Top", "top")

	_, err := service.FindPageByRoutePath(service.GetTree().Children, "top/missing")
	if err == nil {
		t.Error("Expected error for non-existent nested path, got nil")
	}
}

func TestTreeService_FindPageByRoutePath_PartialMatch(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTreeService(tmpDir)
	_ = service.LoadTree()

	service.CreatePage(nil, "Docs", "docs")
	service.CreatePage(nil, "API", "api")

	_, err := service.FindPageByRoutePath(service.GetTree().Children, "docs/should-not-exist")
	if err == nil {
		t.Error("Expected error for unmatched subpath")
	}
}
