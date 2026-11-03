package pages

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/favorites"
)

// TestRespondWithPageError_FavoritesStoreUnavailable_Returns503 is the
// regression test for a real bug found in review: favorites use cases
// propagate favorites.ErrCodeFavoritesStoreUnavailable (surfaced while a
// live restore has favorites.db suspended) unwrapped, but pageErrorStatus
// had no case for it and fell through to the generic 500 default — unlike
// apikeys' equivalent case for its own store, which already maps to 503. A
// client can't tell "retry shortly" apart from "something is broken"
// without this.
func TestRespondWithPageError_FavoritesStoreUnavailable_Returns503(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondWithPageError(c, sharederrors.NewLocalizedError(
		favorites.ErrCodeFavoritesStoreUnavailable,
		"The server is restoring from a backup — please try again in a moment",
		"favorites store is suspended for an in-progress restore",
		nil,
	))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// TestMapPageError pins the sentinel table the REST route and every non-HTTP
// consumer share: each sentinel gets its domain code and client-facing
// message, and an unknown error falls into the internal-error arm.
func TestMapPageError(t *testing.T) {
	unknown := errors.New("disk on fire")
	tests := []struct {
		name        string
		err         error
		wantCode    string
		wantMessage string
	}{
		{name: "page not found", err: tree.ErrPageNotFound, wantCode: ErrCodePageNotFound, wantMessage: "Page not found"},
		{name: "parent not found", err: tree.ErrParentNotFound, wantCode: ErrCodePageParentNotFound, wantMessage: "Parent page not found"},
		{name: "page has children", err: tree.ErrPageHasChildren, wantCode: ErrCodePageHasChildren, wantMessage: "Page has children, use recursive delete"},
		{name: "slug conflict", err: tree.ErrPageAlreadyExists, wantCode: ErrCodePageSlugConflict, wantMessage: "Page already exists"},
		{name: "circular move", err: tree.ErrMovePageCircularReference, wantCode: ErrCodePageCircularMove, wantMessage: "Move would create a circular reference"},
		{name: "move to self", err: tree.ErrPageCannotBeMovedToItself, wantCode: ErrCodePageCannotMoveToSelf, wantMessage: "Page cannot be moved to itself"},
		{name: "convert not allowed", err: tree.ErrConvertNotAllowed, wantCode: ErrCodePageConvertNotAllowed, wantMessage: "Convert operation not allowed"},
		{name: "version conflict", err: tree.ErrVersionConflict, wantCode: ErrCodePageVersionConflict, wantMessage: "Page was changed by another request"},
		{name: "version required", err: tree.ErrVersionRequired, wantCode: ErrCodePageVersionRequired, wantMessage: "Page version is required"},
		{name: "tree not loaded", err: tree.ErrTreeNotLoaded, wantCode: ErrCodePageInternalError, wantMessage: "Tree not loaded"},
		{name: "wrapped sentinel keeps its code", err: errors.Join(fmt.Errorf("creating page: %w", tree.ErrPageNotFound)), wantCode: ErrCodePageNotFound, wantMessage: "Page not found"},
		{name: "unknown error is the internal-error arm", err: unknown, wantCode: ErrCodePageInternalError, wantMessage: "disk on fire"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc := MapPageError(tc.err)
			if loc.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", loc.Code, tc.wantCode)
			}
			if loc.Message != tc.wantMessage {
				t.Fatalf("message = %q, want %q", loc.Message, tc.wantMessage)
			}
		})
	}
}
