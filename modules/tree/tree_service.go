package tree

import (
	"errors"
	"fmt"
	"strings"
)

// PageNode represents a single node in the tree
// It has an ID, a parent, a path, and children
// The ID is a unique identifier for the entry
type PageNode struct {
	ID       string      `json:"id"`       // Unique identifier for the entry
	Title    string      `json:"title"`    // Title is the name of the entry
	Slug     string      `json:"slug"`     // Slug is the path of the entry
	Children []*PageNode `json:"children"` // Children are the children of the entry
	Parent   *PageNode   `json:"-"`
}

type Page struct {
	*PageNode
	Content string `json:"content"`
}

// TreeService is our main component for handling tree operations
// We use this service to create pages, delete pages, update pages, etc.
type TreeService struct {
	storageDir   string
	treeFilename string
	tree         *PageNode
	store        *PageStore
}

// NewTreeService creates a new TreeService
func NewTreeService(storageDir string) *TreeService {
	return &TreeService{
		storageDir:   storageDir,
		treeFilename: "tree.json",
		tree:         nil,
		store:        NewPageStore(storageDir),
	}
}

// LoadTree loads the tree from the storage directory
// If the tree does not exist, it creates a new tree
func (t *TreeService) LoadTree() error {
	// Load the tree from the storage directory
	var err error
	t.tree, err = t.store.LoadTree(t.treeFilename)
	return err
}

func (t *TreeService) SaveTree() error {
	// Save the tree to the storage directory
	return t.store.SaveTree(t.treeFilename, t.tree)
}

// Create Page adds a new page to the tree
func (t *TreeService) CreatePage(parentID *string, title string, slug string) error {
	if t.tree == nil {
		return errors.New("tree not loaded")
	}

	if parentID == nil {
		// The entry needs to be added to the root
		root := t.tree
		if root == nil {
			return errors.New("root not found")
		}

		// Generate a unique ID for the new page
		id, err := GenerateUniqueID()
		if err != nil {
			return fmt.Errorf("could not generate unique ID: %v", err)
		}

		entry := &PageNode{
			ID:       id,
			Title:    title,
			Parent:   root,
			Slug:     slug,
			Children: []*PageNode{},
		}

		if err := t.store.CreatePage(root, entry); err != nil {
			return fmt.Errorf("could not create page entry: %v", err)
		}

		root.Children = append(root.Children, entry)

		// Store Tree after adding page
		return t.SaveTree()
	}

	// Find the parent page
	parent, err := t.FindPageByID(t.tree.Children, *parentID)
	if err != nil {
		return errors.New("parent not found")
	}

	// Generate a unique ID for the new page
	id, err := GenerateUniqueID()
	if err != nil {
		return fmt.Errorf("could not generate unique ID: %v", err)
	}

	entry := &PageNode{
		ID:       id,
		Slug:     slug,
		Title:    title,
		Parent:   parent,
		Children: []*PageNode{},
	}

	if err := t.store.CreatePage(parent, entry); err != nil {
		return fmt.Errorf("could not create page entry: %v", err)
	}

	// Add the new page to the parent
	parent.Children = append(parent.Children, entry)

	return t.SaveTree()
}

// FindPageByID finds a page in the tree by its ID
// If the page is not found, it returns an error
func (t *TreeService) FindPageByID(entry []*PageNode, id string) (*PageNode, error) {
	for _, e := range entry {
		if e.ID == id {
			return e, nil
		}

		if e.Children != nil {
			if page, err := t.FindPageByID(e.Children, id); err == nil {
				return page, nil
			}
		}
	}

	return nil, errors.New("page not found")
}

// DeletePage deletes a page from the tree
func (t *TreeService) DeletePage(id string, recusive bool) error {
	if t.tree == nil {
		return errors.New("tree not loaded")
	}

	// Find the page to delete
	page, err := t.FindPageByID(t.tree.Children, id)
	if err != nil {
		return errors.New("page not found")
	}

	// Check if page has children
	if len(page.Children) > 0 && !recusive {
		return errors.New("page has children")
	}

	// Delete the page from the parent
	parent := page.Parent
	if parent == nil {
		return errors.New("parent not found")
	}

	// Delete the page from the filesystem
	if err := t.store.DeletePage(page); err != nil {
		return fmt.Errorf("could not delete page entry: %v", err)
	}

	// Remove the page from the parent
	for i, e := range parent.Children {
		if e.ID == id {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			break
		}
	}

	return t.SaveTree()
}

// UpdatePage updates a page in the tree
func (t *TreeService) UpdatePage(id string, title string, slug string, content string) error {
	if t.tree == nil {
		return errors.New("tree not loaded")
	}

	// Find the page to update
	page, err := t.FindPageByID(t.tree.Children, id)
	if err != nil {
		return errors.New("page not found")
	}

	// Update the entry in the filesystem!
	if err := t.store.UpdatePage(page, slug, content); err != nil {
		return fmt.Errorf("could not update page entry: %v", err)
	}

	// Update the page
	page.Title = title
	page.Slug = slug

	// Save the tree
	return t.SaveTree()
}

// GetTree returns the tree
func (t *TreeService) GetTree() *PageNode {
	return t.tree
}

// GetPage returns a page by its ID
func (t *TreeService) GetPage(id string) (*Page, error) {
	if t.tree == nil {
		return nil, errors.New("tree not loaded")
	}

	// Find the page
	page, err := t.FindPageByID(t.tree.Children, id)
	if err != nil {
		return nil, errors.New("page not found")
	}

	// Get the content of the page
	content, err := t.store.ReadPageContent(page)
	if err != nil {
		return nil, fmt.Errorf("could not get page content: %v", err)
	}

	return &Page{
		PageNode: page,
		Content:  content,
	}, nil
}

// FindPageByPath finds a page in the tree by its path
func (t *TreeService) FindPageByRoutePath(entry []*PageNode, routePath string) (*Page, error) {
	// Split the routePath into parts
	routePart := strings.Split(routePath, "/")
	// recursive function to find the entry
	var findEntry func(entry []*PageNode, routePart []string) (*Page, error)
	findEntry = func(entry []*PageNode, routePart []string) (*Page, error) {
		for _, e := range entry {
			if e.Slug == routePart[0] {
				if len(routePart) == 1 {
					// Get the content of the entry
					content, err := t.store.ReadPageContent(e)
					if err != nil {
						return nil, fmt.Errorf("could not get page content: %v", err)
					}

					return &Page{
						PageNode: e,
						Content:  content,
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

// MovePage moves a page to another parent
func (t *TreeService) MovePage(id string, parentID string) error {
	if t.tree == nil {
		return errors.New("tree not loaded")
	}

	// Find the page to move
	page, err := t.FindPageByID(t.tree.Children, id)
	if err != nil {
		return errors.New("page not found")
	}

	// Find the new parent
	newParent, err := t.FindPageByID(t.tree.Children, parentID)
	if err != nil {
		return errors.New("new parent not found")
	}

	if err := t.store.MovePage(page, newParent); err != nil {
		return fmt.Errorf("could not move page entry: %v", err)
	}

	// Move the page to the new parent
	// Remove the page from the old parent
	oldParent := page.Parent
	if oldParent == nil {
		return errors.New("old parent not found")
	}

	// Remove the page from the old parent
	for i, e := range oldParent.Children {
		if e.ID == id {
			oldParent.Children = append(oldParent.Children[:i], oldParent.Children[i+1:]...)
			break
		}
	}

	// Add the page to the new parent
	newParent.Children = append(newParent.Children, page)
	page.Parent = newParent

	// Save the tree
	return t.SaveTree()
}
