package pagedisplay

import (
	"slices"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ErrCodeInvalidContentWidth is returned by SetContentWidth and Update for a
// value that is not one of ContentWidths(). The HTTP layer maps it to 400
// (see IsInvalidInput).
const ErrCodeInvalidContentWidth = "page_display_invalid_content_width"

// ErrCodeUpdateFailed is returned when a display setting other than the
// always-show-TOC flag (which keeps its historical toc_display_update_failed)
// cannot be persisted.
const ErrCodeUpdateFailed = "page_display_update_failed"

// Page content widths an admin can choose. The frontend maps each value to a
// reading-column width (index.css, [data-content-width]).
const (
	// ContentWidthDefault is the narrow reading column (680px).
	ContentWidthDefault = "default"
	// ContentWidthWide lifts the reading column to the page viewer's full
	// width, the layout before #1696.
	ContentWidthWide = "wide"
)

// contentWidths lists the valid values, the default first. A new width is
// added here and as a CSS rule plus a UI option in the frontend.
var contentWidths = []string{ContentWidthDefault, ContentWidthWide}

// ContentWidths returns the valid content width values, the default first.
func ContentWidths() []string { return slices.Clone(contentWidths) }

func isValidContentWidth(v string) bool { return slices.Contains(contentWidths, v) }

// normalizeContentWidth maps a stored value to a valid one: files written
// before the setting existed, or edited by hand, fall back to the default.
func normalizeContentWidth(v string) string {
	if isValidContentWidth(v) {
		return v
	}
	return ContentWidthDefault
}

func errInvalidContentWidth(v string) error {
	return sharederrors.NewLocalizedError(
		ErrCodeInvalidContentWidth,
		"Invalid page content width",
		"invalid page content width",
		nil,
		v,
	)
}
