package tree

import (
	"fmt"
	"sort"
	"strings"

	"github.com/perber/wiki/internal/core/shared"
)

// PageNode represents a single node in the tree
// It has an ID, a parent, a path, and children
// The ID is a unique identifier for the entry
type PageNode struct {
	ID       string      `json:"id"`       // Unique identifier for the entry
	Title    string      `json:"title"`    // Title is the name of the entry
	Slug     string      `json:"slug"`     // Slug is the path of the entry
	Children []*PageNode `json:"children"` // Children are the children of the entry
	Position int         `json:"position"` // Position is the position of the entry
	Parent   *PageNode   `json:"-"`
}

func (p *PageNode) HasChildren() bool {
	return len(p.Children) > 0
}

func (p *PageNode) ChildAlreadyExists(slug string) bool {
	for _, child := range p.Children {
		if child.Slug == slug {
			return true
		}
	}
	return false
}

func (p *PageNode) IsChildOf(childID string, recusive bool) bool {
	for _, child := range p.Children {
		if child.ID == childID {
			return true
		}
		if recusive && child.IsChildOf(childID, recusive) {
			return true
		}
	}
	return false
}

func (p *PageNode) CalculatePath() string {
	// Calculate the path of the entry
	// The path is the slug of the entry and its parent's path
	if p.Parent == nil {
		if p.Slug == "" || p.Slug == "root" {
			return ""
		}
		return p.Slug
	}
	return p.Parent.CalculatePath() + "/" + p.Slug
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
func (t *TreeService) CreatePage(parentID *string, title string, slug string) (*string, error) {
	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

	if parentID == nil {
		// The entry needs to be added to the root
		root := t.tree
		if root == nil {
			return nil, ErrParentNotFound
		}

		if root.ChildAlreadyExists(slug) {
			return nil, ErrPageAlreadyExists
		}

		// Generate a unique ID for the new page
		id, err := shared.GenerateUniqueID()
		if err != nil {
			return nil, fmt.Errorf("could not generate unique ID: %v", err)
		}

		entry := &PageNode{
			ID:       id,
			Title:    title,
			Parent:   root,
			Slug:     slug,
			Position: len(root.Children), // Set the position to the end of the list
			Children: []*PageNode{},
		}

		if err := t.store.CreatePage(root, entry); err != nil {
			return nil, fmt.Errorf("could not create page entry: %v", err)
		}

		root.Children = append(root.Children, entry)

		// Store Tree after adding page
		if err := t.SaveTree(); err != nil {
			return nil, fmt.Errorf("could not save tree: %v", err)
		}
		return &entry.ID, nil
	}

	// Find the parent page
	parent, err := t.FindPageByID(t.tree.Children, *parentID)
	if err != nil {
		return nil, ErrParentNotFound
	}

	if parent.ChildAlreadyExists(slug) {
		return nil, ErrPageAlreadyExists
	}

	// Generate a unique ID for the new page
	id, err := shared.GenerateUniqueID()
	if err != nil {
		return nil, fmt.Errorf("could not generate unique ID: %v", err)
	}

	entry := &PageNode{
		ID:       id,
		Slug:     slug,
		Title:    title,
		Parent:   parent,
		Position: len(parent.Children), // Set the position to the end of the list
		Children: []*PageNode{},
	}

	if err := t.store.CreatePage(parent, entry); err != nil {
		return nil, fmt.Errorf("could not create page entry: %v", err)
	}

	// Add the new page to the parent
	parent.Children = append(parent.Children, entry)

	if err := t.SaveTree(); err != nil {
		return nil, fmt.Errorf("could not save tree: %v", err)
	}

	return &entry.ID, nil
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

	return nil, ErrPageNotFound
}

// DeletePage deletes a page from the tree
func (t *TreeService) DeletePage(id string, recusive bool) error {
	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	// Find the page to delete
	page, err := t.FindPageByID(t.tree.Children, id)
	if err != nil {
		return ErrPageNotFound
	}

	// Check if page has children
	if page.HasChildren() && !recusive {
		return ErrPageHasChildren
	}

	// Delete the page from the parent
	parent := page.Parent
	if parent == nil {
		return ErrParentNotFound
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

	t.ReindexPositions(parent)

	return t.SaveTree()
}

// UpdatePage updates a page in the tree
func (t *TreeService) UpdatePage(id string, title string, slug string, content string) error {
	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	// Find the page to update
	page, err := t.FindPageByID(t.tree.Children, id)
	if err != nil {
		return ErrPageNotFound
	}

	// Check if the slug is unique when slug changes!
	if slug != page.Slug && page.Parent.ChildAlreadyExists(slug) {
		return ErrPageAlreadyExists
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
	if t.tree != nil {
		t.sortTreeByPosition(t.tree)
	}
	return t.tree
}

// GetPage returns a page by its ID
func (t *TreeService) GetPage(id string) (*Page, error) {
	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

	// Find the page
	page, err := t.FindPageByID(t.tree.Children, id)
	if err != nil {
		return nil, ErrPageNotFound
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

		return nil, ErrPageNotFound
	}

	return findEntry(t.tree.Children, routePart)
}

// MovePage moves a page to another parent
func (t *TreeService) MovePage(id string, parentID string) error {
	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	// Find the page to move
	page, err := t.FindPageByID(t.tree.Children, id)
	if err != nil {
		return ErrPageNotFound
	}

	// We think that the page is moved to the root
	newParent := t.tree

	// Check if a parentID is provided
	if parentID != "" && parentID != "root" {
		// Find the new parent
		newParent, err = t.FindPageByID(t.tree.Children, parentID)
		if err != nil {
			return fmt.Errorf("new parent not found: %w", ErrParentNotFound)
		}
	}

	// Child with the same slug already exists
	if newParent.ChildAlreadyExists(page.Slug) {
		return fmt.Errorf("child with the same slug already exists: %w", ErrPageAlreadyExists)
	}

	// Check if the page is not moved to itself
	if page.ID == newParent.ID {
		return fmt.Errorf("page cannot be moved to itself: %w", ErrPageCannotBeMovedToItself)
	}

	// Check if a circular reference is created
	if page.IsChildOf(newParent.ID, true) {
		return fmt.Errorf("circular reference detected: %w", ErrMovePageCircularReference)
	}

	// Move the page in the filesystem
	if err := t.store.MovePage(page, newParent); err != nil {
		return fmt.Errorf("could not move page entry: %w", err)
	}

	// Move the page to the new parent
	// Remove the page from the old parent
	oldParent := page.Parent
	if oldParent == nil {
		return fmt.Errorf("old parent not found: %w", ErrParentNotFound)
	}

	// Remove the page from the old parent
	for i, e := range oldParent.Children {
		if e.ID == id {
			oldParent.Children = append(oldParent.Children[:i], oldParent.Children[i+1:]...)
			break
		}
	}

	// Add the page to the new parent
	page.Position = len(newParent.Children)
	newParent.Children = append(newParent.Children, page)
	page.Parent = newParent
	// Reindex the positions of the old parent
	t.ReindexPositions(newParent)
	t.ReindexPositions(oldParent)

	// Save the tree
	return t.SaveTree()
}

func (t *TreeService) SortPages(parentID string, orderedIDs []string) error {
	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	parent := t.tree

	if parentID != "" && parentID != "root" {
		// Find the parent page
		var err error
		parent, err = t.FindPageByID(t.tree.Children, parentID)
		if err != nil {
			return ErrParentNotFound
		}
	}

	// Check if the number of orderedIDs is the same as the number of children
	if len(orderedIDs) != len(parent.Children) {
		return fmt.Errorf("number of ordered IDs does not match the number of children: %w", ErrInvalidSortOrder)
	}

	// Check if all IDs in the sort order are valid
	existingIDs := make(map[string]bool)
	for _, child := range parent.Children {
		existingIDs[child.ID] = true
	}
	for _, id := range orderedIDs {
		if !existingIDs[id] {
			return fmt.Errorf("invalid ID in sort order, ID: %s - %w", id, ErrInvalidSortOrder)
		}
	}

	seen := make(map[string]bool)
	for _, id := range orderedIDs {
		if seen[id] {
			return fmt.Errorf("duplicate ID in sort order: %s", id)
		}
		seen[id] = true
	}

	// Create a map to store the position of each page
	positions := make(map[string]int)
	for i, id := range orderedIDs {
		positions[id] = i
	}

	// Sort the children of the parent
	sort.SliceStable(parent.Children, func(i, j int) bool {
		return positions[parent.Children[i].ID] < positions[parent.Children[j].ID]
	})

	// write postion index to children
	for i, child := range parent.Children {
		child.Position = i
	}

	// Reindex the positions
	t.ReindexPositions(parent)

	// Save the tree
	return t.SaveTree()
}

func (t *TreeService) ReindexPositions(parent *PageNode) {
	sort.SliceStable(parent.Children, func(i, j int) bool {
		return parent.Children[i].Position < parent.Children[j].Position
	})
	for i, child := range parent.Children {
		child.Position = i
	}
}

func (t *TreeService) sortTreeByPosition(node *PageNode) {
	sort.SliceStable(node.Children, func(i, j int) bool {
		return node.Children[i].Position < node.Children[j].Position
	})
	for _, child := range node.Children {
		t.sortTreeByPosition(child)
	}
}
