package tree

import (
	"fmt"
	"path/filepath"

	"github.com/gosimple/slug"
)

type SlugService struct {
}

func NewSlugService() *SlugService {
	return &SlugService{}
}

// GenerateUniqueSlug returns a slug that doesn't conflict with siblings of the given parent
func (s *SlugService) GenerateUniqueSlug(parent *PageNode, desired string) string {
	slug := normalizeSlug(desired)
	original := slug
	i := 1

	for hasSlugConflict(parent, slug) {
		slug = fmt.Sprintf("%s-%d", original, i)
		i++
	}

	return slug
}

// normalizeSlug creates a URL-friendly slug (can be improved)
func normalizeSlug(title string) string {
	return slug.Make(title)
}

// Checks if the given slug already exists among parent's children
func hasSlugConflict(parent *PageNode, slug string) bool {
	for _, child := range parent.Children {
		if child.Slug == slug {
			return true
		}
	}
	return false
}

func (s *SlugService) NormalizeFilename(filename string) string {
	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]
	return normalizeSlug(base) + ext
}

func (s *SlugService) GenerateUniqueFilename(existing []string, desired string) string {
	ext := filepath.Ext(desired)
	base := desired[:len(desired)-len(ext)]
	slugged := normalizeSlug(base)
	name := slugged + ext
	i := 1

	// Check conflicts in existing list
	conflicts := make(map[string]bool)
	for _, f := range existing {
		conflicts[f] = true
	}
	for conflicts[name] {
		name = fmt.Sprintf("%s-%d%s", slugged, i, ext)
		i++
	}

	return name
}
