package markdown

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/perber/wiki/internal/core/shared"
)

// MarkdownFile is the path-aware abstraction for markdown files that may need
// title extraction, frontmatter updates, and writes back to disk.
type MarkdownFile struct {
	path    string
	content string
	fm      Frontmatter
}

// LoadMarkdownFile reads a markdown file from disk and returns a MarkdownFile.
// Use this when file path semantics matter, for example title fallback from the
// filename or later write-back to the same path.
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

// NewMarkdownFileFromRaw builds a MarkdownFile from already loaded content.
// Use this when the caller already has the raw markdown string and wants the
// MarkdownFile behavior without a second filesystem read.
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

// NewMarkdownFile constructs a MarkdownFile from explicit content and parsed
// frontmatter, typically for new files or callers that already control both.
func NewMarkdownFile(filePath string, content string, fm Frontmatter) *MarkdownFile {
	return &MarkdownFile{
		path:    filePath,
		content: content,
		fm:      fm,
	}
}

// WriteToFile serializes frontmatter and body and writes them back atomically
// to the MarkdownFile path.
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

// GetTitle resolves the effective title from leafwiki_title first, then from
// the first markdown heading, and finally from the file name.
func (mf *MarkdownFile) GetTitle() (string, error) {
	if mf.fm.LeafWikiTitle != "" {
		return strings.TrimSpace(mf.fm.LeafWikiTitle), nil
	}

	title, err := mf.extractTitleFromFirstHeading()
	if err == nil && title != "" {
		return title, nil
	}

	base := path.Base(strings.ReplaceAll(mf.path, `\`, "/"))
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

func (mf *MarkdownFile) SetRawContentPreservingManagedFrontmatter(raw string) error {
	incomingFM, body, has, err := ParseFrontmatter(raw)
	if err != nil {
		return err
	}

	if !has {
		mf.content = raw
		return nil
	}

	mf.content = body
	mf.fm.ExtraFields = incomingFM.ExtraFields
	return nil
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

func (mf *MarkdownFile) SetLeafWikiMetadata(createdAt string, updatedAt string, creatorID string, lastAuthorID string) {
	mf.fm.LeafWikiCreatedAt = strings.TrimSpace(createdAt)
	mf.fm.LeafWikiUpdatedAt = strings.TrimSpace(updatedAt)
	mf.fm.LeafWikiCreatorID = strings.TrimSpace(creatorID)
	mf.fm.LeafWikiLastAuthorID = strings.TrimSpace(lastAuthorID)
}
