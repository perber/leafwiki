package pages

import (
	"context"

	"github.com/perber/wiki/internal/favorites"
)

type RemoveFavoriteInput struct {
	UserID string
	PageID string
}

// RemoveFavoriteUseCase un-favorites a page for a given user. Idempotent —
// removing a page that isn't favorited (or no longer exists) is not an error.
type RemoveFavoriteUseCase struct {
	store *favorites.FavoritesStore
}

func NewRemoveFavoriteUseCase(store *favorites.FavoritesStore) *RemoveFavoriteUseCase {
	return &RemoveFavoriteUseCase{store: store}
}

func (uc *RemoveFavoriteUseCase) Execute(_ context.Context, in RemoveFavoriteInput) error {
	return uc.store.Remove(in.UserID, in.PageID)
}
