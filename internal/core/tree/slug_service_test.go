package tree

import (
	"testing"
)

func TestGenerateUniqueSlug_NoConflict(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{},
	}

	s := NewSlugService()
	result := s.GenerateUniqueSlug(parent, "", "My Page")

	if result != "my-page" {
		t.Errorf("Expected 'my-page', got '%s'", result)
	}
}

func TestGenerateUniqueSlug_WithConflict(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{
			{ID: "id", Slug: "my-page"},
		},
	}

	s := NewSlugService()
	result := s.GenerateUniqueSlug(parent, "new-id-same-parent", "My Page")

	if result != "my-page-1" {
		t.Errorf("Expected 'my-page-1', got '%s'", result)
	}
}

func TestGenerateUniqueSlug_MultipleConflicts(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{
			{ID: "id1", Slug: "my-page"},
			{ID: "id2", Slug: "my-page-1"},
			{ID: "id3", Slug: "my-page-2"},
		},
	}

	s := NewSlugService()
	result := s.GenerateUniqueSlug(parent, "new-id", "My Page")

	if result != "my-page-3" {
		t.Errorf("Expected 'my-page-3', got '%s'", result)
	}
}

func TestGenerateUniqueSlug_SlugShouldBeTheSame(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{
			{ID: "id1", Slug: "my-page"},
		},
	}

	s := NewSlugService()
	result := s.GenerateUniqueSlug(parent, "id1", "My Page")

	if result != "my-page" {
		t.Errorf("Expected 'my-page', got '%s'", result)
	}
}

func TestGenerateUniqueSlug_SpecialCharacters(t *testing.T) {
	parent := &PageNode{}

	s := NewSlugService()
	result := s.GenerateUniqueSlug(parent, "", "Äpfel & Bäume!")

	if result != "apfel-and-baume" {
		t.Errorf("Expected 'aepfel-and-baume', got '%s'", result)
	}
}
