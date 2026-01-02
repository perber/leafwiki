package api

import "github.com/perber/wiki/internal/core/auth"

type NodeMetadata struct {
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	CreatorID    string `json:"creatorId"`
	LastAuthorID string `json:"lastAuthorId"`

	Creator    *auth.UserLabel `json:"creator,omitempty"`
	LastAuthor *auth.UserLabel `json:"lastAuthor,omitempty"`
}

type Node struct {
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	Slug     string       `json:"slug"`
	Path     string       `json:"path"`
	Position int          `json:"position"`
	Children []*Node      `json:"children"`
	Metadata NodeMetadata `json:"metadata"`
}
