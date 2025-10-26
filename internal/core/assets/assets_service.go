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
	assetsDir string
	slugger   *tree.SlugService
}

func NewAssetService(storageDir string, slugger *tree.SlugService) *AssetService {
	// Ensure the storage directory exists
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		panic(fmt.Sprintf("could not create storage directory: %v", err))
	}
	// Ensure the assets directory exists
	assetsDir := path.Join(storageDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		panic(fmt.Sprintf("could not create assets directory: %v", err))
	}

	return &AssetService{
		assetsDir: assetsDir,
		slugger:   slugger,
	}
}

func (s *AssetService) GetAssetsDir() string {
	return s.assetsDir
}

func (s *AssetService) ensureAssetPagePathExists(page *tree.PageNode) (string, error) {
	pagePath := path.Join(s.assetsDir, page.ID)
	// check if the page path exists
	if _, err := os.Stat(pagePath); os.IsNotExist(err) {
		// create the page path
		if err := os.MkdirAll(pagePath, 0755); err != nil {
			return "", fmt.Errorf("could not create page path: %w", err)
		}
	}

	return pagePath, nil
}

func (s *AssetService) getAssetPagePath(page *tree.PageNode) (string, error) {
	pagePath := path.Join(s.assetsDir, page.ID)

	// check if the page path exists
	if _, err := os.Stat(pagePath); os.IsNotExist(err) {
		return "", fmt.Errorf("page path does not exist: %w", err)
	}

	return pagePath, nil
}

func (s *AssetService) buildPublicPath(page *tree.PageNode, filename string) string {
	return "/" + path.Join("assets", page.ID, filename)
}

// SaveAssetForPage saves a file under a page's slug-based path and returns its public URL.
func (s *AssetService) SaveAssetForPage(page *tree.PageNode, file multipart.File, originalFilename string) (string, error) {
	uploadPath, err := s.ensureAssetPagePathExists(page)
	if err != nil {
		return "", fmt.Errorf("could not upload file %w", err)
	}

	// Read existing filenames
	entries, _ := os.ReadDir(uploadPath)
	existing := make([]string, 0, len(entries))
	for _, e := range entries {
		existing = append(existing, e.Name())
	}

	finalFilename := s.slugger.GenerateUniqueFilename(existing, originalFilename)
	fullPath := path.Join(uploadPath, finalFilename)

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
	return s.buildPublicPath(page, finalFilename), nil
}

// ListAssetsForPage returns the full paths of all assets for a given page
func (s *AssetService) ListAssetsForPage(page *tree.PageNode) ([]string, error) {
	pagePath, err := s.getAssetPagePath(page)
	if err != nil {
		return []string{}, nil
	}

	files, err := os.ReadDir(pagePath)
	if err != nil && !os.IsNotExist(err) {
		return []string{}, nil
	}

	result := []string{}
	for _, f := range files {
		if !f.IsDir() {
			result = append(result, s.buildPublicPath(page, f.Name()))
		}
	}

	return result, nil
}

// DeleteAsset removes an asset file from disk
func (s *AssetService) DeleteAsset(page *tree.PageNode, filename string) error {
	assetPath, err := s.getAssetPagePath(page)
	if err != nil {
		return fmt.Errorf("asset not found: %s", filename)
	}

	fullPath := path.Join(assetPath, filename)

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("could not delete asset: %w", err)
	}

	// Check if the directory is empty and remove it if so
	files, err := os.ReadDir(assetPath)
	if err == nil && len(files) == 0 {
		_ = os.Remove(assetPath) // we don't care if this fails
	}

	return nil
}

func (s *AssetService) DeleteAllAssetsForPage(page *tree.PageNode) error {
	assetDir, err := s.getAssetPagePath(page)
	if err != nil {
		return fmt.Errorf("could not delete assets: %w", err)
	}
	if _, err := os.Stat(assetDir); err == nil {
		return os.RemoveAll(assetDir)
	}
	return nil
}

// RenameAsset renames an asset file for a given page.
func (s *AssetService) RenameAsset(page *tree.PageNode, oldFilename, newFilename string) (string, error) {
	assetPath, err := s.getAssetPagePath(page)
	if err != nil {
		return "", fmt.Errorf("could not rename asset: %w", err)
	}

	oldFullPath := path.Join(assetPath, oldFilename)
	newFullPath := path.Join(assetPath, newFilename)

	// Ensure that the new filename has the same extension as the old one
	oldExt := path.Ext(oldFilename)
	newExt := path.Ext(newFilename)
	if oldExt != newExt {
		return "", fmt.Errorf("new asset must have the same extension as the old one: %s", oldExt)
	}

	// Used for slug validation
	// The extension is not part of the slug, so we remove it
	newFilenameWithoutExt := newFilename[:len(newFilename)-len(newExt)]
	// Ensure that the new asset is a valid filename (slug)
	if err := s.slugger.IsValidSlug(newFilenameWithoutExt); err != nil {
		return "", fmt.Errorf("invalid asset name: %s", newFilename)
	}

	// Ensure that no file with the new name already exists
	if _, err := os.Stat(newFullPath); !os.IsNotExist(err) {
		return "", fmt.Errorf("new asset already exists: %s", newFilename)
	}

	if _, err := os.Stat(oldFullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("old asset does not exist: %s", oldFilename)
	}

	if err := os.Rename(oldFullPath, newFullPath); err != nil {
		return "", fmt.Errorf("could not rename asset: %w", err)
	}

	return s.buildPublicPath(page, newFilename), nil
}

func (s *AssetService) CopyAllAssets(sourcePage *tree.PageNode, targetPage *tree.PageNode) error {
	sourceAssetPath, err := s.getAssetPagePath(sourcePage)
	if err != nil {
		// No assets to copy
		return nil
	}

	targetAssetPath, err := s.ensureAssetPagePathExists(targetPage)
	if err != nil {
		return fmt.Errorf("could not create target asset path: %w", err)
	}

	entries, err := os.ReadDir(sourceAssetPath)
	if err != nil {
		return fmt.Errorf("could not read source asset directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // skip directories
		}
		if err := s.copySingleAsset(sourceAssetPath, targetAssetPath, entry); err != nil {
			return fmt.Errorf("could not copy asset %s: %w", entry.Name(), err)
		}
	}

	return nil
}

func (s *AssetService) copySingleAsset(sourceAssetPath string, targetAssetPath string, entry os.DirEntry) error {
	sourceFilePath := path.Join(sourceAssetPath, entry.Name())
	targetFilePath := path.Join(targetAssetPath, entry.Name())

	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("could not open source asset file: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return fmt.Errorf("could not create target asset file: %w", err)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("could not copy asset file: %w", err)
	}
}
