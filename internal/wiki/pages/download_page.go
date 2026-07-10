package pages

import (
	"bytes"
	"context"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
)

// DownloadPageInput is the input for DownloadPageUseCase.
type DownloadPageInput struct {
	ID string
}

// DownloadPageOutput carries a downloadable representation of a node. The Kind
// determines the payload:
//   - a page    → Data is Markdown, Filename ends in ".md"
//   - a section → Data is a ZIP archive of the whole subtree, Filename ends in ".zip"
type DownloadPageOutput struct {
	Kind        tree.NodeKind
	Filename    string
	ContentType string
	Data        []byte
}

// DownloadPageUseCase produces a downloadable file for a page or section.
// Pages are served as clean Markdown; sections are zipped with their whole
// subtree so the entire folder can be downloaded in one request.
type DownloadPageUseCase struct {
	tree *tree.TreeService
}

// NewDownloadPageUseCase constructs a DownloadPageUseCase.
func NewDownloadPageUseCase(t *tree.TreeService) *DownloadPageUseCase {
	return &DownloadPageUseCase{tree: t}
}

// Execute resolves the node and returns its downloadable payload.
func (uc *DownloadPageUseCase) Execute(_ context.Context, in DownloadPageInput) (*DownloadPageOutput, error) {
	page, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return nil, err
	}

	base := downloadBaseName(page.Slug, page.Title, page.ID)

	if page.Kind == tree.NodeKindSection {
		var buf bytes.Buffer
		if err := uc.tree.ExportSectionZip(in.ID, &buf); err != nil {
			return nil, err
		}
		return &DownloadPageOutput{
			Kind:        tree.NodeKindSection,
			Filename:    base + ".zip",
			ContentType: "application/zip",
			Data:        buf.Bytes(),
		}, nil
	}

	return &DownloadPageOutput{
		Kind:        tree.NodeKindPage,
		Filename:    base + ".md",
		ContentType: "text/markdown; charset=utf-8",
		Data:        []byte(page.Content),
	}, nil
}

// downloadBaseName derives a safe, extension-less filename base for a download.
// It prefers the slug (already URL-safe), then the title, then the node ID.
func downloadBaseName(slug, title, id string) string {
	if name := sanitizeDownloadName(slug); name != "" {
		return name
	}
	if name := sanitizeDownloadName(title); name != "" {
		return name
	}
	if name := sanitizeDownloadName(id); name != "" {
		return name
	}
	return "page"
}

// sanitizeDownloadName turns an arbitrary label into a filesystem-safe filename
// base: lowercase, spaces and unsafe characters collapsed to single hyphens.
func sanitizeDownloadName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(value) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.':
			b.WriteRune(r)
			lastHyphen = false
		default:
			// Collapse any run of unsafe characters/whitespace into one hyphen.
			if !lastHyphen && b.Len() > 0 {
				b.WriteRune('-')
				lastHyphen = true
			}
		}
	}

	return strings.Trim(b.String(), "-.")
}
