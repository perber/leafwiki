// Package dto contains the HTTP response types and mapping functions shared
// across all domain route registrars. It must NOT import internal/wiki to
// avoid circular dependencies.
package dto

import (
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
)

// NodeMetadata contains authorship and timestamp information for a page node.
type NodeMetadata struct {
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	CreatorID    string `json:"creatorId"`
	LastAuthorID string `json:"lastAuthorId"`

	Creator    *auth.UserLabel `json:"creator,omitempty"`
	LastAuthor *auth.UserLabel `json:"lastAuthor,omitempty"`
}

// Node is the HTTP representation of a page tree node.
type Node struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Slug     string        `json:"slug"`
	Path     string        `json:"path"`
	Version  string        `json:"version"`
	Position int           `json:"position"`
	Kind     tree.NodeKind `json:"kind"`
	Children []*Node       `json:"children"`
	Metadata NodeMetadata  `json:"metadata"`
}

// Page is the HTTP representation of a full page (node + content).
type Page struct {
	*Node
	Content string `json:"content"`
	Path    string `json:"path"`
}

// ToAPIPage converts a tree.Page to its HTTP representation.
func ToAPIPage(p *tree.Page, userResolver *auth.UserResolver) *Page {
	return &Page{
		Node:    ToAPINode(p.PageNode, "", userResolver),
		Content: p.Content,
		Path:    BuildPathFromNode(p.PageNode),
	}
}

// ToAPIPageWithDepth converts a tree.Page with a depth-limited node tree.
func ToAPIPageWithDepth(p *tree.Page, userResolver *auth.UserResolver, depth int) *Page {
	return &Page{
		Node:    ToAPINodeWithDepth(p.PageNode, "", userResolver, depth),
		Content: p.Content,
		Path:    BuildPathFromNode(p.PageNode),
	}
}

// BuildPathFromNode builds the slash-separated path string from a node.
func BuildPathFromNode(node *tree.PageNode) string {
	var parts []string
	current := node
	for current != nil && current.Slug != "root" {
		parts = append([]string{current.Slug}, parts...)
		current = current.Parent
	}
	return strings.Join(parts, "/")
}

// ToAPINode recursively converts a tree.PageNode to its HTTP representation.
func ToAPINode(node *tree.PageNode, parentPath string, userResolver *auth.UserResolver) *Node {
	path := node.Slug
	if node.Slug == "root" {
		path = ""
	}
	if node.Slug != "root" && parentPath != "" {
		path = parentPath + "/" + node.Slug
	}

	var creator, lastAuthor *auth.UserLabel
	if userResolver != nil {
		creator, _ = userResolver.ResolveUserLabel(node.Metadata.CreatorID)
		lastAuthor, _ = userResolver.ResolveUserLabel(node.Metadata.LastAuthorID)
	}

	apiNode := &Node{
		ID:       node.ID,
		Title:    node.Title,
		Slug:     node.Slug,
		Path:     path,
		Version:  node.Version(),
		Position: node.Position,
		Kind:     node.Kind,
		Metadata: NodeMetadata{
			CreatedAt:    node.Metadata.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    node.Metadata.UpdatedAt.Format(time.RFC3339),
			CreatorID:    node.Metadata.CreatorID,
			LastAuthorID: node.Metadata.LastAuthorID,
			Creator:      creator,
			LastAuthor:   lastAuthor,
		},
	}

	for _, child := range node.Children {
		apiNode.Children = append(apiNode.Children, ToAPINode(child, path, userResolver))
	}

	return apiNode
}

// pruneNodeDepth limits the node tree to the given depth.
// depth == 0 → drop all children; depth < 0 → unlimited.
func pruneNodeDepth(n *Node, depth int) {
	if n == nil {
		return
	}
	if depth == 0 {
		n.Children = nil
		return
	}
	if depth < 0 {
		return
	}
	for _, child := range n.Children {
		pruneNodeDepth(child, depth-1)
	}
}

// ToAPINodeWithDepth converts a node with depth limiting.
func ToAPINodeWithDepth(node *tree.PageNode, parentPath string, userResolver *auth.UserResolver, depth int) *Node {
	apiNode := ToAPINode(node, parentPath, userResolver)
	if depth < 0 {
		return apiNode
	}
	pruneNodeDepth(apiNode, depth)
	return apiNode
}

// FormatAPITime formats a time.Time to RFC3339 or empty string for zero time.
func FormatAPITime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}
