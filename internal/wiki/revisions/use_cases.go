package revisions

import (
	"context"
	"log/slog"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/core/revision"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
)


// ─── DTO types ───────────────────────────────────────────────────────────────

// RevisionResponse is the JSON representation of a revision.
type RevisionResponse struct {
	ID                string              `json:"id"`
	PageID            string              `json:"pageId"`
	ParentID          string              `json:"parentId,omitempty"`
	Type              string              `json:"type"`
	AuthorID          string              `json:"authorId"`
	Author            *coreauth.UserLabel `json:"author,omitempty"`
	CreatedAt         string              `json:"createdAt"`
	Title             string              `json:"title"`
	Slug              string              `json:"slug"`
	Kind              string              `json:"kind"`
	Path              string              `json:"path"`
	ContentHash       string              `json:"contentHash"`
	AssetManifestHash string              `json:"assetManifestHash"`
	PageCreatedAt     string              `json:"pageCreatedAt,omitempty"`
	PageUpdatedAt     string              `json:"pageUpdatedAt,omitempty"`
	CreatorID         string              `json:"creatorId,omitempty"`
	LastAuthorID      string              `json:"lastAuthorId,omitempty"`
	Summary           string              `json:"summary,omitempty"`
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
	var author *coreauth.UserLabel
	if userResolver != nil {
		author, _ = userResolver.ResolveUserLabel(rev.AuthorID)
	}
	return &RevisionResponse{
		ID:                rev.ID,
		PageID:            rev.PageID,
		ParentID:          rev.ParentID,
		Type:              string(rev.Type),
		AuthorID:          rev.AuthorID,
		Author:            author,
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


// ─── ListRevisionsUseCase ────────────────────────────────────────────────────

type ListRevisionsInput struct {
	PageID string
	Cursor string
	Limit  int
}

type ListRevisionsOutput struct {
	Revisions  []*revision.Revision
	NextCursor string
}

type ListRevisionsUseCase struct {
	revision *revision.Service
}

func NewListRevisionsUseCase(r *revision.Service) *ListRevisionsUseCase {
	return &ListRevisionsUseCase{revision: r}
}

func (uc *ListRevisionsUseCase) Execute(_ context.Context, in ListRevisionsInput) (*ListRevisionsOutput, error) {
	if uc.revision == nil {
		return &ListRevisionsOutput{Revisions: []*revision.Revision{}}, nil
	}
	revs, nextCursor, err := uc.revision.ListRevisionsPage(in.PageID, in.Cursor, in.Limit)
	if err != nil {
		return nil, err
	}
	return &ListRevisionsOutput{Revisions: revs, NextCursor: nextCursor}, nil
}

// ─── GetRevisionUseCase ──────────────────────────────────────────────────────

type GetRevisionInput struct {
	PageID     string
	RevisionID string
}

type GetRevisionOutput struct {
	Snapshot *revision.RevisionSnapshot
}

type GetRevisionUseCase struct {
	revision *revision.Service
}

func NewGetRevisionUseCase(r *revision.Service) *GetRevisionUseCase {
	return &GetRevisionUseCase{revision: r}
}

func (uc *GetRevisionUseCase) Execute(_ context.Context, in GetRevisionInput) (*GetRevisionOutput, error) {
	if uc.revision == nil {
		return &GetRevisionOutput{}, nil
	}
	snapshot, err := uc.revision.GetRevisionSnapshot(in.PageID, in.RevisionID)
	if err != nil {
		return nil, err
	}
	return &GetRevisionOutput{Snapshot: snapshot}, nil
}

// ─── CompareRevisionsUseCase ─────────────────────────────────────────────────

type CompareRevisionsInput struct {
	PageID           string
	BaseRevisionID   string
	TargetRevisionID string
}

type CompareRevisionsOutput struct {
	Comparison *revision.RevisionComparison
}

type CompareRevisionsUseCase struct {
	revision *revision.Service
}

func NewCompareRevisionsUseCase(r *revision.Service) *CompareRevisionsUseCase {
	return &CompareRevisionsUseCase{revision: r}
}

func (uc *CompareRevisionsUseCase) Execute(_ context.Context, in CompareRevisionsInput) (*CompareRevisionsOutput, error) {
	if uc.revision == nil {
		return &CompareRevisionsOutput{}, nil
	}
	comparison, err := uc.revision.CompareRevisionSnapshots(in.PageID, in.BaseRevisionID, in.TargetRevisionID)
	if err != nil {
		return nil, err
	}
	return &CompareRevisionsOutput{Comparison: comparison}, nil
}

// ─── GetRevisionAssetUseCase ─────────────────────────────────────────────────

type GetRevisionAssetInput struct {
	PageID     string
	RevisionID string
	AssetName  string
}

type GetRevisionAssetOutput struct {
	Asset *revision.RevisionAssetContent
}

type GetRevisionAssetUseCase struct {
	revision *revision.Service
}

func NewGetRevisionAssetUseCase(r *revision.Service) *GetRevisionAssetUseCase {
	return &GetRevisionAssetUseCase{revision: r}
}

func (uc *GetRevisionAssetUseCase) Execute(_ context.Context, in GetRevisionAssetInput) (*GetRevisionAssetOutput, error) {
	if uc.revision == nil {
		return &GetRevisionAssetOutput{}, nil
	}
	asset, err := uc.revision.GetRevisionAsset(in.PageID, in.RevisionID, in.AssetName)
	if err != nil {
		return nil, err
	}
	return &GetRevisionAssetOutput{Asset: asset}, nil
}

// ─── GetLatestRevisionUseCase ────────────────────────────────────────────────

type GetLatestRevisionInput struct {
	PageID string
}

type GetLatestRevisionOutput struct {
	Revision *revision.Revision
}

type GetLatestRevisionUseCase struct {
	revision *revision.Service
}

func NewGetLatestRevisionUseCase(r *revision.Service) *GetLatestRevisionUseCase {
	return &GetLatestRevisionUseCase{revision: r}
}

func (uc *GetLatestRevisionUseCase) Execute(_ context.Context, in GetLatestRevisionInput) (*GetLatestRevisionOutput, error) {
	if uc.revision == nil {
		return &GetLatestRevisionOutput{}, nil
	}
	rev, err := uc.revision.GetLatestRevision(in.PageID)
	if err != nil {
		return nil, err
	}
	return &GetLatestRevisionOutput{Revision: rev}, nil
}

// ─── RestoreRevisionUseCase ──────────────────────────────────────────────────

type RestoreRevisionInput struct {
	UserID     string
	PageID     string
	RevisionID string
}

type RestoreRevisionOutput struct {
	Page *tree.Page
}

type RestoreRevisionUseCase struct {
	revision *revision.Service
	tree     *tree.TreeService
	links    *links.LinkService
	log      *slog.Logger
}

func NewRestoreRevisionUseCase(r *revision.Service, t *tree.TreeService, l *links.LinkService, log *slog.Logger) *RestoreRevisionUseCase {
	return &RestoreRevisionUseCase{revision: r, tree: t, links: l, log: log}
}

func (uc *RestoreRevisionUseCase) Execute(_ context.Context, in RestoreRevisionInput) (*RestoreRevisionOutput, error) {
	if uc.revision == nil {
		return nil, sharederrors.NewLocalizedError(
			ErrCodeRevisionServiceUnavailable,
			"Revision service is not available",
			"revision service is not available",
			nil,
		)
	}
	if err := uc.revision.RestoreRevision(in.PageID, in.RevisionID, in.UserID); err != nil {
		return nil, err
	}
	page, err := uc.tree.GetPage(in.PageID)
	if err != nil {
		return nil, err
	}
	if uc.links != nil {
		if err := uc.links.UpdateLinksForPage(page, page.Content); err != nil {
			uc.log.Warn("failed to update links for restored revision", "pageID", in.PageID, "revisionID", in.RevisionID, "error", err)
		}
		if err := uc.links.HealLinksForExactPath(page); err != nil {
			uc.log.Warn("failed to heal links for restored revision", "pageID", in.PageID, "revisionID", in.RevisionID, "error", err)
		}
	}
	return &RestoreRevisionOutput{Page: page}, nil
}


// ─── CheckIntegrityUseCase ───────────────────────────────────────────────────

type CheckIntegrityInput struct {
	PageID string
}

type CheckIntegrityOutput struct {
	Issues []revision.RevisionIntegrityIssue
}

type CheckIntegrityUseCase struct {
	revision *revision.Service
}

func NewCheckIntegrityUseCase(r *revision.Service) *CheckIntegrityUseCase {
	return &CheckIntegrityUseCase{revision: r}
}

func (uc *CheckIntegrityUseCase) Execute(_ context.Context, in CheckIntegrityInput) (*CheckIntegrityOutput, error) {
	if uc.revision == nil {
		return &CheckIntegrityOutput{Issues: []revision.RevisionIntegrityIssue{}}, nil
	}
	issues, err := uc.revision.CheckRevisionIntegrity(in.PageID)
	if err != nil {
		return nil, err
	}
	return &CheckIntegrityOutput{Issues: issues}, nil
}
