package dto

import (
	"time"

	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
	coreprop "github.com/perber/wiki/internal/properties"
)

// PropertyEntry is the HTTP representation of a single property value.
type PropertyEntry struct {
	Value string `json:"value"`
	Type  string `json:"type"` // currently always "text"
}

// PropertyKeyCount is returned by the property key listing endpoint.
type PropertyKeyCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// PropertyPage is the reduced page DTO returned by the property filter endpoint.
type PropertyPage struct {
	ID           string                   `json:"id"`
	Title        string                   `json:"title"`
	Path         string                   `json:"path"`
	Properties   map[string]PropertyEntry `json:"properties"`
	CreatedAt    string                   `json:"createdAt,omitempty"`
	UpdatedAt    string                   `json:"updatedAt,omitempty"`
	CreatorID    string                   `json:"creatorId,omitempty"`
	LastAuthorID string                   `json:"lastAuthorId,omitempty"`
	LastAuthor   *auth.UserLabel          `json:"lastAuthor,omitempty"`
}

// ToPropertyPage builds a PropertyPage from a PageNode, its properties, and an optional user resolver.
func ToPropertyPage(node *tree.PageNode, props map[string]coreprop.PropertyEntry, userResolver *auth.UserResolver) *PropertyPage {
	apiProps := make(map[string]PropertyEntry, len(props))
	for k, e := range props {
		apiProps[k] = PropertyEntry{Value: e.Value, Type: e.Type}
	}

	p := &PropertyPage{
		ID:           node.ID,
		Title:        node.Title,
		Path:         BuildPathFromNode(node),
		Properties:   apiProps,
		CreatorID:    node.Metadata.CreatorID,
		LastAuthorID: node.Metadata.LastAuthorID,
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
