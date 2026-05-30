package pages

import (
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/http/dto"
)

var reservedPropertyKeys = map[string]struct{}{
	"tags":  {},
	"title": {},
}

// EnrichPageMetadata fills API page metadata from the page frontmatter.
func EnrichPageMetadata(page *dto.Page, readPageRaw func(string) (string, error)) {
	if page == nil {
		return
	}

	page.Tags = []string{}
	page.Properties = map[string]string{}

	raw, err := readPageRaw(page.ID)
	if err != nil {
		return
	}

	fm, _, has, err := markdown.ParseFrontmatter(raw)
	if err != nil || !has || len(fm.ExtraFields) == 0 {
		return
	}

	page.Tags, page.Properties = ExtractPageMetadata(fm.ExtraFields)
}

func ExtractPageMetadata(fields map[string]interface{}) ([]string, map[string]string) {
	tags := []string{}
	properties := map[string]string{}

	for rawKey, value := range fields {
		key := strings.TrimSpace(rawKey)
		lower := strings.ToLower(key)

		if lower == "tags" {
			tags = normalizeMetadataTags(value)
			continue
		}

		if _, reserved := reservedPropertyKeys[lower]; reserved {
			continue
		}
		if strings.HasPrefix(lower, "leafwiki_") {
			continue
		}

		s, ok := value.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" || strings.ContainsRune(s, '\n') {
			continue
		}
		properties[key] = s
	}

	return tags, properties
}

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

func normalizeTagInputs(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))

	for _, tag := range tags {
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
