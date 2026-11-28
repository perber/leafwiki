package revisions

import (
	"context"
	"log/slog"
	"time"

	"github.com/perber/wiki/internal/core/revision"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/wiki/pagesave"
)

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
	revision     *revision.Service
	tree         *tree.TreeService
	orchestrator *pagesave.PageSaveOrchestrator
	log          *slog.Logger
	metrics      *httpmetrics.HTTPMetrics
}

func NewRestoreRevisionUseCase(r *revision.Service, t *tree.TreeService, o *pagesave.PageSaveOrchestrator, log *slog.Logger, metrics *httpmetrics.HTTPMetrics) *RestoreRevisionUseCase {
	return &RestoreRevisionUseCase{revision: r, tree: t, orchestrator: o, log: log, metrics: metrics}
}

func (uc *RestoreRevisionUseCase) Execute(_ context.Context, in RestoreRevisionInput) (out *RestoreRevisionOutput, err error) {
	started := time.Now()
	defer func() {
		uc.metrics.ObservePageSaveWorkflow(string(pagesave.PageOperationRestore), err, started)
	}()

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
	uc.orchestrator.Run(pagesave.PageSaveEvent{
		Operation: pagesave.PageOperationRestore,
		UserID:    in.UserID,
		After:     page,
	})
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
