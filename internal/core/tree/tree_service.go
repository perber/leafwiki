package tree

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared"
)

// TreeService is our main component for handling tree operations
// We use this service to create pages, delete pages, update pages, etc.
type TreeService struct {
	storageDir string
	tree       *PageNode
	store      *NodeStore
	log        *slog.Logger

	mu sync.RWMutex
}

// NewTreeService creates a new TreeService
func NewTreeService(storageDir string) *TreeService {
	return &TreeService{
		storageDir: storageDir,
		tree:       nil,
		store:      NewNodeStore(storageDir),
		log:        slog.Default().With("component", "TreeService"),
	}
}

// LoadTree loads the tree from the storage directory
// If the tree does not exist, it creates a new tree
func (t *TreeService) LoadTree() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// One-time import from legacy tree.json into filesystem model.
	// Only runs if tree.json exists and root/ is still empty/unmigrated.
	if err := t.store.MigrateLegacyTreeJSONToFS(); err != nil {
		t.log.Error("legacy tree.json migration failed", "error", err)
		return err
	}

	// Load the tree from the storage directory
	var err error
	t.tree, err = t.store.ReconstructTreeFromFS()
	if err != nil {
		return err
	}

	return nil
}

func (t *TreeService) withLockedTree(fn func() error) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return fn()
}

func (t *TreeService) withRLockedTree(fn func() error) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return fn()
}

func (t *TreeService) ReloadProjection() error {
	return t.withLockedTree(t.reloadProjectionLocked)
}

func (t *TreeService) reloadProjectionLocked() error {
	newTree, err := t.store.ReconstructTreeFromFS()
	if err != nil {
		return fmt.Errorf("reconstruct tree from fs: %w", err)
	}
	if newTree == nil {
		return fmt.Errorf("internal error: reconstructed tree is nil")
	}
	t.tree = newTree
	return nil
}

// TreeHash returns the current hash of the tree
func (t *TreeService) TreeHash() string {
	var hash string
	_ = t.withRLockedTree(func() error {

		if t.tree == nil {
			hash = ""
			return nil
		}
		hash = t.tree.Hash()
		return nil
	})
	return hash
}

// ReconstructTreeFromFS reconstructs the tree from the filesystem
func (t *TreeService) ReconstructTreeFromFS() error {
	return t.withLockedTree(t.reconstructTreeFromFSLocked)
}

func (t *TreeService) reconstructTreeFromFSLocked() error {
	newTree, err := t.store.ReconstructTreeFromFS()
	if err != nil {
		t.log.Error("Error reconstructing tree from filesystem", "error", err)
		return err
	}

	// Defensive check to protect against unexpected nil returns from ReconstructTreeFromFS
	if newTree == nil {
		return fmt.Errorf("internal error: ReconstructTreeFromFS returned nil tree")
	}

	t.tree = newTree

	return nil
}

// Create Node adds a new node to the tree
func (t *TreeService) CreateNode(userID string, parentID *string, title string, slug string, nodeKind *NodeKind) (*string, error) {
	var result *string
	err := t.withLockedTree(func() error {
		var err error
		result, err = t.createNodeLocked(userID, parentID, title, slug, nodeKind)
		return err
	})

	return result, err
}

// createNodeLocked creates a new node under the given parent
// Lock must be held by the caller
func (t *TreeService) createNodeLocked(userID string, parentID *string, title string, slug string, kind *NodeKind) (*string, error) {
	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

	// Decide which kind we create
	k := NodeKindPage
	if kind != nil {
		k = *kind
	}

	// Resolve the parent
	parent := t.tree
	if parentID != nil && *parentID != "" && *parentID != "root" {
		var err error
		parent, err = t.findPageByIDLocked(t.tree.Children, *parentID)
		if err != nil {
			return nil, ErrParentNotFound
		}
	}

	// Check if a child with the same slug already exists
	if parent.ChildAlreadyExists(slug) {
		return nil, ErrPageAlreadyExists
	}

	// Check if the current parent is a section
	// if not, we need to convert it to a section
	if parent.Kind != NodeKindSection && parent.ID != "root" {
		t.log.Info("converting parent to section", "parentID", parent.ID, "oldKind", parent.Kind, "newKind", NodeKindSection)
		if err := t.store.ConvertNode(parent, NodeKindSection); err != nil {
			return nil, fmt.Errorf("could not convert parent node: %w", err)
		}
		// Transitional in-memory update; authoritative kind comes from reloadProjectionLocked().
		parent.Kind = NodeKindSection
	}

	if parent.Kind != NodeKindSection {
		return nil, fmt.Errorf("cannot add child to non-section parent, got %q", parent.Kind)
	}

	// Generate a unique ID for the new page
	id, err := shared.GenerateUniqueID()
	if err != nil {
		return nil, fmt.Errorf("could not generate unique ID: %w", err)
	}

	now := time.Now().UTC()

	entry := &PageNode{
		ID:       id,
		Title:    title,
		Parent:   parent,
		Slug:     slug,
		Kind:     k,
		Children: []*PageNode{},
		Metadata: PageMetadata{
			CreatedAt:    now,
			UpdatedAt:    now,
			CreatorID:    userID,
			LastAuthorID: userID,
		},
	}

	// Create on disk depending on kind
	switch k {
	case NodeKindPage:
		if err := t.store.CreatePage(parent, entry); err != nil {
			return nil, fmt.Errorf("could not create page entry: %w", err)
		}
	case NodeKindSection:
		if err := t.store.CreateSection(parent, entry); err != nil {
			return nil, fmt.Errorf("could not create section entry: %w", err)
		}
	}

	// Add the new page to the parent
	if err := t.store.AppendChildOrder(parent, entry.ID); err != nil {
		return nil, fmt.Errorf("could not append order entry: %w", err)
	}

	if err := t.reloadProjectionLocked(); err != nil {
		return nil, fmt.Errorf("could not reload tree projection: %w", err)
	}
	return &entry.ID, nil
}

// FindPageByID finds a page in the tree by its ID
// If the page is not found, it returns an error
func (t *TreeService) FindPageByID(id string) (*PageNode, error) {
	var result *PageNode
	err := t.withRLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}
		var err error
		result, err = t.findPageByIDLocked(t.tree.Children, id)
		return err
	})

	return result, err
}

// findPageByIDLocked finds a page in the tree by its ID
// Lock must be held by the caller
func (t *TreeService) findPageByIDLocked(entry []*PageNode, id string) (*PageNode, error) {
	for _, e := range entry {
		if e.ID == id {
			return e, nil
		}

		if e.Children != nil {
			if page, err := t.findPageByIDLocked(e.Children, id); err == nil {
				return page, nil
			}
		}
	}
	return nil, ErrPageNotFound
}

// DeleteNode deletes a node from the tree
func (t *TreeService) DeleteNode(userID string, id string, recursive bool) error {
	err := t.withLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}

		// Find the node to delete
		node, err := t.findPageByIDLocked(t.tree.Children, id)
		if err != nil {
			return ErrPageNotFound
		}

		// Check if node has children
		if node.HasChildren() && !recursive {
			return ErrPageHasChildren
		}

		// Delete the node from the parent
		parent := node.Parent
		if parent == nil {
			return ErrParentNotFound
		}

		switch node.Kind {
		case NodeKindSection:
			if err := t.store.DeleteSection(node); err != nil {
				return fmt.Errorf("could not delete section entry: %w", err)
			}
		case NodeKindPage:
			if node.HasChildren() {
				return fmt.Errorf("invalid projection: page node %q has children", node.ID)
			} else {
				if err := t.store.DeletePage(node); err != nil {
					return fmt.Errorf("could not delete page entry: %w", err)
				}
			}
		default:
			return fmt.Errorf("unknown node kind: %v", node.Kind)
		}

		if err := t.store.RemoveChildOrder(parent, id); err != nil {
			return fmt.Errorf("could not update order.json after delete: %w", err)
		}

		return t.reloadProjectionLocked()

	})
	return err
}

// UpdateNode updates a node (page/section) in the tree and syncs disk state via NodeStore.
func (t *TreeService) UpdateNode(userID string, id string, title string, slug string, content *string) error {
	return t.withLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}

		// Find node
		node, err := t.findPageByIDLocked(t.tree.Children, id)
		if err != nil {
			return ErrPageNotFound
		}

		// Slug must be unique under same parent (when changed)
		if slug != node.Slug && node.Parent != nil && node.Parent.ChildAlreadyExists(slug) {
			return ErrPageAlreadyExists
		}

		// Content update?
		if content != nil {
			t.log.Info("updating node content", "nodeID", node.ID)
			if err := t.store.UpsertContent(node, *content); err != nil {
				return fmt.Errorf("could not upsert content: %w", err)
			}
		}

		// Rename slug on disk (must happen while node still has old slug)
		if slug != node.Slug {
			t.log.Info("renaming node slug", "nodeID", node.ID, "oldSlug", node.Slug, "newSlug", slug)
			if err := t.store.RenameNode(node, slug); err != nil {
				return fmt.Errorf("could not rename node: %w", err)
			}
			node.Slug = slug
		}

		// Update title in tree
		node.Title = title

		// Update metadata
		node.Metadata.UpdatedAt = time.Now().UTC()
		node.Metadata.LastAuthorID = userID

		if err := t.store.WriteNodeFrontmatter(node); err != nil {
			return fmt.Errorf("could not write node frontmatter: %w", err)
		}

		return t.reloadProjectionLocked()
	})

}

func (t *TreeService) ConvertNode(userID string, id string, kind NodeKind) error {
	return t.withLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}

		// Find node
		node, err := t.findPageByIDLocked(t.tree.Children, id)
		if err != nil {
			return ErrPageNotFound
		}

		if node.Kind == kind {
			// No change
			return nil
		}

		// Explicit kind conversion is no longer a primary domain operation.
		// Kind is derived from FS representation.
		// We still keep a transitional implementation for controlled cases.
		if node.Kind == NodeKindSection && kind == NodeKindPage && node.HasChildren() {
			return ErrPageHasChildren
		}

		t.log.Info("changing node kind", "nodeID", node.ID, "oldKind", node.Kind, "newKind", kind)

		if err := t.store.ConvertNode(node, kind); err != nil {
			return fmt.Errorf("could not convert node: %w", err)
		}
		node.Metadata.UpdatedAt = time.Now().UTC()
		node.Metadata.LastAuthorID = userID

		if err := t.store.WriteNodeFrontmatter(node); err != nil {
			return fmt.Errorf("could not write node frontmatter after convert: %w", err)
		}

		return t.reloadProjectionLocked()
	})
}

// GetTree returns the tree
func (t *TreeService) GetTree() *PageNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.tree
}

// GetPage returns a page by its ID
func (t *TreeService) GetPage(id string) (*Page, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

	// Find the page
	page, err := t.findPageByIDLocked(t.tree.Children, id)
	if err != nil {
		return nil, ErrPageNotFound
	}

	// Get the content of the page
	content, err := t.store.ReadPageContent(page)
	if err != nil {
		return nil, fmt.Errorf("could not get page content: %w", err)
	}

	return &Page{
		PageNode: page,
		Content:  content,
	}, nil
}

// FindPageByRoutePath finds a page in the tree by its path
func (t *TreeService) FindPageByRoutePath(entry []*PageNode, routePath string) (*Page, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	routePath = strings.TrimSpace(routePath)
	routePath = strings.Trim(routePath, "/")
	if routePath == "" {
		return nil, ErrPageNotFound
	}

	// Split the routePath into parts
	routePart := strings.Split(routePath, "/")
	// recursive function to find the entry
	var findEntry func(entry []*PageNode, routePart []string) (*Page, error)
	findEntry = func(entry []*PageNode, routePart []string) (*Page, error) {
		if len(routePart) == 0 {
			return nil, ErrPageNotFound
		}
		for _, e := range entry {
			if e.Slug == routePart[0] {
				if len(routePart) == 1 {
					// Get the content of the entry
					content, err := t.store.ReadPageContent(e)
					if err != nil {
						return nil, fmt.Errorf("could not get page content: %w", err)
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

// LookupPagePath looks up a path in the tree and returns a PathLookup struct
// that contains information about the path and its segments and whether they exist
func (t *TreeService) LookupPagePath(entry []*PageNode, p string) (*PathLookup, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.LookupPagePathLocked(entry, p)
}

// LookupPagePathLocked looks up a path in the tree and returns a PathLookup struct
// that contains information about the path and its segments and whether they exist
// Lock must be held by the caller
func (t *TreeService) LookupPagePathLocked(entry []*PageNode, p string) (*PathLookup, error) {
	path := strings.TrimSpace(p)
	path = strings.Trim(path, "/")
	if path == "" {
		return &PathLookup{
			Path:     path,
			Segments: []PathSegment{},
			Exists:   false,
		}, nil
	}

	// remove double slashes
	path = strings.ReplaceAll(path, "//", "/")

	// Split the path into parts
	pathParts := strings.Split(path, "/")
	if len(pathParts) == 0 {
		return &PathLookup{
			Path:     path,
			Segments: []PathSegment{},
			Exists:   false,
		}, nil
	}

	lookup := &PathLookup{
		Path:     path,
		Segments: make([]PathSegment, len(pathParts)),
		Exists:   true,
	}

	// Check each segment in the path
	for i, part := range pathParts {
		if part == "" || part == "." || part == ".." {
			return nil, fmt.Errorf("invalid path segment: %q", part)
		}

		// Find the segment in the tree
		segment := PathSegment{
			Slug:   part,
			Exists: false,
		}

		// push the segment to the lookup
		lookup.Segments[i] = segment

		// Check if the segment exists in the current entry
		for _, e := range entry {
			if e.Slug == part {
				// Segment exists
				lookup.Segments[i].Exists = true
				lookup.Segments[i].ID = &e.ID
				lookup.Segments[i].Kind = &e.Kind
				lookup.Segments[i].Title = &e.Title

				// Move to the next entry
				entry = e.Children
				break
			}
		}

		// If the segment does not exist, set the pathExists flag to false
		if !lookup.Segments[i].Exists {
			// No need to check further segments
			// Set all remaining segments to non-existing
			for j := i + 1; j < len(pathParts); j++ {
				lookup.Segments[j] = PathSegment{
					Slug:   pathParts[j],
					Exists: false,
				}
			}

			lookup.Exists = false

			// Set entry to nil to avoid further checks
			entry = nil
		}
	}

	return lookup, nil
}

// EnsurePagePath ensures that a given path exists in the tree
// It creates any missing segments as needed
// Returns the final page node and a list of created nodes
func (t *TreeService) EnsurePagePath(userID string, p string, targetTitle string, kind *NodeKind) (*EnsurePathResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

	created := []*PageNode{}

	lookup, err := t.LookupPagePathLocked(t.tree.Children, p)
	if err != nil {
		return nil, fmt.Errorf("could not lookup page path: %w", err)
	}

	// Path exists -> return existing
	if lookup.Exists {
		last := lookup.Segments[len(lookup.Segments)-1]
		page, err := t.findPageByIDLocked(t.tree.Children, *last.ID)
		if err != nil {
			return nil, fmt.Errorf("could not find existing page by ID: %w", err)
		}
		return &EnsurePathResult{Exists: true, Page: page}, nil
	}

	// Create missing segments
	var currentID *string // nil means root
	for i, segment := range lookup.Segments {
		if segment.Exists {
			currentID = segment.ID
			continue
		}

		// Title
		segTitle := segment.Slug
		if i == len(lookup.Segments)-1 {
			segTitle = targetTitle
		}

		// Kind: intermediate segments are sections, last segment uses provided kind (or page/section default)
		kindToUse := NodeKindSection
		if i == len(lookup.Segments)-1 && kind != nil {
			kindToUse = *kind
		}

		newID, err := t.createNodeLocked(userID, currentID, segTitle, segment.Slug, &kindToUse)
		if err != nil {
			return nil, fmt.Errorf("could not create segment %q: %w", segment.Slug, err)
		}
		currentID = newID

		created = append(created, &PageNode{
			ID:    *newID,
			Slug:  segment.Slug,
			Title: segTitle,
			Kind:  kindToUse,
		})
	}

	// Resolve final page
	if currentID == nil {
		return nil, fmt.Errorf("could not ensure page path")
	}
	page, err := t.findPageByIDLocked(t.tree.Children, *currentID)
	if err != nil {
		return nil, fmt.Errorf("could not find created page by ID: %w", err)
	}

	return &EnsurePathResult{
		Exists:  true,
		Page:    page,
		Created: created,
	}, nil
}

// MoveNode moves a node to another parent (root if parentID is empty/"root")
func (t *TreeService) MoveNode(userID string, id string, parentID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	// Find node to move
	node, err := t.findPageByIDLocked(t.tree.Children, id)
	if err != nil {
		return ErrPageNotFound
	}

	// Resolve destination parent (default root)
	newParent := t.tree
	if parentID != "" && parentID != "root" {
		newParent, err = t.findPageByIDLocked(t.tree.Children, parentID)
		if err != nil {
			return fmt.Errorf("new parent not found: %w", ErrParentNotFound)
		}
	}

	// Same slug collision under new parent
	if newParent.ChildAlreadyExists(node.Slug) {
		return fmt.Errorf("child with the same slug already exists: %w", ErrPageAlreadyExists)
	}

	// Can't move into itself
	if node.ID == newParent.ID {
		return fmt.Errorf("page cannot be moved to itself: %w", ErrPageCannotBeMovedToItself)
	}

	// Circular reference guard: node cannot be moved under its own descendants
	if node.HasDescendant(newParent.ID, true) {
		return fmt.Errorf("circular reference detected: %w", ErrMovePageCircularReference)
	}

	// If destination parent is a PAGE, auto-convert it to SECTION so it can host children
	if newParent.ID != "root" && newParent.Kind == NodeKindPage {
		if err := t.store.ConvertNode(newParent, NodeKindSection); err != nil {
			return fmt.Errorf("could not auto-convert new parent page to section: %w", err)
		}
		// Transitional in-memory update; authoritative kind comes from reloadProjectionLocked().
		newParent.Kind = NodeKindSection
	}

	// Defensive: after possible conversion, destination must be a section
	if newParent.Kind != NodeKindSection {
		return fmt.Errorf("destination parent must be a section, got %q", newParent.Kind)
	}

	oldParent := node.Parent
	if oldParent == nil {
		return fmt.Errorf("old parent not found: %w", ErrParentNotFound)
	}

	// Move on disk (strict by node.Kind inside NodeStore)
	if err := t.store.MoveNode(node, newParent); err != nil {
		return fmt.Errorf("could not move node on disk: %w", err)
	}

	if err := t.store.RemoveChildOrder(oldParent, node.ID); err != nil {
		return fmt.Errorf("could not update old parent order.json: %w", err)
	}
	if err := t.store.AppendChildOrder(newParent, node.ID); err != nil {
		return fmt.Errorf("could not update new parent order.json: %w", err)
	}
	// Temporary parent update so WriteNodeFrontmatter resolves the new on-disk path.
	// The authoritative projection is rebuilt immediately afterwards.
	node.Parent = newParent
	node.Metadata.UpdatedAt = time.Now().UTC()
	node.Metadata.LastAuthorID = userID

	if err := t.store.WriteNodeFrontmatter(node); err != nil {
		return fmt.Errorf("could not write moved node frontmatter: %w", err)
	}

	return t.reloadProjectionLocked()
}

func (t *TreeService) SortPages(parentID string, orderedIDs []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	parent := t.tree

	if parentID != "" && parentID != "root" {
		// Find the parent page
		var err error
		parent, err = t.findPageByIDLocked(t.tree.Children, parentID)
		if err != nil {
			return ErrParentNotFound
		}
	}

	if parent.Kind != NodeKindSection {
		return fmt.Errorf("cannot sort children of non-section parent %q", parent.Kind)
	}

	// Check if all IDs in the sort order are valid
	existingIDs := make(map[string]bool, len(parent.Children))
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

	if err := t.store.WriteChildOrder(parent, orderedIDs); err != nil {
		return fmt.Errorf("could not write order.json: %w", err)
	}

	return t.reloadProjectionLocked()
}
