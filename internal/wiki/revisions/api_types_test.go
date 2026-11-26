package revisions

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/revision"
)

// TestRevisionListResponse_Marshal_MatchesGinH proves the named list model
// marshals to the same bytes as the gin.H the list handler answered with
// before (encoding/json sorts map keys, so nextCursor comes first).
func TestRevisionListResponse_Marshal_MatchesGinH(t *testing.T) {
	created := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		out  *ListRevisionsOutput
	}{
		{"no revisions, empty cursor", &ListRevisionsOutput{}},
		{"two revisions, cursor abc", &ListRevisionsOutput{
			Revisions: []*revision.Revision{
				{ID: "rev-2", PageID: "page-1", ParentID: "rev-1", Type: revision.RevisionTypeContentUpdate, AuthorID: "u1", CreatedAt: created, Title: "T", Slug: "t", Kind: "page", Path: "t", ContentHash: "c2", AssetManifestHash: "a2", Summary: "edit"},
				{ID: "rev-1", PageID: "page-1", Type: revision.RevisionTypeContentUpdate, AuthorID: "u1", CreatedAt: created.Add(-time.Hour), Title: "T", Slug: "t", Kind: "page", Path: "t", ContentHash: "c1", AssetManifestHash: "a1"},
			},
			NextCursor: "abc",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(toRevisionListResponse(tt.out, nil))
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			// The slice handleListRevisions built before RevisionListResponse existed.
			result := make([]*RevisionResponse, 0, len(tt.out.Revisions))
			for _, rev := range tt.out.Revisions {
				result = append(result, toRevisionResponse(rev, nil))
			}
			legacy, err := json.Marshal(gin.H{"revisions": result, "nextCursor": tt.out.NextCursor})
			if err != nil {
				t.Fatalf("Marshal legacy map: %v", err)
			}

			if !bytes.Equal(got, legacy) {
				t.Errorf("named model = %s, differs from legacy map %s", got, legacy)
			}
		})
	}
}
