package tree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// checkNodeVersion enforces optimistic locking inside a write lock.
// Legacy nodes with no UpdatedAt (empty version) skip the check.
// Pass VersionUnchecked to bypass for internal system operations.
func checkNodeVersion(node *PageNode, expectedVersion string) error {
	if expectedVersion == VersionUnchecked {
		return nil
	}
	nodeVersion := node.Version()
	if nodeVersion == "" {
		return nil
	}
	if expectedVersion == "" {
		return ErrVersionRequired
	}
	if nodeVersion != expectedVersion {
		return ErrVersionConflict
	}
	return nil
}

func GeneratePathFromPageNode(entry *PageNode) string {
	path := ""
	if entry.Parent != nil {
		path = GeneratePathFromPageNode(entry.Parent) + "/" + entry.Slug
	} else {
		path = entry.Slug
	}
	return path
}

func pageDirectoryDiskPath(storageDir string, pagePath string) string {
	normalizedStorageDir := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	normalizedPagePath := filepath.FromSlash(strings.ReplaceAll(pagePath, `\`, `/`))
	return filepath.Join(normalizedStorageDir, normalizedPagePath)
}

func pageMarkdownDiskPath(storageDir string, pagePath string) string {
	normalizedStorageDir := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	normalizedPagePath := filepath.FromSlash(strings.ReplaceAll(pagePath, `\`, `/`))
	return filepath.Join(normalizedStorageDir, normalizedPagePath+".md")
}

func pageIndexDiskPath(storageDir string, pagePath string) string {
	return filepath.Join(pageDirectoryDiskPath(storageDir, pagePath), "index.md")
}

// EnsurePageIsFolder checks if a page path is still a flat .md file,
// and if so, converts it into a folder with an index.md file.
func EnsurePageIsFolder(storageDir string, pagePath string) error {
	mdPath := pageMarkdownDiskPath(storageDir, pagePath)
	dirPath := pageDirectoryDiskPath(storageDir, pagePath)

	// Already a folder? Nothing to do.
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		return nil
	}

	// If .md file exists → convert it to folder
	if _, err := os.Stat(mdPath); err == nil {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("could not create folder: %w", err)
		}

		newPath := pageIndexDiskPath(storageDir, pagePath)
		if err := os.Rename(mdPath, newPath); err != nil {
			return fmt.Errorf("could not move file to index.md: %w", err)
		}
	}

	return nil
}

// FoldPageFolderIfEmpty converts a page folder back into a flat file
// if it contains only "index.md" and nothing else.
func FoldPageFolderIfEmpty(storageDir string, pagePath string) error {
	dirPath := pageDirectoryDiskPath(storageDir, pagePath)
	mdPath := pageMarkdownDiskPath(storageDir, pagePath)
	indexPath := pageIndexDiskPath(storageDir, pagePath)

	// Only run if it's actually a folder
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		return nil // nothing to do
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("could not read folder: %w", err)
	}

	// Only fold if exactly 1 file: index.md
	if len(entries) != 1 || entries[0].Name() != "index.md" {
		return nil
	}

	// Move index.md → page.md
	if err := os.Rename(indexPath, mdPath); err != nil {
		return fmt.Errorf("could not move index.md to flat file: %w", err)
	}

	// Remove the now-empty folder
	if err := os.Remove(dirPath); err != nil {
		return fmt.Errorf("could not remove folder: %w", err)
	}

	return nil
}
