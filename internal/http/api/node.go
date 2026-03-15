package api

import (
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
)

type NodeMetadata struct {
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	CreatorID    string `json:"creatorId"`
	LastAuthorID string `json:"lastAuthorId"`

	Creator    *auth.UserLabel `json:"creator,omitempty"`
	LastAuthor *auth.UserLabel `json:"lastAuthor,omitempty"`
}

type Node struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Slug         string           `json:"slug"`
	Path         string           `json:"path"`
	RepairNeeded bool             `json:"repairNeeded,omitempty"`
	Issues       []tree.NodeIssue `json:"issues,omitempty"`
	// Position is derived from the current sibling order in the read model.
	// It is not canonical persisted state.
	Position int           `json:"position"`
	Kind     tree.NodeKind `json:"kind"`
	Children []*Node       `json:"children"`
	Metadata NodeMetadata  `json:"metadata"`
}
