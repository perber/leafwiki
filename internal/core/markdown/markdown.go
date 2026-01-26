package markdown

import (
	"errors"
	"os"
	"path"
	"strings"
)

type MarkdownFile struct {
	path    string
	content string
	fm      Frontmatter
}

func LoadMarkdownFile(filePath string) (*MarkdownFile, error) {
	if !strings.HasSuffix(filePath, ".md") {
		return nil, errors.New("file is not a markdown file")
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	yamlPart, content, has := splitFrontmatter(string(raw))

	var fm Frontmatter

	if has {
		_, err = fm.LoadFrontMatterFromContent(string(yamlPart))
		if err != nil {
			return nil, err
		}
	} else {
		fm = Frontmatter{}
	}

	return &MarkdownFile{
		path:    filePath,
		content: content,
		fm:      fm,
	}, nil
}

func NewMarkdownFile(filePath string, content string, fm Frontmatter) *MarkdownFile {
	return &MarkdownFile{
		path:    filePath,
		content: content,
		fm:      fm,
	}
}

func (mf *MarkdownFile) WriteToFile() error {
	fmContent, err := BuildMarkdownWithFrontmatter(mf.fm, string(mf.content))
	if err != nil {
		return err
	}
	return os.WriteFile(mf.path, []byte(fmContent), 0644)
}

func (mf *MarkdownFile) GetTitle() (string, error) {
	// 1. Frontmatter title
	if mf.fm.LeafWikiTitle != "" {
		return strings.TrimSpace(mf.fm.LeafWikiTitle), nil
	}

	// 2. First heading
	title, err := mf.extractTitleFromFirstHeading()
	if err == nil && title != "" {
		return title, nil
	}

	// 3. Filename fallback
	base := path.Base(mf.path)
	name := strings.TrimSuffix(base, path.Ext(base))
	return name, nil
}

func (mf *MarkdownFile) extractTitleFromFirstHeading() (string, error) {
	lines := strings.Split(string(mf.content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# ")), nil
		}
	}
	return "", errors.New("no heading found")
}

func (mf *MarkdownFile) GetContent() string {
	return string(mf.content)
}

func (mf *MarkdownFile) GetPath() string {
	return mf.path
}

func (mf *MarkdownFile) GetFrontmatter() Frontmatter {
	return mf.fm
}
