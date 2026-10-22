package sharing

import (
	"errors"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// Error codes are part of the API contract (see docs/development/conventions.md#errors).
const (
	ErrCodeShareLinkNotFound         = "share_link_not_found"
	ErrCodeShareLinkInvalidPage      = "share_link_invalid_page"
	ErrCodeShareLinkInvalidExpiry    = "share_link_invalid_expiry"
	ErrCodeShareLinkStoreUnavailable = "share_link_store_unavailable"
)

// ErrShareLinkNotFound is wrapped by every "no such link" answer (unknown,
// malformed, expired, revoked), so a link holder can't probe which links exist.
var ErrShareLinkNotFound = errors.New("share link not found")

// errLinkNotStored is the store's "no row" result; it never leaves the package.
var errLinkNotStored = errors.New("share link not stored")

func errShareLinkNotFound() error {
	return sharederrors.NewLocalizedError(ErrCodeShareLinkNotFound,
		"Share link not found", "share link not found", ErrShareLinkNotFound)
}

func errShareLinkInvalidPage() error {
	return sharederrors.NewLocalizedError(ErrCodeShareLinkInvalidPage,
		"The page to share does not exist", "the page to share does not exist", nil)
}

func errShareLinkInvalidExpiry() error {
	return sharederrors.NewLocalizedError(ErrCodeShareLinkInvalidExpiry,
		"The expiry date must be in the future", "the expiry date must be in the future", nil)
}

// errShareLinkStoreUnavailable is returned while the store is suspended for
// an in-progress live restore. Mirrors favorites.errFavoritesStoreUnavailable.
func errShareLinkStoreUnavailable() error {
	return sharederrors.NewLocalizedError(ErrCodeShareLinkStoreUnavailable,
		"The server is restoring from a backup — please try again in a moment",
		"share link store is suspended for an in-progress restore", nil)
}
