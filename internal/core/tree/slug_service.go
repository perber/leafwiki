package tree

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
)

var reservedSlugs = map[string]bool{
	"e":      true,
	"edit":   true,
	"api":    true,
	"assets": true,
	"index":  true,
}

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

	for hasSlugConflict(parent, slug) || s.IsValidSlug(slug) != nil {
		slug = fmt.Sprintf("%s-%d", original, i)
		i++
	}

	return slug
}

func (s *SlugService) IsValidSlug(slug string) error {
	if slug == "" {
		return errors.New("slug must not be empty")
	}

	slug = strings.ToLower(slug)

	if reservedSlugs[slug] {
		return fmt.Errorf("slug '%s' is reserved", slug)
	}

	matched, err := regexp.MatchString(`^[a-z0-9]+(-[a-z0-9]+)*$`, slug)
	if err != nil || !matched {
		return errors.New("slug must contain only lowercase letters, numbers and hyphens")
	}

	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return errors.New("slug must not start or end with a hyphen")
	}

	return nil
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
	for conflicts[name] || s.IsValidSlug(name) != nil {
		name = fmt.Sprintf("%s-%d%s", slugged, i, ext)
		i++
	}

	return name
}
