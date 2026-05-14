package dto

import (
	"time"

	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
)

// TagCount is returned by the tag listing endpoint.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// TaggedPage is the reduced page DTO returned by the tag filter endpoint.
type TaggedPage struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Path         string          `json:"path"`
	Excerpt      string          `json:"excerpt,omitempty"`
	Tags         []string        `json:"tags"`
	CreatedAt    string          `json:"createdAt,omitempty"`
	UpdatedAt    string          `json:"updatedAt,omitempty"`
	CreatorID    string          `json:"creatorId,omitempty"`
	LastAuthorID string          `json:"lastAuthorId,omitempty"`
	LastAuthor   *auth.UserLabel `json:"lastAuthor,omitempty"`
}

// ToTaggedPage builds a TaggedPage from a PageNode, its tags, and an optional user resolver.
func ToTaggedPage(node *tree.PageNode, pageTags []string, excerpt string, userResolver *auth.UserResolver) *TaggedPage {
	if pageTags == nil {
		pageTags = []string{}
	}

	p := &TaggedPage{
		ID:           node.ID,
		Title:        node.Title,
		Path:         BuildPathFromNode(node),
		Excerpt:      excerpt,
		Tags:         pageTags,
		LastAuthorID: node.Metadata.LastAuthorID,
		CreatorID:    node.Metadata.CreatorID,
	}

	if !node.Metadata.CreatedAt.IsZero() {
		p.CreatedAt = node.Metadata.CreatedAt.Format(time.RFC3339)
	}
	if !node.Metadata.UpdatedAt.IsZero() {
		p.UpdatedAt = node.Metadata.UpdatedAt.Format(time.RFC3339)
	}

	if userResolver != nil {
		p.LastAuthor, _ = userResolver.ResolveUserLabel(node.Metadata.LastAuthorID)
	}

	return p
}
