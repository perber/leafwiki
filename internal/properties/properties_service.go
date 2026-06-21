package properties

import (
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
)


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
// Skips system keys (tags, leafwiki_*), lists, and nil values.
// "title" is skipped when it is used as a page-title alias (no leafwiki_title
// present); when leafwiki_title is explicit it is a user-defined property and
// is included.
// Nested YAML maps are flattened using dot notation (e.g. a.b: value).
func ExtractPropertiesFromContent(content string) map[string]PropertyEntry {
	fm, _, has, err := markdown.ParseFrontmatter(content)
	if err != nil || !has || len(fm.ExtraFields) == 0 {
		return nil
	}

	result := make(map[string]PropertyEntry)
	for rawKey, value := range fm.ExtraFields {
		key := strings.TrimSpace(rawKey)
		if markdown.IsSystemKey(key) {
			continue
		}
		// "title" without an explicit leafwiki_title is the page-title alias
		// and must not be indexed as a custom property.
		if strings.ToLower(key) == "title" && !fm.HasLeafWikiTitle {
			continue
		}
		extractFlatEntry(key, value, result)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

const maxNestedPropertyDepth = 20

// extractFlatEntry walks a potentially-nested value and stores all scalar leaves
// under dot-joined keys (e.g. {"a": {"b": "v"}} → "a.b" = "v").
// depth guards against unbounded recursion from crafted deeply-nested YAML.
func extractFlatEntry(prefix string, value interface{}, result map[string]PropertyEntry) {
	extractFlatEntryDepth(prefix, value, result, 0)
}

func extractFlatEntryDepth(prefix string, value interface{}, result map[string]PropertyEntry, depth int) {
	if depth >= maxNestedPropertyDepth {
		return
	}
	switch v := value.(type) {
	case string:
		if _, exists := result[prefix]; !exists {
			entry, ok := toPropertyEntry(v)
			if ok {
				result[prefix] = entry
			}
		}
	case map[string]interface{}:
		for childKey, childValue := range v {
			childKey = strings.TrimSpace(childKey)
			if childKey == "" {
				continue
			}
			// Skip child segments that use the system-reserved prefix so that
			// e.g. {"meta": {"leafwiki_id": "x"}} cannot pollute the index.
			if strings.HasPrefix(strings.ToLower(childKey), "leafwiki_") {
				continue
			}
			extractFlatEntryDepth(prefix+"."+childKey, childValue, result, depth+1)
		}
	}
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
