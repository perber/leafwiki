package tree

import "time"

type legacyPageMetadata struct {
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatorID    string    `json:"creatorId"`
	LastAuthorID string    `json:"lastAuthorId"`
}

type legacyPageNode struct {
	ID       string             `json:"id"`
	Title    string             `json:"title"`
	Slug     string             `json:"slug"`
	Children []*legacyPageNode  `json:"children"`
	Position int                `json:"position"`
	Kind     NodeKind           `json:"kind"`
	Metadata legacyPageMetadata `json:"metadata"`
}
