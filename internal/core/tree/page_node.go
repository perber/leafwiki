package tree

import "time"

// PageMetadata hält einfache Metadaten zu einer Seite.
type PageMetadata struct {
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatorID    string    `json:"creatorId"`
	LastAuthorID string    `json:"lastAuthorId"`
}

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

	Metadata PageMetadata `json:"metadata"` // Metadata holds metadata about the page
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

func (p *PageNode) IsChildOf(childID string, recursive bool) bool {
	for _, child := range p.Children {
		if child.ID == childID {
			return true
		}
		if recursive && child.IsChildOf(childID, recursive) {
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
