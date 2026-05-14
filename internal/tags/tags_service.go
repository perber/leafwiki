package tags

import (
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
)

type TagsService struct {
	store *TagsStore
	tree  *tree.TreeService
}

func NewTagsService(treeService *tree.TreeService, store *TagsStore) *TagsService {
	return &TagsService{store: store, tree: treeService}
}

// IndexAllPages rebuilds the entire tag index from the current tree state.
func (s *TagsService) IndexAllPages() error {
	if !s.tree.IsLoaded() {
		return nil
	}

	if err := s.store.Clear(); err != nil {
		return err
	}

	var ids []string
	if err := s.tree.WalkNodes(func(id string) error {
		ids = append(ids, id)
		return nil
	}); err != nil {
		return err
	}

	pages, errs := s.tree.GetPages(ids)
	for i, page := range pages {
		if errs[i] != nil {
			return errs[i]
		}
		tags := ExtractTagsFromContent(page.Content)
		if err := s.store.SetTagsForPage(page.ID, tags); err != nil {
			return err
		}
	}

	return nil
}

func (s *TagsService) SetTagsForPage(pageID string, tags []string) error {
	return s.store.SetTagsForPage(pageID, tags)
}

func (s *TagsService) DeleteTagsForPage(pageID string) error {
	return s.store.DeleteTagsForPage(pageID)
}

func (s *TagsService) GetAllTags(filter string, limit int) ([]TagCount, error) {
	return s.store.GetAllTags(filter, limit)
}

func (s *TagsService) GetPageIDsByTags(tags []string) ([]string, error) {
	return s.store.GetPageIDsByTags(tags)
}

func (s *TagsService) GetTagsForPages(pageIDs []string) (map[string][]string, error) {
	return s.store.GetTagsForPages(pageIDs)
}

// ExtractTagsFromContent parses frontmatter and returns lowercase-normalized tags.
func ExtractTagsFromContent(content string) []string {
	fm, _, has, err := markdown.ParseFrontmatter(content)
	if err != nil || !has {
		return nil
	}

	for key, value := range fm.ExtraFields {
		if strings.EqualFold(strings.TrimSpace(key), "tags") {
			return normalizeTags(value)
		}
	}
	return nil
}

func normalizeTags(value interface{}) []string {
	list, ok := value.([]interface{})
	if !ok {
		return nil
	}

	seen := make(map[string]struct{})
	result := make([]string, 0, len(list))
	for _, item := range list {
		tag, ok := item.(string)
		if !ok {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
