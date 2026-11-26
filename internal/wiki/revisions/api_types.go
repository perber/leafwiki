package revisions

import (
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/http/dto"
)

// ─── DTO types ───────────────────────────────────────────────────────────────

// RevisionResponse is the JSON representation of a revision.
type RevisionResponse struct {
	ID                string         `json:"id"`
	PageID            string         `json:"pageId"`
	ParentID          string         `json:"parentId,omitempty"`
	Type              string         `json:"type"`
	AuthorID          string         `json:"authorId"`
	Author            *dto.UserLabel `json:"author,omitempty"`
	CreatedAt         string         `json:"createdAt"`
	Title             string         `json:"title"`
	Slug              string         `json:"slug"`
	Kind              string         `json:"kind"`
	Path              string         `json:"path"`
	ContentHash       string         `json:"contentHash"`
	AssetManifestHash string         `json:"assetManifestHash"`
	PageCreatedAt     string         `json:"pageCreatedAt,omitempty"`
	PageUpdatedAt     string         `json:"pageUpdatedAt,omitempty"`
	CreatorID         string         `json:"creatorId,omitempty"`
	LastAuthorID      string         `json:"lastAuthorId,omitempty"`
	Summary           string         `json:"summary,omitempty"`
}

// RevisionAssetResponse is the JSON representation of a revision asset.
type RevisionAssetResponse struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
	MIMEType  string `json:"mimeType,omitempty"`
}

// RevisionSnapshotResponse is the JSON representation of a revision snapshot.
type RevisionSnapshotResponse struct {
	Revision *RevisionResponse       `json:"revision"`
	Content  string                  `json:"content"`
	Assets   []RevisionAssetResponse `json:"assets"`
}

// RevisionAssetDeltaResponse is the JSON representation of an asset delta.
type RevisionAssetDeltaResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// RevisionComparisonResponse is the JSON representation of a revision comparison.
type RevisionComparisonResponse struct {
	Base           *RevisionSnapshotResponse    `json:"base"`
	Target         *RevisionSnapshotResponse    `json:"target"`
	ContentChanged bool                         `json:"contentChanged"`
	AssetChanges   []RevisionAssetDeltaResponse `json:"assetChanges"`
}

// RevisionListResponse is the body of GET /api/pages/{id}/revisions. The field
// order matches the sorted keys of the map the handler answered with before, so
// the bytes on the wire are unchanged.
type RevisionListResponse struct {
	// NextCursor continues the list on the next request; empty on the last page.
	NextCursor string              `json:"nextCursor"`
	Revisions  []*RevisionResponse `json:"revisions"`
}

// ─── DTO mapper functions ─────────────────────────────────────────────────────

func formatTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}

func toRevisionResponse(rev *revision.Revision, userResolver *coreauth.UserResolver) *RevisionResponse {
	if rev == nil {
		return nil
	}
	return &RevisionResponse{
		ID:                rev.ID,
		PageID:            rev.PageID,
		ParentID:          rev.ParentID,
		Type:              string(rev.Type),
		AuthorID:          rev.AuthorID,
		Author:            dto.ResolveUserLabel(userResolver, rev.AuthorID),
		CreatedAt:         formatTime(rev.CreatedAt),
		Title:             rev.Title,
		Slug:              rev.Slug,
		Kind:              rev.Kind,
		Path:              rev.Path,
		ContentHash:       rev.ContentHash,
		AssetManifestHash: rev.AssetManifestHash,
		PageCreatedAt:     formatTime(rev.PageCreatedAt),
		PageUpdatedAt:     formatTime(rev.PageUpdatedAt),
		CreatorID:         rev.CreatorID,
		LastAuthorID:      rev.LastAuthorID,
		Summary:           rev.Summary,
	}
}

func toSnapshotResponse(snapshot *revision.RevisionSnapshot, userResolver *coreauth.UserResolver) *RevisionSnapshotResponse {
	if snapshot == nil {
		return nil
	}
	assets := make([]RevisionAssetResponse, 0, len(snapshot.Assets))
	for _, a := range snapshot.Assets {
		assets = append(assets, RevisionAssetResponse{Name: a.Name, SHA256: a.SHA256, SizeBytes: a.SizeBytes, MIMEType: a.MIMEType})
	}
	return &RevisionSnapshotResponse{
		Revision: toRevisionResponse(snapshot.Revision, userResolver),
		Content:  snapshot.Content,
		Assets:   assets,
	}
}

func toComparisonResponse(cmp *revision.RevisionComparison, userResolver *coreauth.UserResolver) *RevisionComparisonResponse {
	if cmp == nil {
		return nil
	}
	changes := make([]RevisionAssetDeltaResponse, 0, len(cmp.AssetChanges))
	for _, c := range cmp.AssetChanges {
		changes = append(changes, RevisionAssetDeltaResponse{Name: c.Name, Status: c.Status})
	}
	return &RevisionComparisonResponse{
		Base:           toSnapshotResponse(cmp.Base, userResolver),
		Target:         toSnapshotResponse(cmp.Target, userResolver),
		ContentChanged: cmp.ContentChanged,
		AssetChanges:   changes,
	}
}

// toRevisionListResponse never returns a nil Revisions, so an empty list is
// [] on the wire, never null.
func toRevisionListResponse(out *ListRevisionsOutput, userResolver *coreauth.UserResolver) *RevisionListResponse {
	revisions := make([]*RevisionResponse, 0, len(out.Revisions))
	for _, rev := range out.Revisions {
		revisions = append(revisions, toRevisionResponse(rev, userResolver))
	}
	return &RevisionListResponse{
		NextCursor: out.NextCursor,
		Revisions:  revisions,
	}
}
