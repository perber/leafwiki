package properties

import (
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
)

// reservedKeys are frontmatter keys that must never be stored in the properties index.
// Any key starting with "leafwiki_" is also reserved (checked separately).
var reservedKeys = map[string]struct{}{
	"tags":  {},
	"title": {},
}

type PropertiesService struct {
	store *PropertiesStore
}

func NewPropertiesService(store *PropertiesStore) *PropertiesService {
	return &PropertiesService{store: store}
}

func (s *PropertiesService) ClearIndex() error {
	return s.store.Clear()
}

func (s *PropertiesService) IndexPageContent(pageID, rawContent string) error {
	props := ExtractPropertiesFromContent(rawContent)
	return s.store.SetPropertiesForPage(pageID, props)
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
