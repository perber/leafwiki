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
	LeafWikiPinned       bool                   `yaml:"leafwiki_pinned,omitempty" json:"pinned,omitempty"`
	ExtraFields          map[string]interface{} `yaml:"-" json:"-"`

	// repaired records whether parsing this frontmatter block required the
	// sanitize-and-retry fallback (see sanitizeFrontmatterYAML): the file's
	// raw frontmatter didn't parse as-is and had to be patched in memory.
	repaired bool
}

// WasRepaired reports whether this frontmatter was recovered via the
// sanitize-and-retry fallback rather than parsing cleanly on the first
// attempt. Callers with a logger (e.g. tree reconstruction) can use this to
// surface a warning that a file's frontmatter needed automatic repair.
func (fm Frontmatter) WasRepaired() bool {
	return fm.repaired
}

var unquotedTemplatePlaceholderLine = regexp.MustCompile(`^(\s*[^:\n]+:\s*)(\{\{[^}\n]+\}\})(\s*(?:#.*)?)$`)

func parseFrontmatterYAML(yamlPart string) (Frontmatter, error) {
	var raw map[string]interface{}
	repaired := false
	if err := yaml.Unmarshal([]byte(yamlPart), &raw); err != nil {
		sanitized, changed := sanitizeFrontmatterYAML(yamlPart)
		if !changed {
			return Frontmatter{}, errors.Join(ErrFrontmatterParse, err)
		}
		if retryErr := yaml.Unmarshal([]byte(sanitized), &raw); retryErr != nil {
			return Frontmatter{}, errors.Join(ErrFrontmatterParse, err)
		}
		repaired = true
	}
	if raw == nil {
		raw = map[string]interface{}{}
	}

	fm := Frontmatter{ExtraFields: map[string]interface{}{}, repaired: repaired}

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
	if value, ok := raw["leafwiki_pinned"]; ok {
		if b, ok := value.(bool); ok {
			fm.LeafWikiPinned = b
		}
	}

	for key, value := range raw {
		switch key {
		case "leafwiki_id", "leafwiki_title", "leafwiki_created_at", "leafwiki_updated_at", "leafwiki_creator_id", "leafwiki_last_author_id", "leafwiki_pinned":
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

// sanitizeLines rewrites yamlPart one line at a time via fn, which returns
// the replacement line and whether it changed anything. Shared scaffolding
// for the frontmatter recovery fixups below, which otherwise differ only in
// their per-line match/replace logic.
func sanitizeLines(yamlPart string, fn func(line string) (string, bool)) (string, bool) {
	lines := strings.Split(yamlPart, "\n")
	changed := false

	for i, line := range lines {
		if rewritten, ok := fn(line); ok {
			lines[i] = rewritten
			changed = true
		}
	}

	return strings.Join(lines, "\n"), changed
}

func sanitizeTemplatePlaceholderFrontmatter(yamlPart string) (string, bool) {
	return sanitizeLines(yamlPart, func(line string) (string, bool) {
		matches := unquotedTemplatePlaceholderLine.FindStringSubmatch(line)
		if matches == nil {
			return "", false
		}
		return matches[1] + strconv.Quote(matches[2]) + matches[3], true
	})
}

// knownLeafWikiStringFrontmatterKeys are the leafwiki_* keys that are always
// plain string scalars. leafwiki_pinned is deliberately excluded: it's a
// bool, and quoting "true"/"false" would break its type assertion.
var knownLeafWikiStringFrontmatterKeys = []string{
	"leafwiki_id",
	"leafwiki_title",
	"leafwiki_created_at",
	"leafwiki_updated_at",
	"leafwiki_creator_id",
	"leafwiki_last_author_id",
}

// unquotedLeafWikiStringFieldLine matches a leafwiki_* string key together
// with the rest of its line as a raw value. Anchored at column 0
// (no leading \s*) deliberately: these keys always live at the top level of
// the frontmatter map, so requiring zero indentation means this can never
// match an indented continuation line inside an unrelated field's block
// scalar (e.g. "notes: |") content, which would otherwise get corrupted.
var unquotedLeafWikiStringFieldLine = regexp.MustCompile(
	`^((?:` + strings.Join(knownLeafWikiStringFrontmatterKeys, "|") + `)\s*:\s*)(\S.*)$`,
)

// lineIsValidYAML reports whether a single "key: value" line parses on its
// own as valid YAML.
func lineIsValidYAML(line string) bool {
	var probe map[string]interface{}
	return yaml.Unmarshal([]byte(line), &probe) == nil
}

// sanitizeUnquotedColonInLeafWikiStringFields quotes leafwiki_* string field
// values that don't parse as valid YAML on their own -- most commonly an
// unquoted colon (e.g. "leafwiki_title: ADR-0001: Filesystem as Source of
// Truth", which yaml.v3 otherwise misparses as a nested mapping key), but
// also other YAML-hostile leading characters (a stray backtick or "@").
// Scoped to known leafwiki_* keys only, since arbitrary user-defined
// frontmatter keys may legitimately hold nested YAML. Values that look like
// deliberate YAML constructs (flow-collections, anchors, aliases, tags,
// block scalars, directives) are left alone regardless of validity, so a
// malformed one keeps failing loudly instead of being silently coerced into
// a string.
//
// Beyond that exclusion, validity is checked by actually parsing the
// candidate line with yaml.v3, rather than a hand-maintained "does it look
// already-quoted" heuristic. That means already-valid lines (properly
// quoted, or simply unproblematic -- including a genuine "#" comment) are
// always left untouched, while anything that truly fails to parse on its
// own gets the same quote-and-retry treatment, regardless of
// which specific character caused the failure.
func sanitizeUnquotedColonInLeafWikiStringFields(yamlPart string) (string, bool) {
	return sanitizeLines(yamlPart, func(line string) (string, bool) {
		matches := unquotedLeafWikiStringFieldLine.FindStringSubmatch(line)
		if matches == nil {
			return "", false
		}

		prefix, value := matches[1], strings.TrimRight(matches[2], " \t")
		if value == "" {
			return "", false
		}
		// Leave flow-collection/anchor/alias/tag/block-scalar/directive-style
		// values alone (e.g. "[...", "{...", "&anchor", "*alias", "!tag",
		// "|block", ">folded", "%directive") -- those are deliberate YAML
		// constructs, not a plain string that merely needs quoting. A
		// malformed one (e.g. an unclosed "[..." list) should keep failing
		// loudly rather than being silently coerced into a string. A leading
		// "{{" is excepted: that's the template-placeholder syntax the
		// sibling sanitizeTemplatePlaceholderFrontmatter fixup targets, not a
		// single-brace YAML flow mapping, so it's safe (and necessary, for
		// composite values like "{{date}}: draft") to let it fall through to
		// the real-parse check below.
		if strings.IndexByte("[&*!|>%", value[0]) >= 0 {
			return "", false
		}
		if value[0] == '{' && !strings.HasPrefix(value, "{{") {
			return "", false
		}
		if lineIsValidYAML(prefix + value) {
			return "", false
		}

		return prefix + strconv.Quote(value), true
	})
}

// sanitizeFrontmatterYAML chains the frontmatter recovery fixups, feeding
// each one's output into the next. Safe to chain rather than retry
// independently: sanitizeUnquotedColonInLeafWikiStringFields re-validates
// each candidate line with the real YAML parser before touching it, so a
// line the template-placeholder fixup already rewrote (and thus already
// parses fine on its own) is correctly recognized as done and left alone --
// this only ever runs after the initial yaml.Unmarshal has already failed.
func sanitizeFrontmatterYAML(yamlPart string) (string, bool) {
	out := yamlPart
	changedAny := false

	if sanitized, changed := sanitizeTemplatePlaceholderFrontmatter(out); changed {
		out, changedAny = sanitized, true
	}
	if sanitized, changed := sanitizeUnquotedColonInLeafWikiStringFields(out); changed {
		out, changedAny = sanitized, true
	}

	return out, changedAny
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
				hasYAMLSeparator := idx == len(trim)-1 || unicode.IsSpace(rune(trim[idx+1]))
				if hasYAMLSeparator && key != "" && strings.IndexFunc(key, invalidYAMLKeyRune) == -1 {
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
	if fm.LeafWikiPinned {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "leafwiki_pinned"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
		)
	}

	return buildMarkdownFromMapping(mapping, body)
}
