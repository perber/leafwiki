package tree

import (
	"log/slog"
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
)

// ParseFrontmatterMetadata extracts tags and typed properties from raw markdown content.
func ParseFrontmatterMetadata(rawContent string) ([]string, map[string]MetadataValue) {
	fm, _, has, err := markdown.ParseFrontmatter(rawContent)
	if err != nil || !has || len(fm.ExtraFields) == 0 {
		return []string{}, map[string]MetadataValue{}
	}
	return ParseFrontmatterFields(fm.ExtraFields)
}

// ParseFrontmatterFields extracts tags and typed properties from parsed frontmatter ExtraFields.
func ParseFrontmatterFields(fields map[string]interface{}) ([]string, map[string]MetadataValue) {
	tags := []string{}
	properties := map[string]MetadataValue{}

	for rawKey, value := range fields {
		key := strings.TrimSpace(rawKey)
		if strings.ToLower(key) == "tags" {
			tags = normalizeFrontmatterTags(value)
			continue
		}
		if markdown.IsSystemKey(key) {
			continue
		}
		mv, err := YamlValueToMetadataValue(value)
		if err != nil {
			slog.Warn("skipping metadata field with unconvertible value", "key", key, "error", err)
			continue
		}
		properties[key] = mv
	}

	return tags, properties
}

func normalizeFrontmatterTags(value interface{}) []string {
	list, ok := value.([]interface{})
	if !ok {
		return []string{}
	}
	seen := make(map[string]struct{}, len(list))
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
