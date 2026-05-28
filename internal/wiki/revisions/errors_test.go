package revisions

import (
	"net/http"
	"testing"
)

func TestRevisionErrorStatus_InvalidLimitIsBadRequest(t *testing.T) {
	if got := revisionErrorStatus(ErrCodeRevisionInvalidLimit); got != http.StatusBadRequest {
		t.Fatalf("revisionErrorStatus(%q) = %d, want %d", ErrCodeRevisionInvalidLimit, got, http.StatusBadRequest)
	}
}

func TestRevisionErrorStatus_RevisionNotFoundIsNotFound(t *testing.T) {
	if got := revisionErrorStatus(ErrCodeRevisionNotFound); got != http.StatusNotFound {
		t.Fatalf("revisionErrorStatus(%q) = %d, want %d", ErrCodeRevisionNotFound, got, http.StatusNotFound)
	}
}
