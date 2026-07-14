package pages

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	coreassets "github.com/perber/wiki/internal/core/assets"
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
	tree  *tree.TreeService
	asset *coreassets.AssetService
}

// NewDownloadPageUseCase constructs a DownloadPageUseCase.
func NewDownloadPageUseCase(t *tree.TreeService, asset *coreassets.AssetService) *DownloadPageUseCase {
	return &DownloadPageUseCase{tree: t, asset: asset}
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
		if err := uc.exportSectionZip(page.PageNode, base, &buf); err != nil {
			return nil, err
		}
		return &DownloadPageOutput{
			Kind:        tree.NodeKindSection,
			Filename:    base + ".zip",
			ContentType: "application/zip",
			Data:        buf.Bytes(),
		}, nil
	}

	assets, err := uc.assetsForPage(page.PageNode)
	if err != nil {
		return nil, err
	}
	if len(assets) > 0 {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		assetDir := base + "_assets"
		content := rewriteAssetLinks(page.Content, assetDir, assets)
		if err := writeZipTextEntry(zw, base+".md", content); err != nil {
			_ = zw.Close()
			return nil, err
		}
		if err := writeAssetsToZip(zw, assetDir, assets); err != nil {
			_ = zw.Close()
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, fmt.Errorf("close zip: %w", err)
		}
		return &DownloadPageOutput{
			Kind:        tree.NodeKindPage,
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

func (uc *DownloadPageUseCase) exportSectionZip(node *tree.PageNode, base string, w io.Writer) error {
	zw := zip.NewWriter(w)
	if err := uc.writeNodeToZip(zw, node, base); err != nil {
		_ = zw.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	return nil
}

func (uc *DownloadPageUseCase) writeNodeToZip(zw *zip.Writer, node *tree.PageNode, basePath string) error {
	if node == nil {
		return nil
	}

	page, err := uc.tree.GetPage(node.ID)
	if err != nil {
		return err
	}

	switch node.Kind {
	case tree.NodeKindSection:
		if err := uc.writePageWithAssets(zw, page, basePath+"/index.md", basePath+"/assets", "assets"); err != nil {
			return err
		}
		for _, child := range node.Children {
			if child == nil {
				continue
			}
			if err := uc.writeNodeToZip(zw, child, basePath+"/"+child.Slug); err != nil {
				return err
			}
		}
		return nil
	case tree.NodeKindPage, "":
		assetRelDir := path.Base(basePath) + "_assets"
		return uc.writePageWithAssets(zw, page, basePath+".md", basePath+"_assets", assetRelDir)
	default:
		return fmt.Errorf("unknown node kind: %q", node.Kind)
	}
}

func (uc *DownloadPageUseCase) writePageWithAssets(zw *zip.Writer, page *tree.Page, mdPath, assetZipDir, assetRelDir string) error {
	assets, err := uc.assetsForPage(page.PageNode)
	if err != nil {
		return err
	}
	content := rewriteAssetLinks(page.Content, assetRelDir, assets)
	if err := writeZipTextEntry(zw, mdPath, content); err != nil {
		return err
	}
	return writeAssetsToZip(zw, assetZipDir, assets)
}

func (uc *DownloadPageUseCase) assetsForPage(page *tree.PageNode) ([]coreassets.AssetFile, error) {
	if uc.asset == nil {
		return []coreassets.AssetFile{}, nil
	}
	return uc.asset.ListAssetFilesForPage(page)
}

func rewriteAssetLinks(content, assetRelDir string, assets []coreassets.AssetFile) string {
	for _, asset := range assets {
		relativePath := path.Join(assetRelDir, asset.Name)
		content = strings.ReplaceAll(content, asset.PublicPath, relativePath)
		content = strings.ReplaceAll(content, strings.TrimPrefix(asset.PublicPath, "/"), relativePath)
	}
	return content
}

func writeZipTextEntry(zw *zip.Writer, name, content string) error {
	fw, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create zip entry %q: %w", name, err)
	}
	if _, err := io.WriteString(fw, content); err != nil {
		return fmt.Errorf("write zip entry %q: %w", name, err)
	}
	return nil
}

func writeAssetsToZip(zw *zip.Writer, assetDir string, assets []coreassets.AssetFile) error {
	for _, asset := range assets {
		if err := writeAssetToZip(zw, path.Join(assetDir, asset.Name), asset.DiskPath); err != nil {
			return err
		}
	}
	return nil
}

func writeAssetToZip(zw *zip.Writer, name, diskPath string) error {
	src, err := os.Open(diskPath)
	if err != nil {
		return fmt.Errorf("open asset %q: %w", diskPath, err)
	}
	defer src.Close()

	fw, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create zip asset %q: %w", name, err)
	}
	if _, err := io.Copy(fw, src); err != nil {
		return fmt.Errorf("write zip asset %q: %w", name, err)
	}
	return nil
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
