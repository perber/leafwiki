package tree

import (
	"strings"
)

// FindPagesByTitle returns all nodes whose title matches the given string
// (case-insensitive) in pre-order (depth-first) tree order, so results are
// stable across calls. Returns nil when the tree is not loaded or the title
// is empty.
func (t *TreeService) FindPagesByTitle(title string) []*PageNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		return nil
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}

	nodes := t.nodesByTitle[strings.ToLower(title)]
	if len(nodes) == 0 {
		return nil
	}
	return nodes
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
	t.nodesByTitle = make(map[string][]*PageNode)
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
			t.addTitleIndexForNodeLocked(node)
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

func (t *TreeService) addTitleIndexForNodeLocked(node *PageNode) {
	if node == nil || node.Title == "" {
		return
	}
	key := strings.ToLower(node.Title)
	t.nodesByTitle[key] = append(t.nodesByTitle[key], node)
}

func (t *TreeService) removeTitleIndexForNodeLocked(node *PageNode) {
	if node == nil || node.Title == "" {
		return
	}
	key := strings.ToLower(node.Title)
	nodes := t.nodesByTitle[key]
	filtered := nodes[:0]
	for _, n := range nodes {
		if n.ID != node.ID {
			filtered = append(filtered, n)
		}
	}
	if len(filtered) == 0 {
		delete(t.nodesByTitle, key)
	} else {
		t.nodesByTitle[key] = filtered
	}
}

func (t *TreeService) indexNodeLocked(node *PageNode) {
	if node == nil {
		return
	}

	if node.ID != "" && node.ID != "root" {
		t.nodesByID[node.ID] = node
		t.addTitleIndexForNodeLocked(node)
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
		t.removeTitleIndexForNodeLocked(current)

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
