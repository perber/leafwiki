package pages

import (
	"context"
	"log/slog"

	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
)

// ConvertPageInput is the input for ConvertPageUseCase.
type ConvertPageInput struct {
	UserID     string
	ID         string
	Version    string
	TargetKind tree.NodeKind
}

// ConvertPageUseCase converts a page to a different node kind (page ↔ section).
type ConvertPageUseCase struct {
	tree     *tree.TreeService
	revision *revision.Service
	log      *slog.Logger
}

// NewConvertPageUseCase constructs a ConvertPageUseCase.
func NewConvertPageUseCase(t *tree.TreeService, r *revision.Service, log *slog.Logger) *ConvertPageUseCase {
	return &ConvertPageUseCase{tree: t, revision: r, log: log}
}

// Execute converts the node kind and records a structure revision.
func (uc *ConvertPageUseCase) Execute(_ context.Context, in ConvertPageInput) error {
	if in.ID == "root" || in.ID == "" {
		return newPageRootOperationError("convert")
	}
	page, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return err
	}
	if err := requireCurrentPageVersion(page, in.Version); err != nil {
		return err
	}
	if err := uc.tree.ConvertNode(in.UserID, in.ID, in.TargetKind); err != nil {
		return err
	}
	if uc.revision != nil {
		if _, _, err := uc.revision.RecordStructureChange(in.ID, in.UserID, ""); err != nil {
			uc.log.Warn("failed to record structure revision", "pageID", in.ID, "error", err)
		}
	}
	return nil
}
