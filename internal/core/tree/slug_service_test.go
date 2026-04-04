package tree

import (
	"testing"
)

func TestGenerateUniqueChildSlug_NoConflict(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{},
	}

	s := NewSlugService()
	result := s.GenerateUniqueChildSlug(parent, "", "My Page")

	if result != "my-page" {
		t.Errorf("Expected 'my-page', got '%s'", result)
	}
}

func TestGenerateUniqueChildSlug_WithConflict(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{
			{ID: "id", Slug: "my-page"},
		},
	}

	s := NewSlugService()
	result := s.GenerateUniqueChildSlug(parent, "new-id-same-parent", "My Page")

	if result != "my-page-1" {
		t.Errorf("Expected 'my-page-1', got '%s'", result)
	}
}

func TestGenerateUniqueChildSlug_MultipleConflicts(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{
			{ID: "id1", Slug: "my-page"},
			{ID: "id2", Slug: "my-page-1"},
			{ID: "id3", Slug: "my-page-2"},
		},
	}

	s := NewSlugService()
	result := s.GenerateUniqueChildSlug(parent, "new-id", "My Page")

	if result != "my-page-3" {
		t.Errorf("Expected 'my-page-3', got '%s'", result)
	}
}

func TestGenerateUniqueChildSlug_SlugShouldBeTheSame(t *testing.T) {
	parent := &PageNode{
		Children: []*PageNode{
			{ID: "id1", Slug: "my-page"},
		},
	}

	s := NewSlugService()
	result := s.GenerateUniqueChildSlug(parent, "id1", "My Page")

	if result != "my-page" {
		t.Errorf("Expected 'my-page', got '%s'", result)
	}
}

func TestGenerateUniqueChildSlug_SpecialCharacters(t *testing.T) {
	parent := &PageNode{}

	s := NewSlugService()
	result := s.GenerateUniqueChildSlug(parent, "", "Äpfel & Bäume!")

	if result != "apfel-and-baume" {
		t.Errorf("Expected 'aepfel-and-baume', got '%s'", result)
	}
}

func TestNormalizePath(t *testing.T) {
	s := NewSlugService()

	tests := []struct {
		input    string
		expected string
	}{
		{"folder/subfolder/page.md", "folder/subfolder/page-md"},
		{"My Folder/Another Folder/Page Title.md", "my-folder/another-folder/page-title-md"},
		{"Äpfel & Bäume/Über uns.md", "apfel-and-baume/uber-uns-md"},
		{"folder//subfolder///page.md", "folder/subfolder/page-md"},
		{"/leading/slash/page.md", "leading/slash/page-md"},
		{"only-file.md", "only-file-md"},
	}

	for _, test := range tests {

		result, err := s.NormalizePath(test.input, true)
		if err != nil {
			t.Errorf("Unexpected error for input %v: %v", test.input, err)
			continue
		}

		if result != test.expected {
			t.Errorf("For input %v, expected %v but got %v", test.input, test.expected, result)
		}
	}
}

func TestIsValidSlug_AllowsUppercase(t *testing.T) {
	s := NewSlugService()

	if err := s.IsValidSlug("ABCD-efg"); err != nil {
		t.Fatalf("expected uppercase slug to be valid, got %v", err)
	}
}

func TestGenerateValidSlug_ReservedSlugGetsSuffix(t *testing.T) {
	s := NewSlugService()

	if got := s.GenerateValidSlug("api"); got != "api-1" {
		t.Fatalf("GenerateValidSlug(api) = %q, want api-1", got)
	}
}

func TestNormalizePathToValidSlugs_ReservedSegmentGetsSuffix(t *testing.T) {
	s := NewSlugService()

	got, err := s.NormalizePathToValidSlugs("Reference/API")
	if err != nil {
		t.Fatalf("NormalizePathToValidSlugs err: %v", err)
	}
	if got != "reference/api-1" {
		t.Fatalf("NormalizePathToValidSlugs = %q, want reference/api-1", got)
	}
}

func TestNormalizeFilenameToValidSlug_ReservedSlugGetsSuffix(t *testing.T) {
	s := NewSlugService()

	got, err := s.NormalizeFilenameToValidSlug("API.md")
	if err != nil {
		t.Fatalf("NormalizeFilenameToValidSlug err: %v", err)
	}
	if got != "api-1.md" {
		t.Fatalf("NormalizeFilenameToValidSlug = %q, want api-1.md", got)
	}
}

func TestNormalizeFilenameToValidSlug_UnderscoreFilename(t *testing.T) {
	s := NewSlugService()

	got, err := s.NormalizeFilenameToValidSlug("CODE_OF_CONDUCT.md")
	if err != nil {
		t.Fatalf("NormalizeFilenameToValidSlug err: %v", err)
	}
	if got != "code-of-conduct.md" {
		t.Fatalf("NormalizeFilenameToValidSlug = %q, want code-of-conduct.md", got)
	}
}
