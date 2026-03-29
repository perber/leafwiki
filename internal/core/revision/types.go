package revision

import "time"

type RevisionType string

const (
	RevisionTypeContentUpdate   RevisionType = "content_update"
	RevisionTypeAssetUpdate     RevisionType = "asset_update"
	RevisionTypeDelete          RevisionType = "delete"
	RevisionTypeRestore         RevisionType = "restore"
	RevisionTypeStructureUpdate RevisionType = "structure_update"
)

type AssetRef struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
	MIMEType  string `json:"mime_type,omitempty"`
}

type RevisionState struct {
	PageID            string
	ParentID          string
	Title             string
	Slug              string
	Kind              string
	Path              string
	Content           string
	ContentHash       string
	Assets            []AssetRef
	AssetManifestHash string
	PageCreatedAt     time.Time
	PageUpdatedAt     time.Time
	CreatorID         string
	LastAuthorID      string
	CapturedAt        time.Time
}

type Revision struct {
	ID                string       `json:"id"`
	PageID            string       `json:"page_id"`
	ParentID          string       `json:"parent_id,omitempty"`
	Type              RevisionType `json:"type"`
	AuthorID          string       `json:"author_id"`
	CreatedAt         time.Time    `json:"created_at"`
	Title             string       `json:"title"`
	Slug              string       `json:"slug"`
	Kind              string       `json:"kind"`
	Path              string       `json:"path"`
	ContentHash       string       `json:"content_hash"`
	AssetManifestHash string       `json:"asset_manifest_hash"`
	PageCreatedAt     time.Time    `json:"page_created_at"`
	PageUpdatedAt     time.Time    `json:"page_updated_at"`
	CreatorID         string       `json:"creator_id"`
	LastAuthorID      string       `json:"last_author_id"`
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

type RevisionIntegrityIssue struct {
	PageID     string `json:"page_id"`
	RevisionID string `json:"revision_id,omitempty"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Path       string `json:"path,omitempty"`
}

type RevisionSnapshot struct {
	Revision *Revision
	Content  string
	Assets   []AssetRef
}

type RevisionAssetContent struct {
	Asset   AssetRef
	Content []byte
}

type RevisionAssetDelta struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type RevisionComparison struct {
	Base           *RevisionSnapshot
	Target         *RevisionSnapshot
	ContentChanged bool
	AssetChanges   []RevisionAssetDelta
}
