package pages

import (
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
)

// readPageMetadata reads the page's frontmatter and returns its tags and
// custom string properties. Both are always non-nil so the API serializes
// them as [] and {} for pages without frontmatter; read or parse failures
// yield the empty values as well.
func readPageMetadata(t *tree.TreeService, pageID string) ([]string, map[string]string) {
	tags := []string{}
	properties := map[string]string{}

	raw, err := t.ReadPageRaw(pageID)
	if err != nil {
		return tags, properties
	}

	fm, _, has, err := markdown.ParseFrontmatter(raw)
	if err != nil || !has || len(fm.ExtraFields) == 0 {
		return tags, properties
	}

	return extractPageMetadata(fm.ExtraFields)
}

func extractPageMetadata(fields map[string]interface{}) ([]string, map[string]string) {
	tags := []string{}
	properties := map[string]string{}

	for rawKey, value := range fields {
		key := strings.TrimSpace(rawKey)
		lower := strings.ToLower(key)

		if lower == "tags" {
			tags = normalizeMetadataTags(value)
			continue
		}
		if markdown.IsSystemKey(key) {
			continue
		}

		flattenMetadataEntry(key, value, properties)
	}

	return tags, properties
}

// flattenMetadataEntry recursively flattens nested YAML maps into dot-notation
// keys (e.g. {"a": {"b": "v"}} → properties["a.b"] = "v").
func flattenMetadataEntry(prefix string, value interface{}, properties map[string]string) {
	flattenMetadataEntryDepth(prefix, value, properties, 0)
}

func flattenMetadataEntryDepth(prefix string, value interface{}, properties map[string]string, depth int) {
	if depth >= maxFlattenDepth {
		return
	}
	switch v := value.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s != "" && !strings.ContainsRune(s, '\n') {
			if _, exists := properties[prefix]; !exists {
				properties[prefix] = s
			}
		}
	case map[string]interface{}:
		for childKey, childValue := range v {
			childKey = strings.TrimSpace(childKey)
			if childKey == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(childKey), "leafwiki_") {
				continue
			}
			flattenMetadataEntryDepth(prefix+"."+childKey, childValue, properties, depth+1)
		}
	}
}

const maxFlattenDepth = 20

func normalizeMetadataTags(value interface{}) []string {
	list, ok := value.([]interface{})
	if !ok {
		return []string{}
	}

	rawTags := make([]string, 0, len(list))
	for _, item := range list {
		tag, ok := item.(string)
		if !ok {
			continue
		}
		rawTags = append(rawTags, tag)
	}

	return normalizeTagInputs(rawTags)
}
