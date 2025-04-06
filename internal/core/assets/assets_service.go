package assets

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"

	"github.com/perber/wiki/internal/core/tree"
)

type AssetService struct {
	storageDir string
	slugger    *tree.SlugService
}

func NewAssetService(storageDir string, slugger *tree.SlugService) *AssetService {
	return &AssetService{
		storageDir: storageDir,
		slugger:    slugger,
	}
}

// SaveAssetForPage saves a file under a page's slug-based path and returns its public URL.
func (s *AssetService) SaveAssetForPage(page *tree.PageNode, file multipart.File, originalFilename string) (string, error) {
	// Build slugified and unique filename
	pagePath := tree.GeneratePathFromPageNode(page)

	if err := tree.EnsurePageIsFolder(s.storageDir, pagePath); err != nil {
		return "", fmt.Errorf("could not ensure page is folder: %w", err)
	}

	assetDir := path.Join(s.storageDir, pagePath, "assets")

	// Ensure assets directory exists
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		return "", fmt.Errorf("could not create asset dir: %w", err)
	}

	// Read existing filenames
	entries, _ := os.ReadDir(assetDir)
	existing := make([]string, 0, len(entries))
	for _, e := range entries {
		existing = append(existing, e.Name())
	}

	finalFilename := s.slugger.GenerateUniqueFilename(existing, originalFilename)
	fullPath := path.Join(assetDir, finalFilename)

	// Create and write the file
	out, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("could not create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", fmt.Errorf("could not write file: %w", err)
	}

	// Return public path (served from /assets)
	publicURL := fmt.Sprintf("/assets/%s/assets/%s", pagePath, finalFilename)
	return publicURL, nil
}

// ListAssetsForPage returns all asset filenames (not full paths)
func (s *AssetService) ListAssetsForPage(page *tree.PageNode) ([]string, error) {
	pagePath := tree.GeneratePathFromPageNode(page)
	assetDir := path.Join(s.storageDir, pagePath, "assets")

	files, err := os.ReadDir(assetDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not list assets: %w", err)
	}

	result := []string{}
	for _, f := range files {
		if !f.IsDir() {
			result = append(result, f.Name())
		}
	}

	return result, nil
}

// DeleteAsset removes an asset file from disk
func (s *AssetService) DeleteAsset(page *tree.PageNode, filename string) error {
	pagePath := tree.GeneratePathFromPageNode(page)
	assetDir := path.Join(s.storageDir, pagePath, "assets")
	assetPath := path.Join(assetDir, filename)

	if _, err := os.Stat(assetPath); os.IsNotExist(err) {
		return fmt.Errorf("asset not found: %s", filename)
	}

	if err := os.Remove(assetPath); err != nil {
		return fmt.Errorf("could not delete asset: %w", err)
	}

	files, err := os.ReadDir(assetDir)
	if err == nil && len(files) == 0 {
		_ = os.Remove(assetDir) // we don't care if this fails

		// 4. Try to fold the page folder back to flat
		_ = tree.FoldPageFolderIfEmpty(s.storageDir, pagePath)
	}

	return nil
}
