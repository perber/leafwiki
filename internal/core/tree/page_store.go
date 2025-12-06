package tree

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// writeFileAtomic writes data to filename atomically by writing to a temp file
// in the same directory and then renaming it over the target.
func writeFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := path.Dir(filename)

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmpFile.Name()
	// Ensure the temp file is removed in case of an error
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if perm != 0 {
		if err := tmpFile.Chmod(perm); err != nil {
			tmpFile.Close()
			return fmt.Errorf("chmod temp file: %w", err)
		}
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, filename); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

type PageStore struct {
	storageDir string
}

func NewPageStore(storageDir string) *PageStore {
	return &PageStore{
		storageDir: storageDir,
	}
}

func (f *PageStore) LoadTree(filename string) (*PageNode, error) {
	fullPath := path.Join(f.storageDir, filename)

	// check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return &PageNode{
			ID:       "root",
			Slug:     "root",
			Title:    "root",
			Parent:   nil,
			Position: 0,
			Children: []*PageNode{},
		}, nil
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("could not open tree file")
	}
	defer file.Close()
	data, err := io.ReadAll(file)

	if err != nil {
		return nil, fmt.Errorf("could not read tree file")
	}

	tree := &PageNode{}
	if err := json.Unmarshal(data, tree); err != nil {
		return nil, fmt.Errorf("could not unmarshal tree data")
	}

	// assigns parent to children
	f.assignParentToChildren(tree)

	return tree, nil
}

func (f *PageStore) assignParentToChildren(parent *PageNode) {
	for _, child := range parent.Children {
		child.Parent = parent
		f.assignParentToChildren(child)
	}
}

func (f *PageStore) SaveTree(filename string, tree *PageNode) error {
	if tree == nil {
		return errors.New("a tree is required")
	}

	fullPath := path.Join(f.storageDir, filename)

	data, err := json.Marshal(tree)
	if err != nil {
		return fmt.Errorf("could not marshal tree: %v", err)
	}

	if err := writeFileAtomic(fullPath, data, 0o644); err != nil {
		return fmt.Errorf("could not atomically write tree file: %v", err)
	}

	return nil
}

func (f *PageStore) CreatePage(parentEntry *PageNode, newEntry *PageNode) error {
	if parentEntry == nil {
		return errors.New("a parent entry is required")
	}

	if newEntry == nil {
		return errors.New("a new entry is required")
	}

	// Retrieving the path of the parent entry
	parentPath := path.Join(f.storageDir, GeneratePathFromPageNode(parentEntry))

	if err := EnsurePageIsFolder(f.storageDir, GeneratePathFromPageNode(parentEntry)); err != nil {
		return fmt.Errorf("could not prepare parent folder: %w", err)
	}

	// Check if the folder exists
	if _, err := os.Stat(parentPath); os.IsNotExist(err) {
		if err := os.MkdirAll(parentPath, 0755); err != nil {
			return fmt.Errorf("could not create folder: %v", err)
		}
		// Create an empty index.md file / Fallback!
		indexPath := path.Join(parentPath, "index.md")
		if err := writeFileAtomic(indexPath, []byte(""), 0o644); err != nil {
			return fmt.Errorf("could not create index file: %v", err)
		}
	}

	// Now we can create the new entry as a file in the parent folder
	newFilename := path.Join(parentPath, newEntry.Slug+".md")
	if _, err := os.Stat(newFilename); err == nil {
		// The file already exists
		return fmt.Errorf("file already exists: %v", err)
	}

	// Create the file
	content := []byte("# " + newEntry.Title + "\n")
	if err := writeFileAtomic(newFilename, content, 0o644); err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}

	return nil
}

func (f *PageStore) DeletePage(entry *PageNode) error {
	if entry == nil {
		return errors.New("an entry is required")
	}

	// Retrieving the path of the entry
	entryPath := path.Join(f.storageDir, GeneratePathFromPageNode(entry))

	// Check if the entry is a folder
	if info, err := os.Stat(entryPath); err == nil && info.IsDir() {
		// Delete the folder
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("could not delete folder: %v", err)
		}
	}

	// Check if the entry is a file
	if _, err := os.Stat(entryPath + ".md"); err == nil {
		// Delete the file
		if err := os.Remove(entryPath + ".md"); err != nil {
			return fmt.Errorf("could not delete file: %v", err)
		}
	}

	if entry.Parent != nil {
		_ = FoldPageFolderIfEmpty(f.storageDir, GeneratePathFromPageNode(entry.Parent))
	}

	return nil
}

func (f *PageStore) UpdatePage(entry *PageNode, slug string, content string) error {
	if entry == nil {
		return errors.New("an entry is required")
	}

	filePath, err := f.getFilePath(entry)
	if err != nil {
		return fmt.Errorf("could not get file path: %v", err)
	}

	// Check if the file exists
	file, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not found: %v", err)
	}

	mode := file.Mode()

	// Update the file content
	if err := writeFileAtomic(filePath, []byte(content), mode); err != nil {
		return fmt.Errorf("could not write to file atomically: %v", err)
	}

	// We need to check if the slug has changed
	if entry.Slug != slug {
		// Get the old path
		oldPath := path.Join(f.storageDir, GeneratePathFromPageNode(entry))
		// Split the path
		parts := strings.Split(oldPath, "/")
		// Create the new path
		newPath := strings.Join(parts[:len(parts)-1], "/") + "/" + slug
		// Check if the old path is a directory
		// If it is a directory, we need to rename the directory
		// If it is a file, we need to rename the file
		if _, err := os.Stat(oldPath); err == nil {
			// Rename the directory
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("could not rename directory: %v", err)
			}

			return nil
		}
		// Rename the file
		if err := os.Rename(oldPath+".md", newPath+".md"); err != nil {
			return fmt.Errorf("could not rename file: %v", err)
		}
	}

	return nil
}

// MovePage moves a page to a other node
func (f *PageStore) MovePage(entry *PageNode, parentEntry *PageNode) error {
	if entry == nil {
		return errors.New("an entry is required")
	}

	if parentEntry == nil {
		return errors.New("a parent entry is required")
	}

	// Retrieving the path of the entry
	parentPath := path.Join(f.storageDir, GeneratePathFromPageNode(parentEntry))

	if err := EnsurePageIsFolder(f.storageDir, GeneratePathFromPageNode(parentEntry)); err != nil {
		return fmt.Errorf("could not convert parent to folder: %w", err)
	}

	// now we have created the folder, we can move the entry to the new parent
	currentPath := path.Join(f.storageDir, GeneratePathFromPageNode(entry))

	// Check if the entry is a file
	var src, dest string
	if _, err := os.Stat(currentPath + ".md"); err == nil {
		src = currentPath + ".md"
		dest = path.Join(parentPath, entry.Slug+".md")
	} else {
		src = currentPath
		dest = path.Join(parentPath, entry.Slug)
	}

	// Move the file to the parentPath
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("could not move file: %v", err)
	}

	if entry.Parent != nil {
		_ = FoldPageFolderIfEmpty(f.storageDir, GeneratePathFromPageNode(entry.Parent))
	}

	return nil
}

// ReadPageContent returns the content of a page
func (f *PageStore) ReadPageContent(entry *PageNode) (string, error) {
	if entry == nil {
		return "", errors.New("an entry is required")
	}

	filePath, err := f.getFilePath(entry)
	if err != nil {
		return "", fmt.Errorf("could not get file path: %v", err)
	}

	// Check if the file exists
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("file not found: %v", err)
	}

	// Read the file content
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("could not read file: %v", err)
	}
	return string(content), nil
}

func (f *PageStore) getFilePath(entry *PageNode) (string, error) {
	if entry == nil {
		return "", errors.New("an entry is required")
	}

	// Retrieving the path of the entry
	entryPath := path.Join(f.storageDir, GeneratePathFromPageNode(entry))

	// Check if the entry is a file
	if _, err := os.Stat(entryPath + ".md"); err == nil {
		return entryPath + ".md", nil
	}

	// Check if the entry is a folder
	if info, err := os.Stat(entryPath); err == nil && info.IsDir() {
		return path.Join(entryPath, "index.md"), nil
	}

	return "", errors.New("file not found")
}
