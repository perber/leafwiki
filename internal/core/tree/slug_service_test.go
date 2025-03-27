package tree

import (
	"testing"
)

func TestGenerateUniqueSlug_NoConflict(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{},
	}

	s := NewSlugService(nil)
	result := s.GenerateUniqueSlug(parent, "My Page")

	if result != "my-page" {
		t.Errorf("Expected 'my-page', got '%s'", result)
	}
}

func TestGenerateUniqueSlug_WithConflict(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{
			{Slug: "my-page"},
		},
	}

	s := NewSlugService(nil)
	result := s.GenerateUniqueSlug(parent, "My Page")

	if result != "my-page-1" {
		t.Errorf("Expected 'my-page-1', got '%s'", result)
	}
}

func TestGenerateUniqueSlug_MultipleConflicts(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{
			{Slug: "my-page"},
			{Slug: "my-page-1"},
			{Slug: "my-page-2"},
		},
	}

	s := NewSlugService(nil)
	result := s.GenerateUniqueSlug(parent, "My Page")

	if result != "my-page-3" {
		t.Errorf("Expected 'my-page-3', got '%s'", result)
	}
}

func TestGenerateUniqueSlug_SpecialCharacters(t *testing.T) {
	parent := &PageNode{}

	s := NewSlugService(nil)
	result := s.GenerateUniqueSlug(parent, "Äpfel & Bäume!")

	if result != "apfel-and-baume" {
		t.Errorf("Expected 'aepfel-and-baume', got '%s'", result)
	}
}
