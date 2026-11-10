package tree

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// GetTree returns the tree
func (t *TreeService) GetTree() *PageNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.tree
}

// IsLoaded reports whether the tree has been loaded into memory.
func (t *TreeService) IsLoaded() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.tree != nil
}

// HasPages reports whether the tree contains at least one non-root node.
func (t *TreeService) HasPages() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.tree != nil && len(t.tree.Children) > 0
}

// FirstRootChildPath returns the route path of the first top-level node, page
// or section, in the order the navigation shows (the order SortPages sets).
// It returns ErrTreeNotLoaded before the tree is loaded and ErrPageNotFound
// when the tree has no nodes.
func (t *TreeService) FirstRootChildPath() (string, error) {
	var routePath string
	err := t.withRLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}
		if len(t.tree.Children) == 0 {
			return ErrPageNotFound
		}
		routePath = t.tree.Children[0].Slug // a root child's route path is its slug
		return nil
	})
	return routePath, err
}

// RouteEntry is one node of the tree as a list of routes needs it.
type RouteEntry struct {
	ID string
	// ParentID is the ID of the node's parent: "root" for the top level, empty
	// only for the tree root itself, which no route read returns.
	ParentID string
	Title    string
	// RoutePath is the node's route path without a leading slash (e.g. "docs/setup").
	RoutePath string
	Kind      NodeKind
	UpdatedAt time.Time
}

// PageRoutes returns every non-root node, pages and sections, depth-first in
// the order the navigation shows. With limit > 0 it stops after limit entries
// and reports truncated when more nodes exist. The walk holds the read lock
// and reads no files, so it costs one pass over the in-memory tree. It returns
// ErrTreeNotLoaded before the tree is loaded.
func (t *TreeService) PageRoutes(limit int) (entries []RouteEntry, truncated bool, err error) {
	err = t.withRLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}
		capacity := len(t.nodesByID)
		if limit > 0 && limit < capacity {
			capacity = limit
		}
		entries = make([]RouteEntry, 0, capacity)
		entries, truncated = appendDescendantRoutesLocked(entries, t.tree, "", limit)
		return nil
	})
	return entries, truncated, err
}

// SubtreeRoutes returns the node with id followed by all its descendants,
// depth-first in the order the navigation shows, so every entry comes after
// its parent. Like PageRoutes it is one locked read without file reads: the
// subtree can't be torn by a concurrent move. It returns ErrTreeNotLoaded
// before the tree is loaded and ErrPageNotFound for an unknown id and for the
// tree root (use PageRoutes for the whole tree).
func (t *TreeService) SubtreeRoutes(id string) ([]RouteEntry, error) {
	var entries []RouteEntry
	err := t.withRLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}
		node := t.getNodeByIDLocked(id)
		if node == nil || node == t.tree {
			return ErrPageNotFound
		}
		entry := routeEntryLocked(node)
		entries, _ = appendDescendantRoutesLocked([]RouteEntry{entry}, node, entry.RoutePath, 0)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// appendDescendantRoutesLocked appends the descendants of node, whose route
// path is nodePath, depth-first in navigation order. With limit > 0 it stops
// at limit entries and reports truncated when more exist. The caller holds
// the read lock.
func appendDescendantRoutesLocked(entries []RouteEntry, node *PageNode, nodePath string, limit int) (_ []RouteEntry, truncated bool) {
	var walk func(node *PageNode, parentPath string) bool
	walk = func(node *PageNode, parentPath string) bool {
		for _, child := range node.Children {
			if limit > 0 && len(entries) == limit {
				truncated = true
				return false
			}
			entry := newRouteEntry(child, parentPath)
			entries = append(entries, entry)
			if !walk(child, entry.RoutePath) {
				return false
			}
		}
		return true
	}
	walk(node, nodePath)
	return entries, truncated
}

// ChildRoutes returns the direct children of the node with id ("root" for
// the top level), pages and sections, in the order the navigation shows. Like
// PageRoutes it is one locked read without file reads. It returns
// ErrTreeNotLoaded before the tree is loaded and ErrPageNotFound for an
// unknown id.
func (t *TreeService) ChildRoutes(id string) ([]RouteEntry, error) {
	var entries []RouteEntry
	err := t.withRLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}
		node := t.getNodeByIDLocked(id)
		if node == nil {
			return ErrPageNotFound
		}
		parentPath := strings.TrimPrefix(node.CalculatePath(), "/") // root-relative, like PageRoutes
		entries = make([]RouteEntry, 0, len(node.Children))
		for _, child := range node.Children {
			entries = append(entries, newRouteEntry(child, parentPath))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// RoutesByTitle returns the route of every node whose title matches title
// (trimmed, case-insensitive, like FindPagesByTitle). Unlike the nodes
// FindPagesByTitle returns, the route paths are computed under the read
// lock, so a concurrent move or rename can't race with them. No files are
// read. It returns ErrTreeNotLoaded before the tree is loaded.
func (t *TreeService) RoutesByTitle(title string) ([]RouteEntry, error) {
	var entries []RouteEntry
	err := t.withRLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}
		for _, node := range t.nodesWithTitleLocked(title) {
			entries = append(entries, routeEntryLocked(node))
		}
		return nil
	})
	return entries, err
}

// RouteByPath returns the route of the node at routePath (without a leading
// slash, exact slugs) from one locked read without reading its file. It
// returns ErrTreeNotLoaded before the tree is loaded and ErrPageNotFound when
// no node is there.
func (t *TreeService) RouteByPath(routePath string) (RouteEntry, error) {
	var entry RouteEntry
	err := t.withRLockedTree(func() error {
		if t.tree == nil {
			return ErrTreeNotLoaded
		}
		node := t.nodeByRoutePathLocked(routePath)
		if node == nil {
			return ErrPageNotFound
		}
		entry = routeEntryLocked(node)
		return nil
	})
	return entry, err
}

// routeEntryLocked returns the RouteEntry of node with its full route path.
// The caller holds the read lock.
func routeEntryLocked(node *PageNode) RouteEntry {
	parentPath := ""
	if node.Parent != nil {
		parentPath = strings.TrimPrefix(node.Parent.CalculatePath(), "/")
	}
	return newRouteEntry(node, parentPath)
}

// newRouteEntry returns the RouteEntry of node below parentPath ("" for the
// top level). The caller holds the read lock.
func newRouteEntry(node *PageNode, parentPath string) RouteEntry {
	routePath := node.Slug
	if parentPath != "" {
		routePath = parentPath + "/" + node.Slug
	}
	parentID := ""
	if node.Parent != nil {
		parentID = node.Parent.ID
	}
	return RouteEntry{
		ID:        node.ID,
		ParentID:  parentID,
		Title:     node.Title,
		RoutePath: routePath,
		Kind:      node.Kind,
		UpdatedAt: node.Metadata.UpdatedAt,
	}
}

// NodeCounts returns how many page and section nodes the tree currently holds,
// excluding the synthetic root. It reads the in-memory id index under the read
// lock and allocates nothing, so it is cheap enough to call on every metrics
// scrape. A node whose kind is unset is counted as a page.
func (t *TreeService) NodeCounts() (pages, sections int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, node := range t.nodesByID {
		if node.Kind == NodeKindSection {
			sections++
			continue
		}
		pages++
	}
	return pages, sections
}

// WalkNodes calls fn with the ID of every non-root node (pages and sections)
// in depth-first order. The read lock is held only while collecting IDs; fn
// is called without any lock held so it may safely call other TreeService
// methods. Returns nil immediately when the tree is not yet loaded.
func (t *TreeService) WalkNodes(fn func(id string) error) error {
	ids := t.collectIDsDFS()
	for _, id := range ids {
		if err := fn(id); err != nil {
			return err
		}
	}
	return nil
}

// collectIDsDFS returns the IDs of all non-root nodes in depth-first order
// under the read lock.
func (t *TreeService) collectIDsDFS() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		return nil
	}

	var ids []string
	var collect func(*PageNode)
	collect = func(node *PageNode) {
		if node.ID != "root" {
			ids = append(ids, node.ID)
		}
		for _, child := range node.Children {
			collect(child)
		}
	}
	collect(t.tree)
	return ids
}

// GetPages returns pages for the given IDs under a single read lock,
// reading files in parallel. Each entry is nil when the corresponding error is non-nil.
func (t *TreeService) GetPages(ids []string) ([]*Page, []error) {
	pages := make([]*Page, len(ids))
	errs := make([]error, len(ids))
	if len(ids) == 0 {
		return pages, errs
	}

	type task struct {
		index int
		node  *PageNode
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		for i := range errs {
			errs[i] = ErrTreeNotLoaded
		}
		return pages, errs
	}

	tasks := make([]task, 0, len(ids))
	for i, id := range ids {
		node := t.getNodeByIDLocked(id)
		if node == nil {
			errs[i] = ErrPageNotFound
			continue
		}
		tasks = append(tasks, task{index: i, node: node})
	}

	if len(tasks) == 0 {
		return pages, errs
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, tk := range tasks {
		go func(tk task) {
			defer wg.Done()
			content, raw, err := t.store.ReadPageAndRaw(tk.node)
			mu.Lock()
			if err != nil {
				errs[tk.index] = fmt.Errorf(errGetPageContentFailed, err)
			} else {
				pages[tk.index] = &Page{PageNode: tk.node, Content: content, RawContent: raw}
			}
			mu.Unlock()
		}(tk)
	}
	wg.Wait()

	return pages, errs
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

	content, raw, err := t.store.ReadPageAndRaw(page)
	if err != nil {
		return nil, fmt.Errorf(errGetPageContentFailed, err)
	}

	return &Page{
		PageNode:   page,
		Content:    content,
		RawContent: raw,
	}, nil
}

// ReadPageRaw returns the raw markdown of a page including frontmatter.
func (t *TreeService) ReadPageRaw(id string) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		return "", ErrTreeNotLoaded
	}

	page := t.getNodeByIDLocked(id)
	if page == nil {
		return "", ErrPageNotFound
	}

	raw, err := t.store.ReadPageRaw(page)
	if err != nil {
		return "", fmt.Errorf("could not get page raw content: %w", err)
	}

	return raw, nil
}

// ResolvePermalinkTarget resolves a stable page ID to the current route path.
func (t *TreeService) ResolvePermalinkTarget(id string) (*PermalinkTarget, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

	node := t.getNodeByIDLocked(id)
	if node == nil {
		return nil, ErrPageNotFound
	}

	return &PermalinkTarget{
		ID:   node.ID,
		Slug: node.Slug,
		Path: strings.TrimPrefix(node.CalculatePath(), "/"),
	}, nil
}

// nodeByRoutePathLocked returns the node at routePath (exact slugs, no empty
// segments), or nil. The caller holds the read lock.
func (t *TreeService) nodeByRoutePathLocked(routePath string) *PageNode {
	parent := t.tree
	var node *PageNode
	for _, part := range strings.Split(routePath, "/") {
		if part == "" {
			return nil
		}
		node = t.findChildBySlugExactInParentLocked(parent, part)
		if node == nil {
			return nil
		}
		parent = node
	}
	return node
}

// FindPageByRoutePath finds a page in the tree by its path.
func (t *TreeService) FindPageByRoutePath(routePath string) (*Page, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		return nil, ErrTreeNotLoaded
	}

	node := t.nodeByRoutePathLocked(routePath)
	if node == nil {
		return nil, ErrPageNotFound
	}

	content, err := t.store.ReadPageContent(node)
	if err != nil {
		return nil, fmt.Errorf(errGetPageContentFailed, err)
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

	slugService := NewSlugService()
	path := strings.TrimSpace(p)
	path = strings.Trim(path, "/")
	if path == "" {
		return &PathLookup{
			Path:      path,
			Segments:  []PathSegment{},
			Exists:    false,
			CanCreate: false,
		}, nil
	}

	// remove double slashes
	path = strings.ReplaceAll(path, "//", "/")

	// Split the path into parts
	pathParts := strings.Split(path, "/")
	if len(pathParts) == 0 {
		return &PathLookup{
			Path:      path,
			Segments:  []PathSegment{},
			Exists:    false,
			CanCreate: false,
		}, nil
	}

	lookup := &PathLookup{
		Path:      path,
		Segments:  make([]PathSegment, len(pathParts)),
		Exists:    true,
		CanCreate: true,
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
			if lookup.CanCreate && slugService.IsValidSlug(part) != nil {
				lookup.CanCreate = false
			}

			// No need to check further segments
			// Set all remaining segments to non-existing
			for j := i + 1; j < len(pathParts); j++ {
				if lookup.CanCreate && slugService.IsValidSlug(pathParts[j]) != nil {
					lookup.CanCreate = false
				}
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
