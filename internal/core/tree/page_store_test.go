package tree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPageStore_CreatePage(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	root := &PageNode{
		ID:       "root",
		Title:    "Root",
		Slug:     "root",
		Children: []*PageNode{},
	}

	page := &PageNode{
		ID:     "page-1",
		Title:  "Hello World",
		Slug:   "hello-world",
		Parent: root,
	}

	err := store.CreatePage(root, page)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Prüfen, ob Datei existiert
	expectedFile := filepath.Join(tmpDir, "root", "hello-world.md")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected file was not created: %v", expectedFile)
	}

	// Optional: Inhalt checken
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "---\nleafwiki_id: page-1\n---\n# Hello World\n"
	if string(content) != expected {
		t.Errorf("Unexpected file content. Got: %q, Expected: %q", string(content), expected)
	}
}

func TestPageStore_CreatePage_WithFallbackCreatesIndex(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	// Simuliere vorhandene root.md-Datei (die in Folder + index.md migriert werden soll)
	rootFile := filepath.Join(tmpDir, "root.md")
	if err := os.WriteFile(rootFile, []byte("# Root File"), 0644); err != nil {
		t.Fatalf("Failed to create root.md: %v", err)
	}

	root := &PageNode{
		ID:       "root",
		Title:    "Root",
		Slug:     "root",
		Children: []*PageNode{},
	}

	page := &PageNode{
		ID:     "page-2",
		Title:  "Subpage",
		Slug:   "subpage",
		Parent: root,
	}

	err := store.CreatePage(root, page)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Erwartet: root/index.md existiert
	indexPath := filepath.Join(tmpDir, "root", "index.md")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Errorf("Expected fallback index.md file not found: %v", indexPath)
	}
}

func TestPageStore_CreatePage_DeepHierarchy(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	// Baue tiefe Baumstruktur: root → arch → project1
	root := &PageNode{
		ID:       "root",
		Title:    "Root",
		Slug:     "root",
		Children: []*PageNode{},
	}
	arch := &PageNode{
		ID:       "arch",
		Title:    "Architecture",
		Slug:     "architecture",
		Parent:   root,
		Children: []*PageNode{},
	}
	project := &PageNode{
		ID:       "project1",
		Title:    "Project One",
		Slug:     "project-one",
		Parent:   arch,
		Children: []*PageNode{},
	}
	page := &PageNode{
		ID:     "final",
		Title:  "Deep Content",
		Slug:   "deep-content",
		Parent: project,
	}

	// Füge Struktur hinzu (simulate parent nodes)
	root.Children = []*PageNode{arch}
	arch.Children = []*PageNode{project}
	project.Children = []*PageNode{}

	// Versuche, Page in tiefem Pfad anzulegen
	err := store.CreatePage(project, page)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// Prüfe, ob Datei wirklich existiert
	expectedPath := filepath.Join(tmpDir, "root", "architecture", "project-one", "deep-content.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file not found at deep path: %s", expectedPath)
	}
}

func TestPageStore_CreatePage_NilChecks(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	validParent := &PageNode{
		ID:       "root",
		Title:    "Root",
		Slug:     "root",
		Children: []*PageNode{},
	}

	// Fall 1: Parent ist nil
	err := store.CreatePage(nil, &PageNode{ID: "1", Title: "Page", Slug: "page"})
	if err == nil {
		t.Error("Expected error when parent is nil, got nil")
	}

	// Fall 2: Page ist nil
	err = store.CreatePage(validParent, nil)
	if err == nil {
		t.Error("Expected error when page is nil, got nil")
	}
}

func TestPageStore_DeletePage_File(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{
		ID:    "p1",
		Title: "Page",
		Slug:  "page",
	}

	// Erstelle Datei manuell
	filePath := filepath.Join(tmpDir, "page.md")
	if err := os.WriteFile(filePath, []byte("# Page"), 0644); err != nil {
		t.Fatalf("Failed to create page file: %v", err)
	}

	// DeletePage aufrufen
	if err := store.DeletePage(page); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	// Prüfen, ob Datei weg ist
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted: %v", filePath)
	}
}

func TestPageStore_DeletePage_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	// Seite mit Ordnerstruktur
	page := &PageNode{
		ID:    "p2",
		Title: "Folder Page",
		Slug:  "folder-page",
	}

	dirPath := filepath.Join(tmpDir, "folder-page")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create folder: %v", err)
	}

	// Simuliere index.md
	indexFile := filepath.Join(dirPath, "index.md")
	if err := os.WriteFile(indexFile, []byte("# Index"), 0644); err != nil {
		t.Fatalf("Failed to create index.md: %v", err)
	}

	// DeletePage aufrufen
	if err := store.DeletePage(page); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	// Ordner darf nicht mehr existieren
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Errorf("Expected folder to be deleted: %v", dirPath)
	}
}

func TestPageStore_DeletePage_NilEntry(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	err := store.DeletePage(nil)
	if err == nil {
		t.Errorf("Expected error when passing nil entry, got none")
	}
}

func TestPageStore_UpdatePage_ContentOnly(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{
		ID:    "p1",
		Title: "My Page",
		Slug:  "my-page",
	}

	filePath := filepath.Join(tmpDir, "my-page.md")
	if err := os.WriteFile(filePath, []byte("# Old Content"), 0644); err != nil {
		t.Fatalf("Failed to create page file: %v", err)
	}

	newContent := "# New Content"
	err := store.UpdatePage(page, "my-page", newContent)
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Could not read updated file: %v", err)
	}

	expectedNewContent := "---\nleafwiki_id: p1\nleafwiki_title: My Page\n---\n# New Content"

	if string(data) != expectedNewContent {
		t.Errorf("Expected content %q, got %q", expectedNewContent, string(data))
	}
}

func TestPageStore_UpdatePage_WithSlugChange_File(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{
		ID:    "p2",
		Title: "Old Page",
		Slug:  "old-page",
	}

	oldPath := filepath.Join(tmpDir, "old-page.md")
	if err := os.WriteFile(oldPath, []byte("# Old Page"), 0644); err != nil {
		t.Fatalf("Failed to create old page: %v", err)
	}

	newSlug := "new-page"
	err := store.UpdatePage(page, newSlug, "# Updated Content")
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	newPath := filepath.Join(tmpDir, "new-page.md")
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("Expected renamed file at: %v", newPath)
	}
}

func TestPageStore_UpdatePage_WithSlugChange_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{
		ID:    "p3",
		Title: "Old Dir",
		Slug:  "old-dir",
	}

	oldDir := filepath.Join(tmpDir, "old-dir")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatalf("Failed to create old directory: %v", err)
	}

	indexFile := filepath.Join(oldDir, "index.md")
	if err := os.WriteFile(indexFile, []byte("# Index"), 0644); err != nil {
		t.Fatalf("Failed to create index.md: %v", err)
	}

	newSlug := "new-dir"
	err := store.UpdatePage(page, newSlug, "# New Index")
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	newDir := filepath.Join(tmpDir, "new-dir")
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Errorf("Expected renamed directory: %v", newDir)
	}
}

func TestPageStore_UpdatePage_InvalidEntry(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	err := store.UpdatePage(nil, "slug", "content")
	if err == nil {
		t.Errorf("Expected error when updating nil entry, got none")
	}
}

func TestPageStore_UpdatePage_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{
		ID:    "p4",
		Title: "Ghost Page",
		Slug:  "ghost",
	}

	err := store.UpdatePage(page, "ghost", "# Nothing here")
	if err == nil {
		t.Errorf("Expected error when updating non-existent file, got none")
	}
}

func TestPageStore_MovePage_FileToFolder(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{ID: "1", Title: "Page A", Slug: "a"}
	pagePath := filepath.Join(tmpDir, "a.md")
	if err := os.WriteFile(pagePath, []byte("# Page A"), 0644); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	parent := &PageNode{ID: "root", Title: "Root", Slug: "root"}
	parentFile := filepath.Join(tmpDir, "root.md")
	if err := os.WriteFile(parentFile, []byte("# Root Page"), 0644); err != nil {
		t.Fatalf("Failed to create root.md: %v", err)
	}

	err := store.MovePage(page, parent)
	if err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}

	newPath := filepath.Join(tmpDir, "root", "a.md")
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("Expected file to be moved to: %v", newPath)
	}
}

func TestPageStore_MovePage_FolderToFolder(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	// Ordnerstruktur erstellen
	page := &PageNode{ID: "2", Title: "Docs", Slug: "docs"}
	pagePath := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(pagePath, 0755); err != nil {
		t.Fatalf("Failed to create source folder: %v", err)
	}

	// Zielordner
	target := &PageNode{ID: "root", Title: "Root", Slug: "root"}
	targetPath := filepath.Join(tmpDir, "root")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatalf("Failed to create target folder: %v", err)
	}

	err := store.MovePage(page, target)
	if err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}

	newPath := filepath.Join(targetPath, "docs")
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("Expected moved folder not found at: %v", newPath)
	}
}

func TestPageStore_MovePage_InvalidNilInput(t *testing.T) {
	store := NewPageStore(t.TempDir())

	err := store.MovePage(nil, nil)
	if err == nil {
		t.Errorf("Expected error on nil inputs, got none")
	}
}

func TestPageStore_MovePage_PreventCircularMove(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	// Erzeuge einfache Baumstruktur: root → parent → child
	root := &PageNode{
		ID:       "root",
		Title:    "Root",
		Slug:     "root",
		Children: []*PageNode{},
	}

	parent := &PageNode{
		ID:       "parent",
		Title:    "Parent",
		Slug:     "parent",
		Parent:   root,
		Children: []*PageNode{},
	}

	child := &PageNode{
		ID:       "child",
		Title:    "Child",
		Slug:     "child",
		Parent:   parent,
		Children: []*PageNode{},
	}

	root.Children = []*PageNode{parent}
	parent.Children = []*PageNode{child}

	// 🧪 Versuch: parent in child verschieben → sollte fehlschlagen (wenn später implementiert)
	err := store.MovePage(parent, child)

	// Aktuell kein Check implementiert → nur Hinweis
	if err == nil {
		t.Log("[TODO] Expected failure when moving parent into child (circular), but got none.")
		// Optionale manuelle Fehlerausgabe, damit es sichtbar bleibt
		t.Fail()
	}
}

func TestPageStore_ReadPageContent_File(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{
		ID:    "read1",
		Title: "Read Me",
		Slug:  "read-me",
	}

	filePath := filepath.Join(tmpDir, "read-me.md")
	expected := "# Hello from file"
	if err := os.WriteFile(filePath, []byte(expected), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	content, err := store.ReadPageContent(page)
	if err != nil {
		t.Fatalf("ReadPageContent failed: %v", err)
	}

	if content != expected {
		t.Errorf("Expected content %q, got %q", expected, content)
	}
}

func TestPageStore_ReadPageContent_Index(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{
		ID:    "read2",
		Title: "Folder Page",
		Slug:  "folder-page",
	}

	folder := filepath.Join(tmpDir, "folder-page")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatalf("Failed to create folder: %v", err)
	}

	indexPath := filepath.Join(folder, "index.md")
	expected := "# Hello from index"
	if err := os.WriteFile(indexPath, []byte(expected), 0644); err != nil {
		t.Fatalf("Failed to write index file: %v", err)
	}

	content, err := store.ReadPageContent(page)
	if err != nil {
		t.Fatalf("ReadPageContent failed: %v", err)
	}

	if content != expected {
		t.Errorf("Expected content %q, got %q", expected, content)
	}
}

func TestPageStore_ReadPageContent_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	page := &PageNode{
		ID:    "read3",
		Title: "Missing Page",
		Slug:  "missing",
	}

	_, err := store.ReadPageContent(page)
	if err == nil {
		t.Errorf("Expected error for missing file, got none")
	}
}

func TestPageStore_SaveAndLoadTree_AssignsParent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	tree := &PageNode{
		ID:    "root",
		Title: "Root",
		Slug:  "root",
		Children: []*PageNode{
			{
				ID:    "child-1",
				Title: "Child 1",
				Slug:  "child-1",
				Children: []*PageNode{
					{
						ID:    "grandchild-1",
						Title: "Grandchild 1",
						Slug:  "grandchild-1",
					},
				},
			},
		},
	}

	if err := store.SaveTree("tree.json", tree); err != nil {
		t.Fatalf("SaveTree failed: %v", err)
	}

	loaded, err := store.LoadTree("tree.json")
	if err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	child := loaded.Children[0]
	grandchild := child.Children[0]

	if child.Parent == nil || child.Parent.ID != loaded.ID {
		t.Errorf("Child node's parent not assigned correctly")
	}

	if grandchild.Parent == nil || grandchild.Parent.ID != child.ID {
		t.Errorf("Grandchild node's parent not assigned correctly")
	}
}

func TestPageStore_LoadTree_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	tree, err := store.LoadTree("nonexistent.json")
	if err != nil {
		t.Fatalf("Expected default tree, got error: %v", err)
	}

	if tree.ID != "root" {
		t.Errorf("Expected root ID, got %q", tree.ID)
	}
}

func TestPageStore_LoadTree_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	path := filepath.Join(tmpDir, "tree.json")
	if err := os.WriteFile(path, []byte("invalid-json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupt file: %v", err)
	}

	_, err := store.LoadTree("tree.json")
	if err == nil {
		t.Error("Expected error when loading invalid JSON, got none")
	}
}

func TestPageStore_getFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPageStore(tmpDir)

	// Case 1: .md file exists
	fileNode := &PageNode{
		ID:    "file1",
		Slug:  "page",
		Title: "Page",
	}
	filePath := filepath.Join(tmpDir, "page.md")
	if err := os.WriteFile(filePath, []byte("Content"), 0644); err != nil {
		t.Fatalf("Failed to create .md file: %v", err)
	}

	path, err := store.getFilePath(fileNode)
	if err != nil {
		t.Fatalf("Expected file path for .md file, got error: %v", err)
	}
	if path != filePath {
		t.Errorf("Unexpected path. Got: %s, Expected: %s", path, filePath)
	}

	// Case 2: Directory with index.md
	dirNode := &PageNode{
		ID:    "dir1",
		Slug:  "folder",
		Title: "Folder",
	}
	dirPath := filepath.Join(tmpDir, "folder")
	indexPath := filepath.Join(dirPath, "index.md")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create folder: %v", err)
	}
	if err := os.WriteFile(indexPath, []byte("Index content"), 0644); err != nil {
		t.Fatalf("Failed to write index.md: %v", err)
	}

	path, err = store.getFilePath(dirNode)
	if err != nil {
		t.Fatalf("Expected index.md path, got error: %v", err)
	}
	if path != indexPath {
		t.Errorf("Unexpected path. Got: %s, Expected: %s", path, indexPath)
	}

	// Case 3: Not found
	invalidNode := &PageNode{
		ID:    "missing",
		Slug:  "does-not-exist",
		Title: "Missing",
	}
	_, err = store.getFilePath(invalidNode)
	if err == nil {
		t.Errorf("Expected error for missing file, got nil")
	}
}
