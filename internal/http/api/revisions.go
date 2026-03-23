package api

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/wiki"
)

type RevisionResponse struct {
	ID                string          `json:"id"`
	PageID            string          `json:"pageId"`
	Type              string          `json:"type"`
	AuthorID          string          `json:"authorId"`
	Author            *auth.UserLabel `json:"author,omitempty"`
	CreatedAt         string          `json:"createdAt"`
	Title             string          `json:"title"`
	Slug              string          `json:"slug"`
	Kind              string          `json:"kind"`
	Path              string          `json:"path"`
	ContentHash       string          `json:"contentHash"`
	AssetManifestHash string          `json:"assetManifestHash"`
	Summary           string          `json:"summary,omitempty"`
}

type TrashEntryResponse struct {
	PageID         string          `json:"pageId"`
	DeletedAt      string          `json:"deletedAt"`
	DeletedBy      string          `json:"deletedBy"`
	DeletedByUser  *auth.UserLabel `json:"deletedByUser,omitempty"`
	Title          string          `json:"title"`
	Slug           string          `json:"slug"`
	Path           string          `json:"path"`
	LastRevisionID string          `json:"lastRevisionId"`
}

func formatAPITime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}

func ToAPIRevision(rev *revision.Revision, userResolver *auth.UserResolver) *RevisionResponse {
	if rev == nil {
		return nil
	}

	var author *auth.UserLabel
	if userResolver != nil {
		author, _ = userResolver.ResolveUserLabel(rev.AuthorID)
	}

	return &RevisionResponse{
		ID:                rev.ID,
		PageID:            rev.PageID,
		Type:              string(rev.Type),
		AuthorID:          rev.AuthorID,
		Author:            author,
		CreatedAt:         formatAPITime(rev.CreatedAt),
		Title:             rev.Title,
		Slug:              rev.Slug,
		Kind:              rev.Kind,
		Path:              rev.Path,
		ContentHash:       rev.ContentHash,
		AssetManifestHash: rev.AssetManifestHash,
		Summary:           rev.Summary,
	}
}

func ToAPITrashEntry(entry *revision.TrashEntry, userResolver *auth.UserResolver) *TrashEntryResponse {
	if entry == nil {
		return nil
	}

	var deletedByUser *auth.UserLabel
	if userResolver != nil {
		deletedByUser, _ = userResolver.ResolveUserLabel(entry.DeletedBy)
	}

	return &TrashEntryResponse{
		PageID:         entry.PageID,
		DeletedAt:      formatAPITime(entry.DeletedAt),
		DeletedBy:      entry.DeletedBy,
		DeletedByUser:  deletedByUser,
		Title:          entry.Title,
		Slug:           entry.Slug,
		Path:           entry.Path,
		LastRevisionID: entry.LastRevisionID,
	}
}

func ListPageRevisionsHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		if pageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing page ID"})
			return
		}

		revisions, err := w.ListRevisions(pageID)
		if err != nil {
			respondWithError(c, err)
			return
		}

		resolver := w.GetUserResolver()
		out := make([]*RevisionResponse, 0, len(revisions))
		for _, rev := range revisions {
			out = append(out, ToAPIRevision(rev, resolver))
		}

		c.JSON(http.StatusOK, gin.H{
			"revisions": out,
		})
	}
}

func GetLatestPageRevisionHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		if pageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing page ID"})
			return
		}

		rev, err := w.GetLatestRevision(pageID)
		if err != nil {
			respondWithError(c, err)
			return
		}
		if rev == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "revision not found"})
			return
		}

		c.JSON(http.StatusOK, ToAPIRevision(rev, w.GetUserResolver()))
	}
}

func ListTrashHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		trash, err := w.ListTrash()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		resolver := w.GetUserResolver()
		out := make([]*TrashEntryResponse, 0, len(trash))
		for _, entry := range trash {
			out = append(out, ToAPITrashEntry(entry, resolver))
		}

		c.JSON(http.StatusOK, gin.H{
			"trash": out,
		})
	}
}

func GetTrashEntryHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		if pageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing page ID"})
			return
		}

		entry, err := w.GetTrashEntry(pageID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "trash entry not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if entry == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "trash entry not found"})
			return
		}

		c.JSON(http.StatusOK, ToAPITrashEntry(entry, w.GetUserResolver()))
	}
}
