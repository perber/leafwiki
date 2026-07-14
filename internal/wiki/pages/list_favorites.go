package pages

import (
	"context"
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/favorites"
)

type ListFavoritesInput struct {
	UserID string
}

type ListFavoritesOutput struct {
	Pages []*tree.Page
}

// ListFavoritesUseCase resolves a user's favorited page IDs to full pages,
// silently skipping ids that no longer resolve (e.g. leftover rows from
// before the delete-cascade cleanup shipped).
type ListFavoritesUseCase struct {
	treeService *tree.TreeService
	store       *favorites.FavoritesStore
	log         *slog.Logger
}

func NewListFavoritesUseCase(treeService *tree.TreeService, store *favorites.FavoritesStore, log *slog.Logger) *ListFavoritesUseCase {
	return &ListFavoritesUseCase{treeService: treeService, store: store, log: log}
}

func (uc *ListFavoritesUseCase) Execute(_ context.Context, in ListFavoritesInput) (*ListFavoritesOutput, error) {
	ids, err := uc.store.ListPageIDsForUser(in.UserID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return &ListFavoritesOutput{Pages: []*tree.Page{}}, nil
	}

	pages, errs := uc.treeService.GetPages(ids)
	result := make([]*tree.Page, 0, len(pages))
	for i, p := range pages {
		if errs[i] != nil {
			uc.log.Warn("skipping stale favorite", "userID", in.UserID, "pageID", ids[i], "error", errs[i])
			continue
		}
		result = append(result, p)
	}
	return &ListFavoritesOutput{Pages: result}, nil
}
