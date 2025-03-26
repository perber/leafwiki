package tree

import (
	"errors"
	"fmt"
	"strings"
)

// TreeEntry represents a single entry in the tree
// It has an ID, a parent, a path, and children
// The ID is a unique identifier for the entry
type TreeEntry struct {
	ID       string       `json:"id"`       // Unique identifier for the entry
	Title    string       `json:"title"`    // Title is the name of the entry
	Slug     string       `json:"slug"`     // Slug is the path of the entry
	Children []*TreeEntry `json:"children"` // Children are the children of the entry
	Parent   *TreeEntry   `json:"-"`
}

type PageEntry struct {
	*TreeEntry
	Content string `json:"content"`
}

// TreeService is our main component for handling tree operations
// We use this service to create pages, delete pages, update pages, etc.
type TreeService struct {
	storageDir   string
	treeFilename string
	tree         *TreeEntry
	fsService    *FileSystemTreeService
}

// NewTreeService creates a new TreeService
func NewTreeService(storageDir string) *TreeService {
	return &TreeService{
		storageDir:   storageDir,
		treeFilename: "tree.json",
		tree:         nil,
		fsService:    NewFileSystemTreeService(storageDir),
	}
}

// LoadTree loads the tree from the storage directory
// If the tree does not exist, it creates a new tree
func (t *TreeService) LoadTree() error {
	// Load the tree from the storage directory
	var err error
	t.tree, err = t.fsService.LoadTree(t.treeFilename)
	return err
}

func (t *TreeService) SaveTree() error {
	// Save the tree to the storage directory
	return t.fsService.SaveTree(t.treeFilename, t.tree)
}

// AddPageEntry adds a new leaf to the tree
func (t *TreeService) AddPageEntry(parentID *string, title string, slug string) error {
	if t.tree == nil {
		return errors.New("tree not loaded")
	}

	if parentID == nil {
		// The entry needs to be added to the root
		root := t.tree
		if root == nil {
			return errors.New("root not found")
		}

		// Generate a unique ID for the new leaf
		id, err := GenerateUniqueID()
		if err != nil {
			return fmt.Errorf("could not generate unique ID: %v", err)
		}

		entry := &TreeEntry{
			ID:       id,
			Title:    title,
			Parent:   root,
			Slug:     slug,
			Children: []*TreeEntry{},
		}

		if err := t.fsService.CreatePageEntry(root, entry); err != nil {
			return fmt.Errorf("could not create page entry: %v", err)
		}

		root.Children = append(root.Children, entry)

		// Store Tree after adding leaf
		return t.SaveTree()
	}

	// Find the parent leaf
	parent, err := t.FindPageEntryByID(t.tree.Children, *parentID)
	if err != nil {
		return errors.New("parent not found")
	}

	// Generate a unique ID for the new leaf
	id, err := GenerateUniqueID()
	if err != nil {
		return fmt.Errorf("could not generate unique ID: %v", err)
	}

	entry := &TreeEntry{
		ID:       id,
		Slug:     slug,
		Title:    title,
		Parent:   parent,
		Children: []*TreeEntry{},
	}

	if err := t.fsService.CreatePageEntry(parent, entry); err != nil {
		return fmt.Errorf("could not create page entry: %v", err)
	}

	// Add the new leaf to the parent
	parent.Children = append(parent.Children, entry)

	return t.SaveTree()
}

// FindPageEntryByID finds a leaf in the tree by its ID
// If the leaf is not found, it returns an error
func (t *TreeService) FindPageEntryByID(entry []*TreeEntry, id string) (*TreeEntry, error) {
	for _, e := range entry {
		if e.ID == id {
			return e, nil
		}

		if e.Children != nil {
			if leaf, err := t.FindPageEntryByID(e.Children, id); err == nil {
				return leaf, nil
			}
		}
	}

	return nil, errors.New("leaf not found")
}

// DeletePageEntry deletes a leaf from the tree
func (t *TreeService) DeletePageEntry(id string, recusive bool) error {
	if t.tree == nil {
		return errors.New("tree not loaded")
	}

	// Find the leaf to delete
	leaf, err := t.FindPageEntryByID(t.tree.Children, id)
	if err != nil {
		return errors.New("leaf not found")
	}

	// Check if leaf has children
	if len(leaf.Children) > 0 && !recusive {
		return errors.New("leaf has children")
	}

	// Delete the leaf from the parent
	parent := leaf.Parent
	if parent == nil {
		return errors.New("parent not found")
	}

	// Delete the leaf from the filesystem
	if err := t.fsService.DeletePageEntry(leaf); err != nil {
		return fmt.Errorf("could not delete page entry: %v", err)
	}

	// Remove the leaf from the parent
	for i, e := range parent.Children {
		if e.ID == id {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			break
		}
	}

	return t.SaveTree()
}

// UpdatePageEntry updates the title and slug of a leaf
func (t *TreeService) UpdatePageEntry(id string, title string, slug string, content string) error {
	if t.tree == nil {
		return errors.New("tree not loaded")
	}

	// Find the leaf to update
	leaf, err := t.FindPageEntryByID(t.tree.Children, id)
	if err != nil {
		return errors.New("leaf not found")
	}

	// Update the entry in the filesystem!
	if err := t.fsService.UpdatePageEntry(leaf, slug, content); err != nil {
		return fmt.Errorf("could not update page entry: %v", err)
	}

	// Update the leaf
	leaf.Title = title
	leaf.Slug = slug

	// Save the tree
	return t.SaveTree()
}

// GetTree returns the tree
func (t *TreeService) GetTree() *TreeEntry {
	return t.tree
}

// GetPageEntry returns a leaf by its ID
func (t *TreeService) GetPageEntry(id string) (*PageEntry, error) {
	if t.tree == nil {
		return nil, errors.New("tree not loaded")
	}

	// Find the leaf
	leaf, err := t.FindPageEntryByID(t.tree.Children, id)
	if err != nil {
		return nil, errors.New("leaf not found")
	}

	// Get the content of the leaf
	content, err := t.fsService.GetPageContent(leaf)
	if err != nil {
		return nil, fmt.Errorf("could not get page content: %v", err)
	}

	page := &PageEntry{
		TreeEntry: leaf,
		Content:   content,
	}

	return page, nil
}

// FindPageEntryByPath finds a leaf in the tree by its path
func (t *TreeService) FindPageEntryByRoutePath(entry []*TreeEntry, routePath string) (*PageEntry, error) {
	// Split the routePath into parts
	routePart := strings.Split(routePath, "/")
	// recursive function to find the entry
	var findEntry func(entry []*TreeEntry, routePart []string) (*PageEntry, error)
	findEntry = func(entry []*TreeEntry, routePart []string) (*PageEntry, error) {
		for _, e := range entry {
			if e.Slug == routePart[0] {
				if len(routePart) == 1 {
					// Get the content of the entry
					content, err := t.fsService.GetPageContent(e)
					if err != nil {
						return nil, fmt.Errorf("could not get page content: %v", err)
					}

					return &PageEntry{
						TreeEntry: e,
						Content:   content,
					}, nil
				}

				// Find the entry in the children
				return findEntry(e.Children, routePart[1:])
			}
		}

		return nil, errors.New("entry not found")
	}

	return findEntry(t.tree.Children, routePart)
}

// MovePageEntry moves a page to another parent
func (t *TreeService) MovePageEntry(id string, parentID string) error {
	if t.tree == nil {
		return errors.New("tree not loaded")
	}

	// Find the leaf to move
	leaf, err := t.FindPageEntryByID(t.tree.Children, id)
	if err != nil {
		return errors.New("leaf not found")
	}

	// Find the new parent
	newParent, err := t.FindPageEntryByID(t.tree.Children, parentID)
	if err != nil {
		return errors.New("new parent not found")
	}

	if err := t.fsService.MovePageEntry(leaf, newParent); err != nil {
		return fmt.Errorf("could not move page entry: %v", err)
	}

	// Move the leaf to the new parent
	// Remove the leaf from the old parent
	oldParent := leaf.Parent
	if oldParent == nil {
		return errors.New("old parent not found")
	}

	// Remove the leaf from the old parent
	for i, e := range oldParent.Children {
		if e.ID == id {
			oldParent.Children = append(oldParent.Children[:i], oldParent.Children[i+1:]...)
			break
		}
	}

	// Add the leaf to the new parent
	newParent.Children = append(newParent.Children, leaf)
	leaf.Parent = newParent

	// Save the tree
	return t.SaveTree()
}
