package tree

import (
	"bytes"
	"errors"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

type Frontmatter struct {
	LeafWikiID    string `yaml:"leafwiki_id,omitempty" json:"id,omitempty"`
	LeafWikiTitle string `yaml:"leafwiki_title,omitempty" json:"title,omitempty"`
}

func SplitFrontmatter(md string) (yamlPart string, body string, has bool) {
	// BOM-safe + normalize newlines
	s := strings.TrimPrefix(md, "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Must start with '---' on the very first line
	if !(s == "---" || strings.HasPrefix(s, "---\n")) {
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
	// We'll scan line-by-line using indices.
	pos := firstNL + 1
	yamlStart := pos

	endDelimLineStart := -1
	endDelimLineEnd := -1

	looksLikeYAML := false

	for pos <= len(s) {
		// find end of current line
		nextNL := strings.IndexByte(s[pos:], '\n')
		var line string
		var lineEnd int
		if nextNL == -1 {
			// last line
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

		// Heuristic: at least one "key:" line => treat as YAML frontmatter
		// Skip blanks/comments
		if trim != "" && !strings.HasPrefix(trim, "#") {
			if idx := strings.IndexByte(trim, ':'); idx > 0 {
				key := strings.TrimSpace(trim[:idx])
				if key != "" && strings.IndexFunc(key, func(r rune) bool {
					return !(r >= 'a' && r <= 'z' ||
						r >= 'A' && r <= 'Z' ||
						r >= '0' && r <= '9' ||
						r == '_' || r == '-')
				}) == -1 {
					looksLikeYAML = true
				}
			}
		}

		// advance to next line
		if nextNL == -1 {
			pos = len(s) + 1
		} else {
			pos = lineEnd + 1
		}
	}

	// No closing delimiter found => treat as no frontmatter
	if endDelimLineStart == -1 {
		return "", md, false
	}

	// If it doesn't look like YAML, treat as plain markdown (separator use-case)
	if !looksLikeYAML {
		return "", md, false
	}

	// YAML is between yamlStart and the start of the closing delimiter line
	yamlPart = s[yamlStart:endDelimLineStart]
	yamlPart = strings.TrimSuffix(yamlPart, "\n") // nice-to-have

	// Body starts after the closing delimiter line (+ its trailing newline if present)
	bodyStart := endDelimLineEnd
	if bodyStart < len(s) && s[bodyStart:bodyStart+1] == "\n" {
		bodyStart++
	}
	body = s[bodyStart:]

	return yamlPart, body, true
}

func ParseFrontmatter(md string) (fm Frontmatter, body string, has bool, err error) {
	yamlPart, body, has := SplitFrontmatter(md)
	if !has {
		return Frontmatter{}, md, false, nil
	}

	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return Frontmatter{}, md, true, errors.Join(ErrFrontmatterParse, err)
	}
	return fm, body, true, nil
}

func BuildMarkdownWithFrontmatter(fm Frontmatter, body string) (string, error) {
	// Avoid emitting empty frontmatter like "{}"
	if strings.TrimSpace(fm.LeafWikiID) == "" {
		return body, nil
	}

	b, err := yaml.Marshal(fm)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(b) // yaml.v3 usually ends with \n, which is fine
	out.WriteString("---\n")
	out.WriteString(body)
	return out.String(), nil
}
