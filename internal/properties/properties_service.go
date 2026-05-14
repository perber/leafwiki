package properties

import (
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
)

// reservedKeys are frontmatter keys that must never be stored in the properties index.
// Any key starting with "leafwiki_" is also reserved (checked separately).
var reservedKeys = map[string]struct{}{
	"tags":  {},
	"title": {},
}

type PropertiesService struct {
	store *PropertiesStore
	tree  *tree.TreeService
}

func NewPropertiesService(treeService *tree.TreeService, store *PropertiesStore) *PropertiesService {
	return &PropertiesService{store: store, tree: treeService}
}

// IndexAllPages rebuilds the entire properties index from the current tree state.
func (s *PropertiesService) IndexAllPages() error {
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

	for _, id := range ids {
		raw, err := s.tree.ReadPageRaw(id)
		if err != nil {
			return err
		}

		props := ExtractPropertiesFromContent(raw)
		if err := s.store.SetPropertiesForPage(id, props); err != nil {
			return err
		}
	}

	return nil
}

func (s *PropertiesService) SetPropertiesForPage(pageID string, props map[string]PropertyEntry) error {
	return s.store.SetPropertiesForPage(pageID, props)
}

func (s *PropertiesService) DeletePropertiesForPage(pageID string) error {
	return s.store.DeletePropertiesForPage(pageID)
}

func (s *PropertiesService) GetAllPropertyKeys(filter string, limit int) ([]PropertyKeyCount, error) {
	return s.store.GetAllPropertyKeys(filter, limit)
}

func (s *PropertiesService) GetPageIDsByProperty(key, value string) ([]string, error) {
	return s.store.GetPageIDsByProperty(key, value)
}

func (s *PropertiesService) GetPropertiesForPages(pageIDs []string) (map[string]map[string]PropertyEntry, error) {
	return s.store.GetPropertiesForPages(pageIDs)
}

// ExtractPropertiesFromContent parses frontmatter and returns scalar properties.
// Skips: reserved keys (tags, title, leafwiki_*), lists, nil values.
func ExtractPropertiesFromContent(content string) map[string]PropertyEntry {
	fm, _, has, err := markdown.ParseFrontmatter(content)
	if err != nil || !has || len(fm.ExtraFields) == 0 {
		return nil
	}

	result := make(map[string]PropertyEntry)
	for rawKey, value := range fm.ExtraFields {
		key := strings.TrimSpace(rawKey)
		if isReservedKey(key) {
			continue
		}
		entry, ok := toPropertyEntry(value)
		if !ok {
			continue
		}
		result[key] = entry
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func isReservedKey(key string) bool {
	lower := strings.ToLower(key)
	if _, ok := reservedKeys[lower]; ok {
		return true
	}
	return strings.HasPrefix(lower, "leafwiki_")
}

func toPropertyEntry(value interface{}) (PropertyEntry, bool) {
	s, ok := value.(string)
	if !ok {
		return PropertyEntry{}, false
	}
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsRune(s, '\n') {
		return PropertyEntry{}, false
	}
	return PropertyEntry{Value: s, Type: "text"}, true
}
