package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	yaml "gopkg.in/yaml.v3"
)

func invalidYAMLKeyRune(r rune) bool {
	//nolint:staticcheck
	return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
}

type Frontmatter struct {
	LeafWikiID           string                 `yaml:"leafwiki_id,omitempty" json:"id,omitempty"`
	LeafWikiTitle        string                 `yaml:"leafwiki_title,omitempty" json:"title,omitempty"`
	LeafWikiCreatedAt    string                 `yaml:"leafwiki_created_at,omitempty" json:"createdAt,omitempty"`
	LeafWikiUpdatedAt    string                 `yaml:"leafwiki_updated_at,omitempty" json:"updatedAt,omitempty"`
	LeafWikiCreatorID    string                 `yaml:"leafwiki_creator_id,omitempty" json:"creatorId,omitempty"`
	LeafWikiLastAuthorID string                 `yaml:"leafwiki_last_author_id,omitempty" json:"lastAuthorId,omitempty"`
	ExtraFields          map[string]interface{} `yaml:"-" json:"-"`
}

var unquotedTemplatePlaceholderLine = regexp.MustCompile(`^(\s*[^:\n]+:\s*)(\{\{[^}\n]+\}\})(\s*(?:#.*)?)$`)

func parseFrontmatterYAML(yamlPart string) (Frontmatter, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlPart), &raw); err != nil {
		sanitized, changed := sanitizeTemplatePlaceholderFrontmatter(yamlPart)
		if !changed {
			return Frontmatter{}, errors.Join(ErrFrontmatterParse, err)
		}
		if retryErr := yaml.Unmarshal([]byte(sanitized), &raw); retryErr != nil {
			return Frontmatter{}, errors.Join(ErrFrontmatterParse, err)
		}
	}
	if raw == nil {
		raw = map[string]interface{}{}
	}

	fm := Frontmatter{ExtraFields: map[string]interface{}{}}

	if value, ok := raw["leafwiki_id"]; ok {
		fm.LeafWikiID = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(valueToString(value)))
	}

	if value, ok := raw["leafwiki_title"]; ok {
		fm.LeafWikiTitle = fm.stripSingleAndDoubleQuotes(valueToString(value))
	} else if value, ok := raw["title"]; ok {
		fm.LeafWikiTitle = fm.stripSingleAndDoubleQuotes(valueToString(value))
	}
	if value, ok := raw["leafwiki_created_at"]; ok {
		fm.LeafWikiCreatedAt = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(valueToString(value)))
	}
	if value, ok := raw["leafwiki_updated_at"]; ok {
		fm.LeafWikiUpdatedAt = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(valueToString(value)))
	}
	if value, ok := raw["leafwiki_creator_id"]; ok {
		fm.LeafWikiCreatorID = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(valueToString(value)))
	}
	if value, ok := raw["leafwiki_last_author_id"]; ok {
		fm.LeafWikiLastAuthorID = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(valueToString(value)))
	}

	for key, value := range raw {
		switch key {
		case "leafwiki_id", "leafwiki_title", "leafwiki_created_at", "leafwiki_updated_at", "leafwiki_creator_id", "leafwiki_last_author_id":
			continue
		default:
			fm.ExtraFields[key] = value
		}
	}

	if len(fm.ExtraFields) == 0 {
		fm.ExtraFields = nil
	}

	return fm, nil
}

func sanitizeTemplatePlaceholderFrontmatter(yamlPart string) (string, bool) {
	lines := strings.Split(yamlPart, "\n")
	changed := false

	for i, line := range lines {
		matches := unquotedTemplatePlaceholderLine.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		lines[i] = matches[1] + strconv.Quote(matches[2]) + matches[3]
		changed = true
	}

	return strings.Join(lines, "\n"), changed
}

func valueToString(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case time.Time:
		return typed.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprint(typed)
	}
}

func (fm *Frontmatter) stripSingleAndDoubleQuotes(s string) string {
	s = strings.Trim(s, `"`)
	s = strings.Trim(s, `'`)
	return s
}

func splitFrontmatter(md string) (yamlPart string, body string, has bool) {
	// BOM-safe + normalize newlines
	s := strings.TrimPrefix(md, "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Must start with '---' on the very first line
	if s != "---" && !strings.HasPrefix(s, "---\n") {
		return "", md, false
	}

	// Find end of first line
	firstNL := strings.IndexByte(s, '\n')
	if firstNL == -1 {
		// it's exactly "---" (or a single-line file)
		return "", md, false
	}
	if strings.TrimSpace(s[:firstNL]) != "---" {
		return "", md, false
	}

	// Find closing delimiter on its own line: "\n---\n" or "\n---" at EOF
	pos := firstNL + 1
	yamlStart := pos

	endDelimLineStart := -1
	endDelimLineEnd := -1

	looksLikeYAML := false

	for pos <= len(s) {
		nextNL := strings.IndexByte(s[pos:], '\n')
		var line string
		var lineEnd int
		if nextNL == -1 {
			lineEnd = len(s)
			line = s[pos:lineEnd]
		} else {
			lineEnd = pos + nextNL
			line = s[pos:lineEnd]
		}

		trim := strings.TrimSpace(line)
		if trim == "---" {
			endDelimLineStart = pos
			endDelimLineEnd = lineEnd
			break
		}

		if trim != "" && !strings.HasPrefix(trim, "#") {
			if idx := strings.IndexByte(trim, ':'); idx > 0 {
				key := strings.TrimSpace(trim[:idx])
				if key != "" && strings.IndexFunc(key, invalidYAMLKeyRune) == -1 {
					looksLikeYAML = true
				}
			}
		}

		if nextNL == -1 {
			pos = len(s) + 1
		} else {
			pos = lineEnd + 1
		}
	}

	if endDelimLineStart == -1 {
		return "", md, false
	}

	if !looksLikeYAML {
		return "", md, false
	}

	yamlPart = s[yamlStart:endDelimLineStart]
	yamlPart = strings.TrimSuffix(yamlPart, "\n")

	bodyStart := endDelimLineEnd
	if bodyStart < len(s) && s[bodyStart:bodyStart+1] == "\n" {
		bodyStart++
	}
	body = s[bodyStart:]

	return yamlPart, body, true
}

// ParseFrontmatter splits already loaded markdown into frontmatter and body.
// Use this on raw content that is already in memory when you only need parsing,
// not path-based title fallback or write-back via MarkdownFile.
func ParseFrontmatter(md string) (fm Frontmatter, body string, has bool, err error) {
	yamlPart, body, has := splitFrontmatter(md)
	if !has {
		return Frontmatter{}, md, false, nil
	}

	fm, err = parseFrontmatterYAML(yamlPart)
	if err != nil {
		return Frontmatter{}, md, true, err
	}
	return fm, body, true, nil
}

func toYAMLNode(value interface{}) (*yaml.Node, error) {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return nil, err
	}
	return &node, nil
}

func buildExtraFieldsMapping(extraFields map[string]interface{}) (*yaml.Node, error) {
	mapping := &yaml.Node{Kind: yaml.MappingNode}

	extraKeys := make([]string, 0, len(extraFields))
	for key := range extraFields {
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)
	for _, key := range extraKeys {
		valueNode, err := toYAMLNode(extraFields[key])
		if err != nil {
			return nil, err
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			valueNode,
		)
	}

	return mapping, nil
}

func buildMarkdownFromMapping(mapping *yaml.Node, body string) (string, error) {
	if mapping == nil || len(mapping.Content) == 0 {
		return body, nil
	}

	b, err := yaml.Marshal(mapping)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(b)
	out.WriteString("---\n")
	out.WriteString(body)
	return out.String(), nil
}

func BuildMarkdownWithExtraFrontmatter(extraFields map[string]interface{}, body string) (string, error) {
	if len(extraFields) == 0 {
		return body, nil
	}

	mapping, err := buildExtraFieldsMapping(extraFields)
	if err != nil {
		return "", err
	}

	return buildMarkdownFromMapping(mapping, body)
}

// BuildMarkdownWithFrontmatter rebuilds markdown from parsed frontmatter data
// and a markdown body. It preserves additional frontmatter keys and emits them
// in deterministic order to keep rewrites stable.
func BuildMarkdownWithFrontmatter(fm Frontmatter, body string) (string, error) {
	if strings.TrimSpace(fm.LeafWikiID) == "" {
		return BuildMarkdownWithExtraFrontmatter(fm.ExtraFields, body)
	}

	mapping, err := buildExtraFieldsMapping(fm.ExtraFields)
	if err != nil {
		return "", err
	}

	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "leafwiki_id"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(fm.LeafWikiID)},
	)
	if strings.TrimSpace(fm.LeafWikiTitle) != "" {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "leafwiki_title"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(fm.LeafWikiTitle)},
		)
	}
	if strings.TrimSpace(fm.LeafWikiCreatedAt) != "" {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "leafwiki_created_at"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(fm.LeafWikiCreatedAt)},
		)
	}
	if strings.TrimSpace(fm.LeafWikiUpdatedAt) != "" {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "leafwiki_updated_at"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(fm.LeafWikiUpdatedAt)},
		)
	}
	if strings.TrimSpace(fm.LeafWikiCreatorID) != "" {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "leafwiki_creator_id"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(fm.LeafWikiCreatorID)},
		)
	}
	if strings.TrimSpace(fm.LeafWikiLastAuthorID) != "" {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "leafwiki_last_author_id"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(fm.LeafWikiLastAuthorID)},
		)
	}

	return buildMarkdownFromMapping(mapping, body)
}
