package markdown

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/perber/wiki/internal/core/shared"
)

type MarkdownFile struct {
	path    string
	content string
	fm      Frontmatter
}

func LoadMarkdownFile(filePath string) (*MarkdownFile, error) {
	if !strings.EqualFold(filepath.Ext(filePath), ".md") {
		return nil, errors.New("file is not a markdown file")
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return NewMarkdownFileFromRaw(filePath, string(raw))
}

func NewMarkdownFileFromRaw(filePath string, raw string) (*MarkdownFile, error) {
	fm, content, has, err := ParseFrontmatter(raw)
	if err != nil {
		return nil, err
	}
	if !has {
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
	fmContent, err := BuildMarkdownWithFrontmatter(mf.fm, mf.content)
	if err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	if st, err := os.Stat(mf.path); err == nil {
		mode = st.Mode()
	}

	return shared.WriteFileAtomic(mf.path, []byte(fmContent), mode)
}

func (mf *MarkdownFile) GetTitle() (string, error) {
	if mf.fm.LeafWikiTitle != "" {
		return strings.TrimSpace(mf.fm.LeafWikiTitle), nil
	}

	title, err := mf.extractTitleFromFirstHeading()
	if err == nil && title != "" {
		return title, nil
	}

	base := path.Base(mf.path)
	name := strings.TrimSuffix(base, path.Ext(base))
	return name, nil
}

func (mf *MarkdownFile) extractTitleFromFirstHeading() (string, error) {
	lines := strings.Split(mf.content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# ")), nil
		}
	}
	return "", errors.New("no heading found")
}

func (mf *MarkdownFile) GetContent() string {
	return mf.content
}

func (mf *MarkdownFile) SetContent(content string) {
	mf.content = content
}

func (mf *MarkdownFile) GetPath() string {
	return mf.path
}

func (mf *MarkdownFile) GetFrontmatter() Frontmatter {
	return mf.fm
}

func (mf *MarkdownFile) setFrontmatterID(id string) {
	mf.fm.LeafWikiID = id
}

func (mf *MarkdownFile) setFrontmatterTitle(title string) {
	mf.fm.LeafWikiTitle = title
}

func (mf *MarkdownFile) SetLeafWikiFrontmatter(id string, title string) {
	mf.setFrontmatterID(id)
	mf.setFrontmatterTitle(title)
}
