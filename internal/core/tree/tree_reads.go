package tree

import (
	"fmt"
	"strings"
	"sync"
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
