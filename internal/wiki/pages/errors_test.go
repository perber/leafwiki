package pages

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
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
