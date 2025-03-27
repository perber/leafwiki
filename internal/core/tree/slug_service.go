package tree

import (
	"fmt"

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
