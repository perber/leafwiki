package assets

import (
	"context"
	"log/slog"
	"mime/multipart"

	coreassets "github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
)

// ─── UploadAssetUseCase ──────────────────────────────────────────────────────

type UploadAssetInput struct {
	UserID   string
	PageID   string
	File     multipart.File
	Filename string
	MaxBytes int64
}

type UploadAssetOutput struct {
	URL string
}

type UploadAssetUseCase struct {
	tree     *tree.TreeService
	asset    *coreassets.AssetService
	revision *revision.Service
	log      *slog.Logger
}

func NewUploadAssetUseCase(t *tree.TreeService, a *coreassets.AssetService, r *revision.Service, log *slog.Logger) *UploadAssetUseCase {
	return &UploadAssetUseCase{tree: t, asset: a, revision: r, log: log}
}

func (uc *UploadAssetUseCase) Execute(_ context.Context, in UploadAssetInput) (*UploadAssetOutput, error) {
	page, err := uc.tree.FindPageByID(in.PageID)
	if err != nil {
		return nil, err
	}
	url, err := uc.asset.SaveAssetForPage(page, in.File, in.Filename, in.MaxBytes)
	if err != nil {
		return nil, err
	}
	recordAssetRevision(uc.revision, in.PageID, in.UserID, uc.log)
	return &UploadAssetOutput{URL: url}, nil
}

// ─── ListAssetsUseCase ───────────────────────────────────────────────────────

type ListAssetsInput struct {
	PageID string
}

type ListAssetsOutput struct {
	Files []string
}

type ListAssetsUseCase struct {
	tree  *tree.TreeService
	asset *coreassets.AssetService
}

func NewListAssetsUseCase(t *tree.TreeService, a *coreassets.AssetService) *ListAssetsUseCase {
	return &ListAssetsUseCase{tree: t, asset: a}
}

func (uc *ListAssetsUseCase) Execute(_ context.Context, in ListAssetsInput) (*ListAssetsOutput, error) {
	page, err := uc.tree.FindPageByID(in.PageID)
	if err != nil {
		return nil, err
	}
	files, err := uc.asset.ListAssetsForPage(page)
	if err != nil {
		return nil, err
	}
	return &ListAssetsOutput{Files: files}, nil
}

// ─── RenameAssetUseCase ──────────────────────────────────────────────────────

type RenameAssetInput struct {
	UserID      string
	PageID      string
	OldFilename string
	NewFilename string
}

type RenameAssetOutput struct {
	URL string
}

type RenameAssetUseCase struct {
	tree     *tree.TreeService
	asset    *coreassets.AssetService
	revision *revision.Service
	log      *slog.Logger
}

func NewRenameAssetUseCase(t *tree.TreeService, a *coreassets.AssetService, r *revision.Service, log *slog.Logger) *RenameAssetUseCase {
	return &RenameAssetUseCase{tree: t, asset: a, revision: r, log: log}
}

func (uc *RenameAssetUseCase) Execute(_ context.Context, in RenameAssetInput) (*RenameAssetOutput, error) {
	page, err := uc.tree.FindPageByID(in.PageID)
	if err != nil {
		return nil, err
	}
	newPath, err := uc.asset.RenameAsset(page, in.OldFilename, in.NewFilename)
	if err != nil {
		return nil, err
	}
	recordAssetRevision(uc.revision, in.PageID, in.UserID, uc.log)
	return &RenameAssetOutput{URL: newPath}, nil
}

// ─── DeleteAssetUseCase ──────────────────────────────────────────────────────

type DeleteAssetInput struct {
	UserID   string
	PageID   string
	Filename string
}

type DeleteAssetUseCase struct {
	tree     *tree.TreeService
	asset    *coreassets.AssetService
	revision *revision.Service
	log      *slog.Logger
}

func NewDeleteAssetUseCase(t *tree.TreeService, a *coreassets.AssetService, r *revision.Service, log *slog.Logger) *DeleteAssetUseCase {
	return &DeleteAssetUseCase{tree: t, asset: a, revision: r, log: log}
}

func (uc *DeleteAssetUseCase) Execute(_ context.Context, in DeleteAssetInput) error {
	page, err := uc.tree.FindPageByID(in.PageID)
	if err != nil {
		return err
	}
	if err := uc.asset.DeleteAsset(page, in.Filename); err != nil {
		return err
	}
	recordAssetRevision(uc.revision, in.PageID, in.UserID, uc.log)
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func recordAssetRevision(svc *revision.Service, pageID, userID string, log *slog.Logger) {
	if svc == nil {
		return
	}
	if _, _, err := svc.RecordAssetChange(pageID, userID, ""); err != nil {
		log.Warn("failed to record asset revision", "pageID", pageID, "error", err)
	}
}
