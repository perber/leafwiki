package tree

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared"
	"github.com/perber/wiki/internal/core/treemigration"
)

// TreeService is our main component for handling tree operations
// We use this service to create pages, delete pages, update pages, etc.
type TreeService struct {
	storageDir string
	tree       *PageNode
	store      *NodeStore
	log        *slog.Logger
	nodesByID  map[string]*PageNode
	childSlugs map[string]map[string]*PageNode

	mu sync.RWMutex
}

// NewTreeService creates a new TreeService
const legacyTreeFilename = "tree.json"

func NewTreeService(storageDir string) *TreeService {
	return &TreeService{
		storageDir: storageDir,
		tree:       nil,
		store:      NewNodeStore(storageDir),
		log:        slog.Default().With("component", "TreeService"),
		nodesByID:  make(map[string]*PageNode),
		childSlugs: make(map[string]map[string]*PageNode),
	}
}

// LoadTree reconstructs the in-memory tree from the filesystem.
// Legacy tree.json data is only used as a migration source for older schema versions.
func (t *TreeService) LoadTree() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.log.Info("Checking schema version...")
	schema, err := loadSchema(t.storageDir)
	if err != nil {
		t.log.Error("Error loading schema", "error", err)
		return err
	}

	if schema.Version == CurrentSchemaVersion {
		reconstructed, err := t.store.ReconstructTreeFromFS()
		if err != nil {
			return err
		}
		if reconstructed == nil {
			return fmt.Errorf("internal error: tree reconstruction returned nil tree")
		}
		t.tree = reconstructed
		t.rebuildIndexesLocked()
		return nil
	}

	legacyTreePath := filepath.Join(t.storageDir, legacyTreeFilename)
	if info, statErr := os.Stat(legacyTreePath); statErr == nil && !info.IsDir() {
		legacyTree, legacyErr := t.store.LoadTree(legacyTreeFilename)
		if legacyErr != nil {
			t.log.Warn("Could not load legacy tree, falling back to filesystem reconstruction", "path", legacyTreePath, "error", legacyErr)
			t.tree, err = t.store.ReconstructTreeFromFS()
			if err != nil {
				return err
			}
		} else {
			t.tree = legacyTree
		}
	} else {
		t.tree, err = t.store.ReconstructTreeFromFS()
		if err != nil {
			return err
		}
	}

	if t.tree == nil {
		return fmt.Errorf("internal error: tree reconstruction returned nil tree")
	}

	t.log.Info("Migrating schema", "fromVersion", schema.Version, "toVersion", CurrentSchemaVersion)
	if err := treemigration.Run(schema.Version, t.migrationDependencies()); err != nil {
		t.log.Error("Error migrating schema", "error", err)
		return err
	}

	reconstructed, err := t.store.ReconstructTreeFromFS()
	if err != nil {
		return err
	}
	if reconstructed == nil {
		return fmt.Errorf("internal error: tree reconstruction returned nil tree")
	}

	t.tree = reconstructed
	t.rebuildIndexesLocked()

	if err := os.Remove(legacyTreePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.log.Warn("Could not remove migrated legacy tree snapshot", "path", legacyTreePath, "error", err)
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

// TreeHash returns the current hash of the tree
func (t *TreeService) TreeHash() string {
	var hash string
	_ = t.withRLockedTree(func() error {
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
	// Reconstruct the tree from the filesystem
	// This is a more complex operation and may involve reading the filesystem structure
	newTree, err := t.store.ReconstructTreeFromFS()
	if err != nil {
		t.log.Error("Error reconstructing tree from filesystem", "error", err)
		return err
	}

	// Defensive check to protect against unexpected nil returns from ReconstructTreeFromFS
	if newTree == nil {
		return fmt.Errorf("internal error: ReconstructTreeFromFS returned nil tree")
	}

	// Save the old tree in case we need to revert
	// Note: oldTree may be nil if this is the first reconstruction (which is expected)
	oldTree := t.tree
	t.tree = newTree
	t.rebuildIndexesLocked()

	// Reconstructed nodes already carry metadata from frontmatter or safe defaults.

	if err := saveSchema(t.storageDir, CurrentSchemaVersion); err != nil {
		t.log.Error("Error saving schema after reconstruction", "error", err)
		t.tree = oldTree
		t.rebuildIndexesLocked()
		return err
	}

	return nil
}

type createNodeResult struct {
	id                 string
	entry              *PageNode
	parent             *PageNode
	parentWasConverted bool
}

type createNodeOptions struct {
	existingID string
}

// Create Node adds a new node to the tree
func (t *TreeService) CreateNode(userID string, parentID *string, title string, slug string, nodeKind *NodeKind) (*string, error) {
	var result *string
	err := t.withLockedTree(func() error {
		created, err := t.createNodeLocked(userID, parentID, title, slug, nodeKind, createNodeOptions{})
		if err != nil {
			return err
		}
		result = &created.id

		return nil
	})

	return result, err
}

func (t *TreeService) RestoreNode(userID, id string, parentID *string, title, slug string, nodeKind NodeKind, content string, metadata PageMetadata) (*Page, error) {
	var restored *Page
	err := t.withLockedTree(func() error {
		kind := nodeKind
		created, err := t.createNodeLocked(userID, parentID, title, slug, &kind, createNodeOptions{existingID: id})
		if err != nil {
			return err
		}

		if err := t.store.UpsertContent(created.entry, content); err != nil {
			return fmt.Errorf("could not restore content: %w", err)
		}

		created.entry.Metadata = metadata
		created.entry.Metadata.UpdatedAt = metadata.UpdatedAt.UTC()
		created.entry.Metadata.CreatedAt = metadata.CreatedAt.UTC()
		created.entry.Metadata.CreatorID = strings.TrimSpace(metadata.CreatorID)
		created.entry.Metadata.LastAuthorID = strings.TrimSpace(metadata.LastAuthorID)
		if err := t.store.SyncFrontmatterIfExists(created.entry); err != nil {
			return fmt.Errorf("could not sync restored frontmatter: %w", err)
		}

		restored = &Page{PageNode: created.entry, Content: content}
		return nil
	})
	return restored, err
}

// createNodeLocked creates a new node under the given parent.
// Lock must be held by the caller.
func (t *TreeService) createNodeLocked(userID string, parentID *string, title string, slug string, kind *NodeKind, opts createNodeOptions) (*createNodeResult, error) {
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
		parent = t.getNodeByIDLocked(*parentID)
		if parent == nil {
			return nil, ErrParentNotFound
		}
	}

	// Check if a child with the same slug already exists
	if t.findChildBySlugInParentLocked(parent, slug) != nil {
		return nil, ErrPageAlreadyExists
	}

	parentWasConverted := false

	// Check if the current parent is a section
	// if not, we need to convert it to a section
	if parent.Kind != NodeKindSection && parent.ID != "root" {
		t.log.Info("converting parent to section", "parentID", parent.ID, "oldKind", parent.Kind, "newKind", NodeKindSection)
		if err := t.store.ConvertNode(parent, NodeKindSection); err != nil {
			return nil, fmt.Errorf("could not convert parent node: %w", err)
		}
		parent.Kind = NodeKindSection
		parentWasConverted = true
	}

	if parent.Kind != NodeKindSection {
		return nil, fmt.Errorf("cannot add child to non-section parent, got %q", parent.Kind)
	}

	id := strings.TrimSpace(opts.existingID)
	if id == "" {
		var err error
		id, err = shared.GenerateUniqueID()
		if err != nil {
			return nil, fmt.Errorf("could not generate unique ID: %w", err)
		}
	} else if _, err := t.findPageByIDLocked(t.tree.Children, id); err == nil {
		return nil, fmt.Errorf("page id already exists: %s", id)
	} else if !errors.Is(err, ErrPageNotFound) {
		return nil, err
	}

	now := time.Now().UTC()

	entry := &PageNode{
		ID:       id,
		Title:    title,
		Parent:   parent,
		Slug:     slug,
		Kind:     k,
		Position: len(parent.Children), // Set the position to the end of the list
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
	parent.Children = append(parent.Children, entry)
	t.indexNodeLocked(entry)
	if err := t.store.SaveChildOrder(parent); err != nil {
		rollbackErr := t.rollbackCreatedNodeLocked(parent, entry, parentWasConverted)
		if rollbackErr != nil {
			return nil, errors.Join(fmt.Errorf("could not persist child order: %w", err), fmt.Errorf("rollback created node: %w", rollbackErr))
		}
		return nil, fmt.Errorf("could not persist child order: %w", err)
	}
	return &createNodeResult{
		id:                 entry.ID,
		entry:              entry,
		parent:             parent,
		parentWasConverted: parentWasConverted,
	}, nil
}

func (t *TreeService) rollbackCreatedNodeLocked(parent *PageNode, entry *PageNode, parentWasConverted bool) error {
	if parent == nil || entry == nil {
		return nil
	}

	for i, child := range parent.Children {
		if child == entry {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			break
		}
	}
	t.removeNodeIndexLocked(entry)

	switch entry.Kind {
	case NodeKindSection:
		if err := t.store.DeleteSection(entry); err != nil {
			return err
		}
	case NodeKindPage:
		if err := t.store.DeletePage(entry); err != nil {
			return err
		}
	}

	if parentWasConverted && parent != nil && len(parent.Children) == 0 {
		orderPath, err := t.store.dirPathForNode(parent)
		if err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(orderPath, orderFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove parent order file before fold-back: %w", err)
		}
		if err := t.store.ConvertNode(parent, NodeKindPage); err != nil {
			return err
		}
		parent.Kind = NodeKindPage
	}

	return nil
}

// FindPageByID finds a page in the tree by its ID.
func (t *TreeService) FindPageByID(id string) (*PageNode, error) {
	var result *PageNode
	err := t.withRLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}

		result = t.getNodeByIDLocked(id)
		if result == nil {
			return ErrPageNotFound
		}

		return nil
	})

	return result, err
}

func (t *TreeService) getNodeByIDLocked(id string) *PageNode {
	if id == "" {
		return nil
	}
	if id == "root" {
		return t.tree
	}

	return t.nodesByID[id]
}

func (t *TreeService) rebuildIndexesLocked() {
	t.nodesByID = make(map[string]*PageNode)
	t.childSlugs = make(map[string]map[string]*PageNode)

	if t.tree == nil {
		return
	}

	var walk func(node *PageNode)
	walk = func(node *PageNode) {
		if node == nil {
			return
		}

		if node.ID != "" && node.ID != "root" {
			t.nodesByID[node.ID] = node
		}
		t.rebuildChildSlugIndexForParentLocked(node)

		for _, child := range node.Children {
			walk(child)
		}
	}

	walk(t.tree)
}

func (t *TreeService) rebuildChildSlugIndexForParentLocked(parent *PageNode) {
	if parent == nil {
		return
	}

	index := make(map[string]*PageNode, len(parent.Children))
	for _, child := range parent.Children {
		if child == nil {
			continue
		}
		index[strings.ToLower(child.Slug)] = child
	}
	t.childSlugs[parent.ID] = index
}

func (t *TreeService) indexNodeLocked(node *PageNode) {
	if node == nil {
		return
	}

	if node.ID != "" && node.ID != "root" {
		t.nodesByID[node.ID] = node
	}
	t.rebuildChildSlugIndexForParentLocked(node)
	if node.Parent != nil {
		t.rebuildChildSlugIndexForParentLocked(node.Parent)
	}
}

func (t *TreeService) removeNodeIndexLocked(node *PageNode) {
	if node == nil {
		return
	}

	var walk func(current *PageNode)
	walk = func(current *PageNode) {
		if current == nil {
			return
		}

		delete(t.nodesByID, current.ID)
		delete(t.childSlugs, current.ID)

		for _, child := range current.Children {
			walk(child)
		}
	}

	walk(node)
	if node.Parent != nil {
		t.rebuildChildSlugIndexForParentLocked(node.Parent)
	}
}

func (t *TreeService) findChildBySlugInParentLocked(parent *PageNode, slug string) *PageNode {
	if parent == nil {
		return nil
	}

	if index, ok := t.childSlugs[parent.ID]; ok {
		return index[strings.ToLower(slug)]
	}

	for _, child := range parent.Children {
		if child != nil && strings.EqualFold(child.Slug, slug) {
			return child
		}
	}

	return nil
}

func (t *TreeService) findChildBySlugExactInParentLocked(parent *PageNode, slug string) *PageNode {
	if parent == nil {
		return nil
	}

	for _, child := range parent.Children {
		if child != nil && child.Slug == slug {
			return child
		}
	}

	return nil
}

// DeleteNode deletes a node from the tree
func (t *TreeService) DeleteNode(userID string, id string, recursive bool) error {
	err := t.withLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}

		// Find the node to delete
		node := t.getNodeByIDLocked(id)
		if node == nil {
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
				// This should not happen due to earlier check, but just in case
				// Convert to section and delete recursively
				t.log.Info("converting page to section for recursive delete", "pageID", node.ID)
				if err := t.store.ConvertNode(node, NodeKindSection); err != nil {
					return fmt.Errorf("could not convert page to section: %w", err)
				}
				node.Kind = NodeKindSection
				if err := t.store.DeleteSection(node); err != nil {
					return fmt.Errorf("could not delete section entry: %w", err)
				}
			} else {
				if err := t.store.DeletePage(node); err != nil {
					return fmt.Errorf("could not delete page entry: %w", err)
				}
			}
		default:
			return fmt.Errorf("unknown node kind: %v", node.Kind)
		}

		// Remove the page from the parent
		for i, e := range parent.Children {
			if e.ID == id {
				parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
				break
			}
		}
		t.removeNodeIndexLocked(node)

		t.reindexPositions(parent)
		if err := t.store.SaveChildOrder(parent); err != nil {
			return fmt.Errorf("could not persist child order: %w", err)
		}
		return nil
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
		node := t.getNodeByIDLocked(id)
		if node == nil {
			return ErrPageNotFound
		}

		// Slug must be unique under same parent (when changed)
		if slug != node.Slug && node.Parent != nil {
			existing := t.findChildBySlugInParentLocked(node.Parent, slug)
			if existing != nil && existing.ID != node.ID {
				return ErrPageAlreadyExists
			}
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
			if node.Parent != nil {
				t.rebuildChildSlugIndexForParentLocked(node.Parent)
			}
		}

		// Update title in tree
		node.Title = title

		// Update metadata
		node.Metadata.UpdatedAt = time.Now().UTC()
		node.Metadata.LastAuthorID = userID

		// Keep frontmatter in sync *if file exists* (important when title changed but content == nil)
		if err := t.store.SyncFrontmatterIfExists(node); err != nil {
			return fmt.Errorf("could not sync frontmatter: %w", err)
		}

		// Save tree
		return nil
	})

}

func (t *TreeService) ConvertNode(userID string, id string, kind NodeKind) error {
	return t.withLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}

		// Find node
		node := t.getNodeByIDLocked(id)
		if node == nil {
			return ErrPageNotFound
		}

		if node.Kind == kind {
			// No change
			return nil
		}

		// Section -> Page only allowed if no children
		if node.Kind == NodeKindSection && kind == NodeKindPage && node.HasChildren() {
			return ErrPageHasChildren
		}

		t.log.Info("changing node kind", "nodeID", node.ID, "oldKind", node.Kind, "newKind", kind)

		if err := t.store.ConvertNode(node, kind); err != nil {
			return fmt.Errorf("could not convert node: %w", err)
		}
		node.Kind = kind

		// Update metadata
		node.Metadata.UpdatedAt = time.Now().UTC()
		node.Metadata.LastAuthorID = userID

		// Keep frontmatter in sync *if file exists* (important when kind changed but content == nil)
		if err := t.store.SyncFrontmatterIfExists(node); err != nil {
			return fmt.Errorf("could not sync frontmatter: %w", err)
		}

		// Save tree
		return nil
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
	page := t.getNodeByIDLocked(id)
	if page == nil {
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

// FindPageByRoutePath finds a page in the tree by its path.
func (t *TreeService) FindPageByRoutePath(routePath string) (*Page, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

	// Split the routePath into parts
	routePart := strings.Split(routePath, "/")
	if len(routePart) == 0 {
		return nil, ErrPageNotFound
	}

	parent := t.tree
	var node *PageNode
	for _, part := range routePart {
		if part == "" {
			return nil, ErrPageNotFound
		}

		node = t.findChildBySlugExactInParentLocked(parent, part)
		if node == nil {
			return nil, ErrPageNotFound
		}

		parent = node
	}

	content, err := t.store.ReadPageContent(node)
	if err != nil {
		return nil, fmt.Errorf("could not get page content: %w", err)
	}

	return &Page{
		PageNode: node,
		Content:  content,
	}, nil
}

// LookupPagePath looks up a path in the tree and returns a PathLookup struct
// that contains information about the path and its segments and whether they exist.
func (t *TreeService) LookupPagePath(p string) (*PathLookup, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.lookupPagePathLocked(p)
}

// lookupPagePathLocked looks up a path in the tree and returns a PathLookup struct
// that contains information about the path and its segments and whether they exist.
// Lock must be held by the caller.
func (t *TreeService) lookupPagePathLocked(p string) (*PathLookup, error) {
	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

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

	parent := t.tree

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

		// Check if the segment exists under the current parent.
		e := t.findChildBySlugInParentLocked(parent, part)
		if e != nil {
			// Segment exists
			lookup.Segments[i].Exists = true
			lookup.Segments[i].ID = &e.ID
			lookup.Segments[i].Kind = &e.Kind
			lookup.Segments[i].Title = &e.Title

			// Move to the next parent
			parent = e
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

			parent = nil
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

	lookup, err := t.lookupPagePathLocked(p)
	if err != nil {
		return nil, fmt.Errorf("could not lookup page path: %w", err)
	}

	// Path exists -> return existing
	if lookup.Exists {
		last := lookup.Segments[len(lookup.Segments)-1]
		page := t.getNodeByIDLocked(*last.ID)
		if page == nil {
			return nil, fmt.Errorf("could not find existing page by ID: %w", ErrPageNotFound)
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

		createdNode, err := t.createNodeLocked(userID, currentID, segTitle, segment.Slug, &kindToUse, createNodeOptions{})
		if err != nil {
			return nil, fmt.Errorf("could not create segment %q: %w", segment.Slug, err)
		}
		currentID = &createdNode.id

		created = append(created, &PageNode{
			ID:    createdNode.id,
			Slug:  segment.Slug,
			Title: segTitle,
			Kind:  kindToUse,
		})
	}

	// Resolve final page
	if currentID == nil {
		return nil, fmt.Errorf("could not ensure page path")
	}
	page := t.getNodeByIDLocked(*currentID)
	if page == nil {
		return nil, fmt.Errorf("could not find created page by ID: %w", ErrPageNotFound)
	}

	// Save once
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
	node := t.getNodeByIDLocked(id)
	if node == nil {
		return ErrPageNotFound
	}

	oldParent := node.Parent
	if oldParent == nil {
		return fmt.Errorf("old parent not found: %w", ErrParentNotFound)
	}

	// Resolve destination parent (default root)
	newParent := t.tree
	if parentID != "" && parentID != "root" {
		newParent = t.getNodeByIDLocked(parentID)
		if newParent == nil {
			return fmt.Errorf("new parent not found: %w", ErrParentNotFound)
		}
	}

	// Same slug collision under new parent
	if existing := t.findChildBySlugInParentLocked(newParent, node.Slug); existing != nil && existing.ID != node.ID {
		return fmt.Errorf("child with the same slug already exists: %w", ErrPageAlreadyExists)
	}

	// Can't move into itself
	if node.ID == newParent.ID {
		return fmt.Errorf("page cannot be moved to itself: %w", ErrPageCannotBeMovedToItself)
	}

	// Circular reference guard: node cannot be moved under its own descendants
	if node.IsChildOf(newParent.ID, true) {
		return fmt.Errorf("circular reference detected: %w", ErrMovePageCircularReference)
	}

	newParentWasConverted := false
	if newParent.ID != "root" && newParent.Kind == NodeKindPage {
		if err := t.store.ConvertNode(newParent, NodeKindSection); err != nil {
			return fmt.Errorf("could not auto-convert new parent page to section: %w", err)
		}
		newParent.Kind = NodeKindSection
		newParentWasConverted = true
	}

	if newParent.Kind != NodeKindSection {
		return fmt.Errorf("destination parent must be a section, got %q", newParent.Kind)
	}

	previousOldChildren := append([]*PageNode(nil), oldParent.Children...)
	previousOldPositions := snapshotChildPositions(oldParent.Children)
	previousNewChildren := append([]*PageNode(nil), newParent.Children...)
	previousNewPositions := snapshotChildPositions(newParent.Children)
	previousPosition := node.Position
	previousMetadata := node.Metadata

	if err := t.store.MoveNode(node, newParent); err != nil {
		return fmt.Errorf("could not move node on disk: %w", err)
	}

	for i, e := range oldParent.Children {
		if e.ID == id {
			oldParent.Children = append(oldParent.Children[:i], oldParent.Children[i+1:]...)
			break
		}
	}

	node.Position = len(newParent.Children)
	newParent.Children = append(newParent.Children, node)
	node.Parent = newParent
	t.rebuildChildSlugIndexForParentLocked(oldParent)
	t.rebuildChildSlugIndexForParentLocked(newParent)
	node.Metadata.UpdatedAt = time.Now().UTC()
	node.Metadata.LastAuthorID = userID

	t.reindexPositions(newParent)
	t.reindexPositions(oldParent)

	if err := t.store.SaveChildOrder(oldParent); err != nil {
		rollbackErr := t.rollbackMovedNodeLocked(node, oldParent, newParent, previousOldChildren, previousOldPositions, previousNewChildren, previousNewPositions, previousPosition, previousMetadata, newParentWasConverted)
		if rollbackErr != nil {
			return errors.Join(fmt.Errorf("could not persist source child order: %w", err), fmt.Errorf("rollback moved node: %w", rollbackErr))
		}
		return fmt.Errorf("could not persist source child order: %w", err)
	}
	if newParent != oldParent {
		if err := t.store.SaveChildOrder(newParent); err != nil {
			rollbackErr := t.rollbackMovedNodeLocked(node, oldParent, newParent, previousOldChildren, previousOldPositions, previousNewChildren, previousNewPositions, previousPosition, previousMetadata, newParentWasConverted)
			if rollbackErr != nil {
				return errors.Join(fmt.Errorf("could not persist destination child order: %w", err), fmt.Errorf("rollback moved node: %w", rollbackErr))
			}
			return fmt.Errorf("could not persist destination child order: %w", err)
		}
	}

	if err := t.store.SyncFrontmatterIfExists(node); err != nil {
		rollbackErr := t.rollbackMovedNodeLocked(node, oldParent, newParent, previousOldChildren, previousOldPositions, previousNewChildren, previousNewPositions, previousPosition, previousMetadata, newParentWasConverted)
		if rollbackErr != nil {
			return errors.Join(fmt.Errorf("could not sync moved node frontmatter: %w", err), fmt.Errorf("rollback moved node: %w", rollbackErr))
		}
		return fmt.Errorf("could not sync moved node frontmatter: %w", err)
	}

	return nil
}

func snapshotChildPositions(children []*PageNode) map[string]int {
	positions := make(map[string]int, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}
		positions[child.ID] = child.Position
	}
	return positions
}

func restoreChildSnapshot(parent *PageNode, children []*PageNode, positions map[string]int) {
	if parent == nil {
		return
	}

	parent.Children = append([]*PageNode(nil), children...)
	for _, child := range parent.Children {
		if child == nil {
			continue
		}
		child.Parent = parent
		if pos, ok := positions[child.ID]; ok {
			child.Position = pos
		}
	}
}

func (t *TreeService) rollbackMovedNodeLocked(node *PageNode, oldParent *PageNode, newParent *PageNode, previousOldChildren []*PageNode, previousOldPositions map[string]int, previousNewChildren []*PageNode, previousNewPositions map[string]int, previousPosition int, previousMetadata PageMetadata, newParentWasConverted bool) error {
	var rollbackErr error

	if moveErr := t.store.MoveNode(node, oldParent); moveErr != nil {
		rollbackErr = errors.Join(rollbackErr, fmt.Errorf("move node back on disk: %w", moveErr))
	}

	restoreChildSnapshot(oldParent, previousOldChildren, previousOldPositions)
	if newParent != oldParent {
		restoreChildSnapshot(newParent, previousNewChildren, previousNewPositions)
	}
	t.rebuildChildSlugIndexForParentLocked(oldParent)
	if newParent != oldParent {
		t.rebuildChildSlugIndexForParentLocked(newParent)
	}
	node.Parent = oldParent
	node.Position = previousPosition
	node.Metadata = previousMetadata

	if newParentWasConverted && newParent != nil && newParent.ID != "root" && len(newParent.Children) == 0 {
		newParentDir, dirErr := t.store.dirPathForNode(newParent)
		if dirErr != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("resolve converted parent dir: %w", dirErr))
		} else {
			if removeErr := os.RemoveAll(filepath.Join(newParentDir, orderFilename)); removeErr != nil {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove child order before parent rollback: %w", removeErr))
			}
		}
		if convertErr := t.store.ConvertNode(newParent, NodeKindPage); convertErr != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("convert destination parent back to page: %w", convertErr))
		} else {
			newParent.Kind = NodeKindPage
		}
	}

	if err := t.store.SaveChildOrder(oldParent); err != nil {
		rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore source child order: %w", err))
	}
	if newParent != oldParent && newParent.Kind == NodeKindSection {
		if err := t.store.SaveChildOrder(newParent); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore destination child order: %w", err))
		}
	}

	return rollbackErr
}

func (t *TreeService) SortPages(parentID string, orderedIDs []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	parent := t.tree

	if parentID != "" && parentID != "root" {
		parent = t.getNodeByIDLocked(parentID)
		if parent == nil {
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

	previousChildren := append([]*PageNode(nil), parent.Children...)
	previousPositions := make(map[string]int, len(parent.Children))
	for _, child := range parent.Children {
		previousPositions[child.ID] = child.Position
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
	t.reindexPositions(parent)

	if err := t.store.SaveChildOrder(parent); err != nil {
		parent.Children = previousChildren
		for _, child := range parent.Children {
			if pos, ok := previousPositions[child.ID]; ok {
				child.Position = pos
			}
		}
		return fmt.Errorf("could not persist child order: %w", err)
	}

	return nil
}

func (t *TreeService) reindexPositions(parent *PageNode) {
	sort.SliceStable(parent.Children, func(i, j int) bool {
		return parent.Children[i].Position < parent.Children[j].Position
	})
	for i, child := range parent.Children {
		child.Position = i
	}
}
