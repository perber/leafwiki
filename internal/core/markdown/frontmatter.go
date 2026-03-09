package markdown

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"unicode"

	yaml "gopkg.in/yaml.v3"
)

func invalidYAMLKeyRune(r rune) bool {
	//nolint:staticcheck
	return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
}

type Frontmatter struct {
	LeafWikiID    string `yaml:"leafwiki_id,omitempty" json:"id,omitempty"`
	LeafWikiTitle string `yaml:"leafwiki_title,omitempty" json:"title,omitempty"`
	CreatedAt     string `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	CreatorID     string `yaml:"creator_id,omitempty" json:"creator_id,omitempty"`
	UpdatedAt     string `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	LastAuthorID  string `yaml:"last_author_id,omitempty" json:"last_author_id,omitempty"`
}

func (fm *Frontmatter) stripSingleAndDoubleQuotes(s string) string {
	s = strings.Trim(s, `"`)
	s = strings.Trim(s, `'`)
	return s
}

func (fm *Frontmatter) Normalize() {
	fm.LeafWikiID = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(fm.LeafWikiID))
	fm.LeafWikiTitle = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(fm.LeafWikiTitle))
	fm.CreatedAt = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(fm.CreatedAt))
	fm.CreatorID = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(fm.CreatorID))
	fm.UpdatedAt = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(fm.UpdatedAt))
	fm.LastAuthorID = fm.stripSingleAndDoubleQuotes(strings.TrimSpace(fm.LastAuthorID))
}

func (fm Frontmatter) IsZero() bool {
	return fm.LeafWikiID == "" &&
		fm.LeafWikiTitle == "" &&
		fm.CreatedAt == "" &&
		fm.CreatorID == "" &&
		fm.UpdatedAt == "" &&
		fm.LastAuthorID == ""
}

func (fm *Frontmatter) LoadFrontMatterFromContent(yamlPart string) (has bool, err error) {
	if err := yaml.Unmarshal([]byte(yamlPart), fm); err != nil {
		return true, errors.Join(ErrFrontmatterParse, err)
	}

	type titleOnlyStruct struct {
		Title string `yaml:"title,omitempty"`
	}
	var tos titleOnlyStruct
	if err := yaml.Unmarshal([]byte(yamlPart), &tos); err == nil {
		if tos.Title != "" && fm.LeafWikiTitle == "" {
			fm.LeafWikiTitle = tos.Title
		}
	}

	fm.Normalize()
	return true, nil
}

func (fm *Frontmatter) LoadFrontMatterFromFile(mdFilePath string) (has bool, err error) {
	content, err := os.ReadFile(mdFilePath)
	if err != nil {
		return false, err
	}
	return fm.LoadFrontMatterFromContent(string(content))
}

func splitFrontmatter(md string) (yamlPart string, body string, has bool) {
	s := strings.TrimPrefix(md, "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	if s != "---" && !strings.HasPrefix(s, "---\n") {
		return "", md, false
	}

	firstNL := strings.IndexByte(s, '\n')
	if firstNL == -1 {
		return "", md, false
	}
	if strings.TrimSpace(s[:firstNL]) != "---" {
		return "", md, false
	}

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

func ParseFrontmatter(md string) (fm Frontmatter, body string, has bool, err error) {
	yamlPart, body, has := splitFrontmatter(md)
	if !has {
		return Frontmatter{}, md, false, nil
	}

	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return Frontmatter{}, md, true, errors.Join(ErrFrontmatterParse, err)
	}

	type titleOnlyStruct struct {
		Title string `yaml:"title,omitempty"`
	}
	var tos titleOnlyStruct
	if err := yaml.Unmarshal([]byte(yamlPart), &tos); err == nil {
		if tos.Title != "" && fm.LeafWikiTitle == "" {
			fm.LeafWikiTitle = tos.Title
		}
	}

	fm.Normalize()
	return fm, body, true, nil
}

func BuildMarkdownWithFrontmatter(fm Frontmatter, body string) (string, error) {
	fm.Normalize()

	if fm.IsZero() {
		return body, nil
	}

	b, err := yaml.Marshal(fm)
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
