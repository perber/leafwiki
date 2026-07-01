package pages

import (
	"context"
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
)

type PinPageInput struct {
	ID      string
	Version string
	Pinned  bool
}

type PinPageOutput struct {
	Page *tree.Page
}

type PinPageUseCase struct {
	treeService *tree.TreeService
	log         *slog.Logger
}

func NewPinPageUseCase(treeService *tree.TreeService, log *slog.Logger) *PinPageUseCase {
	return &PinPageUseCase{treeService: treeService, log: log}
}

func (uc *PinPageUseCase) Execute(_ context.Context, in PinPageInput) (*PinPageOutput, error) {
	page, err := uc.treeService.SetPinned(in.ID, in.Version, in.Pinned)
	if err != nil {
		return nil, err
	}
	return &PinPageOutput{Page: page}, nil
}
