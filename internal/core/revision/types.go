package revision

import "time"

type RevisionType string

const (
	RevisionTypeContentUpdate RevisionType = "content_update"
	RevisionTypeAssetUpdate   RevisionType = "asset_update"
	RevisionTypeDelete        RevisionType = "delete"
)

type AssetRef struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
	MIMEType  string `json:"mime_type,omitempty"`
}

type RevisionState struct {
	PageID            string
	Title             string
	Slug              string
	Kind              string
	Path              string
	Content           string
	ContentHash       string
	Assets            []AssetRef
	AssetManifestHash string
	CapturedAt        time.Time
}

type Revision struct {
	ID                string       `json:"id"`
	PageID            string       `json:"page_id"`
	Type              RevisionType `json:"type"`
	AuthorID          string       `json:"author_id"`
	CreatedAt         time.Time    `json:"created_at"`
	Title             string       `json:"title"`
	Slug              string       `json:"slug"`
	Kind              string       `json:"kind"`
	Path              string       `json:"path"`
	ContentHash       string       `json:"content_hash"`
	AssetManifestHash string       `json:"asset_manifest_hash"`
	Summary           string       `json:"summary,omitempty"`
}

type TrashEntry struct {
	PageID         string    `json:"page_id"`
	DeletedAt      time.Time `json:"deleted_at"`
	DeletedBy      string    `json:"deleted_by"`
	Title          string    `json:"title"`
	Slug           string    `json:"slug"`
	Path           string    `json:"path"`
	LastRevisionID string    `json:"last_revision_id"`
}

type assetManifest struct {
	Items []AssetRef `json:"items"`
}
