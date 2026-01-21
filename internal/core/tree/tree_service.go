package tree

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared"
)

// TreeService is our main component for handling tree operations
// We use this service to create pages, delete pages, update pages, etc.
type TreeService struct {
	storageDir   string
	treeFilename string
	tree         *PageNode
	store        *NodeStore
	log          *slog.Logger

	mu sync.RWMutex
}

// NewTreeService creates a new TreeService
func NewTreeService(storageDir string) *TreeService {
	return &TreeService{
		storageDir:   storageDir,
		treeFilename: "tree.json",
		tree:         nil,
		store:        NewNodeStore(storageDir),
		log:          slog.Default().With("component", "TreeService"),
	}
}

// LoadTree loads the tree from the storage directory
// If the tree does not exist, it creates a new tree
func (t *TreeService) LoadTree() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Load the tree from the storage directory
	var err error
	t.tree, err = t.store.LoadTree(t.treeFilename)
	if err != nil {
		return err
	}

	// Load the schema version
	t.log.Info("Checking schema version...")
	schema, err := loadSchema(t.storageDir)
	if err != nil {
		t.log.Error("Error loading schema", "error", err)
		return err
	}

	if schema.Version < CurrentSchemaVersion {
		t.log.Info("Migrating schema", "fromVersion", schema.Version, "toVersion", CurrentSchemaVersion)
		if err := t.migrate(schema.Version); err != nil {
			t.log.Error("Error migrating schema", "error", err)
			return err
		}
	}

	return err
}

func (t *TreeService) migrate(fromVersion int) error {

	for v := fromVersion; v < CurrentSchemaVersion; v++ {
		switch v {
		case 0:
			if err := t.migrateToV1(); err != nil {
				t.log.Error("Error migrating to v1", "error", err)
				return err
			}
		case 1:
			if err := t.migrateToV2(); err != nil {
				t.log.Error("Error migrating to v2", "error", err)
				return err
			}
		}

		// Save the tree after each migration step
		if err := t.saveTreeLocked(); err != nil {
			t.log.Error("Error saving tree after migration", "version", v+1, "error", err)
			return err
		}

		// Update the schema version file
		if err := saveSchema(t.storageDir, v+1); err != nil {
			t.log.Error("Error saving schema", "version", v+1, "error", err)
			return err
		}
	}
	return nil
}

func (t *TreeService) migrateToV1() error {
	// Backfill metadata for all pages
	var backfillMetadata func(node *PageNode) error
	backfillMetadata = func(node *PageNode) error {
		// If CreatedAt is already set, assume metadata was backfilled and skip
		if !node.Metadata.CreatedAt.IsZero() {
			return nil
		}

		// Read creation and modification times from the filesystem
		// and set them in the metadata

		r, err := t.store.resolveNode(node)
		if err != nil {
			// Log and continue (same behavior as before)
			t.log.Error("Could not resolve node for metadata backfill", "nodeID", node.ID, "error", err)
			return nil
		}

		// Prefer the real on-disk object:
		// - Page => <base>.md
		// - Folder with content => <base>/index.md
		// - Folder without content => use folder mtime
		statPath := r.FilePath
		if r.Kind == NodeKindSection && !r.HasContent {
			statPath = r.DirPath
		}

		// The default value is set to now
		createdAt := time.Now().UTC()
		updatedAt := time.Now().UTC()

		if statPath != "" {
			if info, err := os.Stat(statPath); err == nil {
				createdAt = info.ModTime().UTC()
				updatedAt = info.ModTime().UTC()
			} else if !os.IsNotExist(err) {
				t.log.Error("Could not stat node for metadata", "nodeID", node.ID, "path", statPath, "error", err)
			}
		}

		node.Metadata = PageMetadata{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}

		// Recurse into children
		for _, child := range node.Children {
			if err := backfillMetadata(child); err != nil {
				return err
			}
		}

		return nil
	}

	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	return backfillMetadata(t.tree)
}

// migrateToV2 migrates the tree to the v2 schema
// Adds frontmatter to all existing pages if missing
// Adds kind to all nodes
func (t *TreeService) migrateToV2() error {
	if t.tree == nil {
		return ErrTreeNotLoaded
	}
	t.backfillKindFromFSLocked()

	// Traverse all pages and add frontmatter if missing
	var addFrontmatter func(node *PageNode) error
	addFrontmatter = func(node *PageNode) error {
		// Read the content of the page
		content, err := t.store.ReadPageRaw(node)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrFileNotFound) {
				t.log.Warn("Page file does not exist, skipping frontmatter addition", "nodeID", node.ID)
				// Recurse into children
				for _, child := range node.Children {
					if err := addFrontmatter(child); err != nil {
						t.log.Error("Error adding frontmatter to child node", "nodeID", child.ID, "error", err)
						return err
					}
				}
				return nil
			}
			t.log.Error("Could not read page content for node", "nodeID", node.ID, "error", err)
			return fmt.Errorf("could not read page content for node %s: %w", node.ID, err)
		}

		// Parse the frontmatter
		fm, body, has, err := ParseFrontmatter(content)
		if err != nil {
			t.log.Error("Could not parse frontmatter for node", "nodeID", node.ID, "error", err)
			return fmt.Errorf("could not parse frontmatter for node %s: %w", node.ID, err)
		}

		// Decide if we need to change anything
		changed := false

		// If there is no frontmatter, start with a new one
		if !has {
			fm = Frontmatter{}
			changed = true
		}

		// Ensure required fields exist
		if strings.TrimSpace(fm.LeafWikiID) == "" {
			fm.LeafWikiID = node.ID
			changed = true
		}
		// Optional but nice: keep title in sync *at least once*
		// (you might choose to NOT overwrite existing title)
		if strings.TrimSpace(fm.LeafWikiTitle) == "" {
			fm.LeafWikiTitle = node.Title
			changed = true
		}

		// Only write if changed
		if changed {
			newContent, err := BuildMarkdownWithFrontmatter(fm, body)
			if err != nil {
				t.log.Error("could not build markdown with frontmatter", "nodeID", node.ID, "error", err)
				return fmt.Errorf("could not build markdown with frontmatter for node %s: %w", node.ID, err)
			}

			filePath, err := t.store.contentPathForNodeWrite(node)
			if err != nil {
				return fmt.Errorf("could not determine content path for node %s: %w", node.ID, err)
			}

			if err := shared.WriteFileAtomic(filePath, []byte(newContent), 0o644); err != nil {
				t.log.Error("could not write updated page content", "nodeID", node.ID, "filePath", filePath, "error", err)
				return fmt.Errorf("could not write updated page content for node %s: %w", node.ID, err)
			}

			t.log.Info("frontmatter backfilled", "nodeID", node.ID, "path", filePath)
		}

		// Recurse into children
		for _, child := range node.Children {
			if err := addFrontmatter(child); err != nil {
				t.log.Error("Error adding frontmatter to child node", "nodeID", child.ID, "error", err)
				return err
			}
		}

		return nil
	}

	// start the recursion from the children of the root
	for _, child := range t.tree.Children {
		if err := addFrontmatter(child); err != nil {
			t.log.Error("Error adding frontmatter to child node", "nodeID", child.ID, "error", err)
			return err
		}
	}

	return nil
}

func (t *TreeService) backfillKindFromFSLocked() {
	if t.tree == nil {
		return
	}
	t.tree.Kind = NodeKindSection

	var walk func(n *PageNode)
	walk = func(n *PageNode) {
		if n == nil {
			return
		}

		// Root skip
		if n.ID != "root" {
			// Nur backfillen, wenn Kind fehlt/unknown
			if n.Kind != NodeKindPage && n.Kind != NodeKindSection {
				r, err := t.store.resolveNode(n)
				if err == nil {
					n.Kind = r.Kind
				} else {
					// Fallback-Heuristik, wenn auf Disk nichts existiert
					if n.HasChildren() {
						n.Kind = NodeKindSection
					} else {
						n.Kind = NodeKindPage
					}
					t.log.Warn("could not resolve node on disk; kind backfilled by heuristic",
						"nodeID", n.ID, "slug", n.Slug, "err", err, "kind", n.Kind)
				}
			}
		}

		for _, ch := range n.Children {
			walk(ch)
		}
	}

	for _, ch := range t.tree.Children {
		walk(ch)
	}
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

// SaveTree saves the tree to the storage directory
func (t *TreeService) SaveTree() error {
	return t.withLockedTree(t.saveTreeLocked)
}

func (t *TreeService) TreeHash() string {
	var hash string
	_ = t.withRLockedTree(func() error {
		hash = t.tree.Hash()
		return nil
	})
	return hash
}

func (t *TreeService) saveTreeLocked() error {
	// Save the tree to the storage directory
	return t.store.SaveTree(t.treeFilename, t.tree)
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
	return &entry.ID, nil
}

// FindPageByID finds a page in the tree by its ID
// If the page is not found, it returns an error
func (t *TreeService) FindPageByID(entry []*PageNode, id string) (*PageNode, error) {
	var result *PageNode
	err := t.withRLockedTree(func() error {
		var err error
		result, err = t.findPageByIDLocked(entry, id)
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

		t.reindexPositions(parent)
		return t.saveTreeLocked()
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

		// Kind change?
		// This operation is currently disabled to avoid complexity with content migration.
		// We need to check if we need it later.
		// if kind != nil && *kind != node.Kind {
		// 	// Section -> Page only allowed if no children
		// 	if node.Kind == NodeKindSection && *kind == NodeKindPage && node.HasChildren() {
		// 		return ErrPageHasChildren
		// 	}

		// 	t.log.Info("changing node kind", "nodeID", node.ID, "oldKind", node.Kind, "newKind", *kind)
		// 	if err := t.store.ConvertNode(node, *kind); err != nil {
		// 		return fmt.Errorf("could not convert node: %w", err)
		// 	}
		// 	node.Kind = *kind
		// }

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

		// Keep frontmatter in sync *if file exists* (important when title changed but content == nil)
		if err := t.store.SyncFrontmatterIfExists(node); err != nil {
			return fmt.Errorf("could not sync frontmatter: %w", err)
		}

		// Save tree
		return t.saveTreeLocked()
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
		return t.saveTreeLocked()
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

// FindPageByPath finds a page in the tree by its path
func (t *TreeService) FindPageByRoutePath(entry []*PageNode, routePath string) (*Page, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

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

	// Save once
	if err := t.saveTreeLocked(); err != nil {
		return nil, fmt.Errorf("could not save tree: %w", err)
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
	if node.IsChildOf(newParent.ID, true) {
		return fmt.Errorf("circular reference detected: %w", ErrMovePageCircularReference)
	}

	// If destination parent is a PAGE, auto-convert it to SECTION so it can host children
	if newParent.ID != "root" && newParent.Kind == NodeKindPage {
		if err := t.store.ConvertNode(newParent, NodeKindSection); err != nil {
			return fmt.Errorf("could not auto-convert new parent page to section: %w", err)
		}
		newParent.Kind = NodeKindSection
	}

	// Defensive: after possible conversion, destination must be a section
	if newParent.Kind != NodeKindSection {
		return fmt.Errorf("destination parent must be a section, got %q", newParent.Kind)
	}

	// Move on disk (strict by node.Kind inside NodeStore)
	if err := t.store.MoveNode(node, newParent); err != nil {
		return fmt.Errorf("could not move node on disk: %w", err)
	}

	// Unlink from old parent in tree
	oldParent := node.Parent
	if oldParent == nil {
		return fmt.Errorf("old parent not found: %w", ErrParentNotFound)
	}
	for i, e := range oldParent.Children {
		if e.ID == id {
			oldParent.Children = append(oldParent.Children[:i], oldParent.Children[i+1:]...)
			break
		}
	}

	// Link under new parent
	node.Position = len(newParent.Children)
	newParent.Children = append(newParent.Children, node)
	node.Parent = newParent

	// Update metadata
	node.Metadata.UpdatedAt = time.Now().UTC()
	node.Metadata.LastAuthorID = userID

	// Reindex positions
	t.reindexPositions(newParent)
	t.reindexPositions(oldParent)

	// Persist tree
	return t.saveTreeLocked()
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
	t.reindexPositions(parent)

	// Save the tree
	return t.saveTreeLocked()
}

// maybeCollapseSectionToPageLocked tries to collapse a section node into a page node
// It is not used currently, but after testing the user flow we might want to integrate it
// into UpdateNode or MoveNode operations
// Lock must be held by the caller
// func (t *TreeService) maybeCollapseSectionToPageLocked(node *PageNode) {
// 	if node == nil || node.ID == "root" {
// 		return
// 	}
// 	if node.Kind != NodeKindSection {
// 		return
// 	}
// 	if node.HasChildren() {
// 		return
// 	}

// 	// Only collapse if index.md exists
// 	indexPath, err := t.store.contentPathForNodeRead(node)
// 	if err != nil {
// 		return
// 	}
// 	if _, err := os.Stat(indexPath); err != nil {
// 		// no index.md => keep as section
// 		return
// 	}

// 	// Try collapse (will refuse if folder has other files)
// 	if err := t.store.ConvertNode(node, NodeKindPage); err != nil {
// 		// not allowed (e.g. folder not empty) -> keep section
// 		return
// 	}
// 	node.Kind = NodeKindPage
// }

func (t *TreeService) reindexPositions(parent *PageNode) {
	sort.SliceStable(parent.Children, func(i, j int) bool {
		return parent.Children[i].Position < parent.Children[j].Position
	})
	for i, child := range parent.Children {
		child.Position = i
	}
}

// func (t *TreeService) sortTreeByPosition(node *PageNode) {
// 	sort.SliceStable(node.Children, func(i, j int) bool {
// 		return node.Children[i].Position < node.Children[j].Position
// 	})
// 	for _, child := range node.Children {
// 		t.sortTreeByPosition(child)
// 	}
// }
