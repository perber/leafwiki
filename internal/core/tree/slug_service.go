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
	"e":        true,
	"edit":     true,
	"api":      true,
	"assets":   true,
	"branding": true,
	"index":    true,
	"users":    true,
	"user":     true,
	"login":    true,
	"settings": true,
}

type SlugService struct {
}

func NewSlugService() *SlugService {
	return &SlugService{}
}

// GenerateUniqueSlug generates a unique slug for a page under the given parent
// It normalizes the desired slug and checks for conflicts with siblings and reserved slugs.
// If there is a conflict, it appends a number to the slug until it finds a unique one.
// currentID is used to exclude the current page when checking for conflicts (useful for updates)
// For example, if desired is "about" and there is already an "about" page, it will try "about-1", "about-2", etc.
func (s *SlugService) GenerateUniqueSlug(parent *PageNode, currentID, desired string) string {
	slug := normalizeSlug(desired)
	original := slug
	i := 1

	for hasSlugConflict(parent, currentID, slug) || s.IsValidSlug(slug) != nil {
		slug = fmt.Sprintf("%s-%d", original, i)
		i++
	}

	return slug
}

// IsValidSlug checks if the slug is valid according to our rules
// Rules:
// - Must not be empty
// - Must not be a reserved slug (case-insensitive)
// - Must contain only letters, numbers and hyphens
// - Must not start or end with a hyphen
func (s *SlugService) IsValidSlug(slug string) error {
	if slug == "" {
		return errors.New("slug must not be empty")
	}

	// Check for reserved slugs (case-insensitive)
	lowerSlug := strings.ToLower(slug)
	if reservedSlugs[lowerSlug] {
		return fmt.Errorf("slug '%s' is reserved", slug)
	}

	matched, err := regexp.MatchString(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`, slug)
	if err != nil || !matched {
		return errors.New("slug must contain only letters, numbers and hyphens")
	}

	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return errors.New("slug must not start or end with a hyphen")
	}

	return nil
}

func (s *SlugService) GenerateSafeSlug(desired string) string {
	slug := normalizeSlug(desired)
	if slug == "" {
		return ""
	}

	original := slug
	i := 1
	for s.IsValidSlug(slug) != nil {
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
func hasSlugConflict(parent *PageNode, currentID string, slug string) bool {
	for _, child := range parent.Children {
		if strings.EqualFold(child.Slug, slug) && child.ID != currentID {
			return true
		}
	}
	return false
}

func (s *SlugService) NormalizePath(path string, validate bool) (string, error) {
	segments := make([]string, 0)

	for _, segment := range strings.Split(path, string("/")) {

		if segment == "" {
			continue
		}

		if validate {
			// normalize first and then validate
			// the validation will ensure that the segment is a proper slug
			seg := normalizeSlug(segment)
			if err := s.IsValidSlug(seg); err != nil {
				return "", fmt.Errorf("segment '%s' is not a valid slug: %v", segment, err)
			}
			segment = seg
		} else {
			segment = normalizeSlug(segment)
		}
		segments = append(segments, segment)
	}
	return strings.Join(segments, string("/")), nil
}

func (s *SlugService) NormalizeFilename(filename string) string {
	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]
	return normalizeSlug(base) + ext
}

func (s *SlugService) NormalizePathForCreation(value string) (string, error) {
	segments := make([]string, 0)

	for _, segment := range strings.Split(value, "/") {
		if segment == "" {
			continue
		}

		safe := s.GenerateSafeSlug(segment)
		if safe == "" {
			return "", fmt.Errorf("segment '%s' is not a valid slug: slug must not be empty", segment)
		}
		segments = append(segments, safe)
	}

	return strings.Join(segments, "/"), nil
}

func (s *SlugService) NormalizeFilenameForCreation(filename string) (string, error) {
	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]
	safe := s.GenerateSafeSlug(base)
	if safe == "" {
		return "", fmt.Errorf("filename '%s' is not a valid slug: slug must not be empty", filename)
	}
	return safe + ext, nil
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
