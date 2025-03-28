package wiki

import (
	"testing"
)

func setupTestWiki(t *testing.T) *Wiki {
	tempDir := t.TempDir()
	w, err := NewWiki(tempDir)
	if err != nil {
		t.Fatalf("Failed to create wiki: %v", err)
	}
	return w
}

func TestWiki_CreatePage_Root(t *testing.T) {
	w := setupTestWiki(t)

	page, err := w.CreatePage(nil, "Home")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if page.Title != "Home" {
		t.Errorf("Expected title 'Home', got %q", page.Title)
	}
}

func TestWiki_CreatePage_WithParent(t *testing.T) {
	w := setupTestWiki(t)
	rootPage, _ := w.CreatePage(nil, "Docs")

	page, err := w.CreatePage(&rootPage.ID, "API")
	if err != nil {
		t.Fatalf("CreatePage with parent failed: %v", err)
	}

	if page.Parent.ID != rootPage.ID {
		t.Errorf("Expected parent ID %q, got %q", rootPage.ID, page.Parent.ID)
	}
}

func TestWiki_CreatePage_InvalidParent(t *testing.T) {
	w := setupTestWiki(t)
	invalidID := "not-real"
	_, err := w.CreatePage(&invalidID, "Broken")
	if err == nil {
		t.Error("Expected error with invalid parent ID, got none")
	}
}

func TestWiki_GetPage_ValidID(t *testing.T) {
	w := setupTestWiki(t)
	page, _ := w.CreatePage(nil, "ReadMe")

	found, err := w.GetPage(page.ID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	if found.ID != page.ID {
		t.Errorf("Expected ID %q, got %q", page.ID, found.ID)
	}
}

func TestWiki_GetPage_InvalidID(t *testing.T) {
	w := setupTestWiki(t)
	_, err := w.GetPage("unknown")
	if err == nil {
		t.Error("Expected error for unknown ID, got none")
	}
}

func TestWiki_MovePage_Valid(t *testing.T) {
	w := setupTestWiki(t)
	parent, _ := w.CreatePage(nil, "Projects")
	child, _ := w.CreatePage(nil, "Old")

	err := w.MovePage(child.ID, parent.ID)
	if err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}
}

func TestWiki_DeletePage_Simple(t *testing.T) {
	w := setupTestWiki(t)
	page, _ := w.CreatePage(nil, "Trash")

	err := w.DeletePage(page.ID, false)
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}
}

func TestWiki_DeletePage_WithChildren(t *testing.T) {
	w := setupTestWiki(t)
	parent, _ := w.CreatePage(nil, "Parent")
	_, _ = w.CreatePage(&parent.ID, "Child")

	err := w.DeletePage(parent.ID, false)
	if err == nil {
		t.Error("Expected error when deleting parent with children")
	}
}

func TestWiki_DeletePage_Recursive(t *testing.T) {
	w := setupTestWiki(t)
	parent, _ := w.CreatePage(nil, "Parent")
	_, _ = w.CreatePage(&parent.ID, "Child")

	err := w.DeletePage(parent.ID, true)
	if err != nil {
		t.Fatalf("DeletePage recursive failed: %v", err)
	}
}

func TestWiki_UpdatePage(t *testing.T) {
	w := setupTestWiki(t)
	page, _ := w.CreatePage(nil, "Draft")

	page, err := w.UpdatePage(page.ID, "Final", "final", "# Updated")
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	updated, _ := w.GetPage(page.ID)
	if updated.Title != "Final" {
		t.Errorf("Expected title 'Final', got %q", updated.Title)
	}
}

func TestWiki_SuggestSlug_Unique(t *testing.T) {
	w := setupTestWiki(t)
	slug, err := w.SuggestSlug("root", "My Page")
	if err != nil {
		t.Fatalf("SuggestSlug failed: %v", err)
	}
	if slug != "my-page" {
		t.Errorf("Expected 'my-page', got %q", slug)
	}
}

func TestWiki_SuggestSlug_Conflict(t *testing.T) {
	w := setupTestWiki(t)
	root := w.GetTree()
	_, err := w.CreatePage(nil, "My Page")

	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	slug, err := w.SuggestSlug(root.ID, "My Page")
	if err != nil {
		t.Fatalf("SuggestSlug failed: %v", err)
	}
	if slug != "my-page-1" {
		t.Errorf("Expected 'my-page-1', got %q", slug)
	}
}

func TestWiki_SuggestSlug_DeepHierarchy(t *testing.T) {
	tmpDir := t.TempDir()
	wiki, err := NewWiki(tmpDir)
	if err != nil {
		t.Fatalf("Failed to initialize Wiki: %v", err)
	}

	// Erstelle tiefere Struktur: root -> architecture -> backend
	_, err = wiki.CreatePage(nil, "Architecture")
	if err != nil {
		t.Fatalf("Failed to create 'Architecture': %v", err)
	}
	root := wiki.GetTree()
	arch := root.Children[0]

	_, err = wiki.CreatePage(&arch.ID, "Backend")
	if err != nil {
		t.Fatalf("Failed to create 'Backend': %v", err)
	}
	backend := arch.Children[0]

	// Jetzt dort einen Slug vorschlagen
	slug, err := wiki.SuggestSlug(backend.ID, "Data Layer")
	if err != nil {
		t.Fatalf("SuggestSlug failed: %v", err)
	}

	if slug != "data-layer" {
		t.Errorf("Expected 'data-layer', got %q", slug)
	}

	// Erzeuge ein zweites mit gleichem Namen → es muss nummeriert werden
	_, err = wiki.CreatePage(&backend.ID, "Data Layer")
	if err != nil {
		t.Fatalf("Failed to create 'Data Layer': %v", err)
	}

	slug2, err := wiki.SuggestSlug(backend.ID, "Data Layer")
	if err != nil {
		t.Fatalf("SuggestSlug 2 failed: %v", err)
	}

	if slug2 != "data-layer-1" {
		t.Errorf("Expected 'data-layer-1', got %q", slug2)
	}
}

func TestWiki_FindByPath_Valid(t *testing.T) {
	w := setupTestWiki(t)
	_, _ = w.CreatePage(nil, "Company")

	found, err := w.FindByPath("company")
	if err != nil {
		t.Fatalf("FindByPath failed: %v", err)
	}
	if found.Slug != "company" {
		t.Errorf("Expected slug 'company', got %q", found.Slug)
	}
}

func TestWiki_FindByPath_Invalid(t *testing.T) {
	w := setupTestWiki(t)
	_, err := w.FindByPath("does/not/exist")
	if err == nil {
		t.Error("Expected error for invalid path, got none")
	}
}
