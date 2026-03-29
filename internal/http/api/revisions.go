package api

import (
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/revision"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

type RevisionResponse struct {
	ID                string          `json:"id"`
	PageID            string          `json:"pageId"`
	ParentID          string          `json:"parentId,omitempty"`
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
	PageCreatedAt     string          `json:"pageCreatedAt,omitempty"`
	PageUpdatedAt     string          `json:"pageUpdatedAt,omitempty"`
	CreatorID         string          `json:"creatorId,omitempty"`
	LastAuthorID      string          `json:"lastAuthorId,omitempty"`
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

type RestorePageRequest struct {
	TargetParentID *string `json:"targetParentId"`
}

type RevisionAssetResponse struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
	MIMEType  string `json:"mimeType,omitempty"`
}

type RevisionSnapshotResponse struct {
	Revision *RevisionResponse       `json:"revision"`
	Content  string                  `json:"content"`
	Assets   []RevisionAssetResponse `json:"assets"`
}

type RevisionAssetDeltaResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type RevisionComparisonResponse struct {
	Base           *RevisionSnapshotResponse    `json:"base"`
	Target         *RevisionSnapshotResponse    `json:"target"`
	ContentChanged bool                         `json:"contentChanged"`
	AssetChanges   []RevisionAssetDeltaResponse `json:"assetChanges"`
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
		ParentID:          rev.ParentID,
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
		PageCreatedAt:     formatAPITime(rev.PageCreatedAt),
		PageUpdatedAt:     formatAPITime(rev.PageUpdatedAt),
		CreatorID:         rev.CreatorID,
		LastAuthorID:      rev.LastAuthorID,
		Summary:           rev.Summary,
	}
}

func ToAPIRevisionSnapshot(snapshot *revision.RevisionSnapshot, userResolver *auth.UserResolver) *RevisionSnapshotResponse {
	if snapshot == nil {
		return nil
	}
	assets := make([]RevisionAssetResponse, 0, len(snapshot.Assets))
	for _, asset := range snapshot.Assets {
		assets = append(assets, RevisionAssetResponse{Name: asset.Name, SHA256: asset.SHA256, SizeBytes: asset.SizeBytes, MIMEType: asset.MIMEType})
	}
	return &RevisionSnapshotResponse{
		Revision: ToAPIRevision(snapshot.Revision, userResolver),
		Content:  snapshot.Content,
		Assets:   assets,
	}
}

func ToAPIRevisionComparison(comparison *revision.RevisionComparison, userResolver *auth.UserResolver) *RevisionComparisonResponse {
	if comparison == nil {
		return nil
	}
	assetChanges := make([]RevisionAssetDeltaResponse, 0, len(comparison.AssetChanges))
	for _, change := range comparison.AssetChanges {
		assetChanges = append(assetChanges, RevisionAssetDeltaResponse{Name: change.Name, Status: change.Status})
	}
	return &RevisionComparisonResponse{
		Base:           ToAPIRevisionSnapshot(comparison.Base, userResolver),
		Target:         ToAPIRevisionSnapshot(comparison.Target, userResolver),
		ContentChanged: comparison.ContentChanged,
		AssetChanges:   assetChanges,
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
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_page_id", "Page ID is required", "page id is required")
			return
		}

		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed <= 0 || parsed > 200 {
				respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_limit", "Revision list limit is invalid", "revision list limit for page %s is invalid", pageID)
				return
			}
			limit = parsed
		}

		revisions, nextCursor, err := w.ListRevisionsPage(pageID, strings.TrimSpace(c.Query("cursor")), limit)
		if err != nil {
			respondWithRevisionError(c, err)
			return
		}

		resolver := w.GetUserResolver()
		out := make([]*RevisionResponse, 0, len(revisions))
		for _, rev := range revisions {
			out = append(out, ToAPIRevision(rev, resolver))
		}

		c.JSON(http.StatusOK, gin.H{
			"revisions":  out,
			"nextCursor": nextCursor,
		})
	}
}

func ComparePageRevisionsHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		baseRevisionID := strings.TrimSpace(c.Query("base"))
		targetRevisionID := strings.TrimSpace(c.Query("target"))
		if pageID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_page_id", "Page ID is required", "page id is required")
			return
		}
		if baseRevisionID == "" || targetRevisionID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_compare_invalid_request", "Revision compare request is invalid", "revision compare request for page %s is invalid", pageID)
			return
		}

		comparison, err := w.CompareRevisionSnapshots(pageID, baseRevisionID, targetRevisionID)
		if err != nil {
			respondWithRevisionError(c, err)
			return
		}
		if comparison == nil || comparison.Base == nil || comparison.Target == nil {
			respondWithRevisionStatusError(c, http.StatusNotFound, "revision_not_found", "Revision not found", "revision compare resource for page %s not found", pageID)
			return
		}

		c.JSON(http.StatusOK, ToAPIRevisionComparison(comparison, w.GetUserResolver()))
	}
}

func GetPageRevisionHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		revisionID := strings.TrimSpace(c.Param("revisionId"))
		if pageID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_page_id", "Page ID is required", "page id is required")
			return
		}
		if revisionID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_revision_id", "Revision ID is required", "revision id is required")
			return
		}

		snapshot, err := w.GetRevisionSnapshot(pageID, revisionID)
		if err != nil {
			respondWithRevisionError(c, err)
			return
		}
		if snapshot == nil || snapshot.Revision == nil {
			respondWithRevisionStatusError(c, http.StatusNotFound, "revision_not_found", "Revision not found", "revision %s for page %s not found", revisionID, pageID)
			return
		}

		c.JSON(http.StatusOK, ToAPIRevisionSnapshot(snapshot, w.GetUserResolver()))
	}
}

func GetPageRevisionAssetHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		revisionID := strings.TrimSpace(c.Param("revisionId"))
		assetName := strings.TrimSpace(strings.TrimPrefix(c.Param("name"), "/"))
		if pageID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_page_id", "Page ID is required", "page id is required")
			return
		}
		if revisionID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_revision_id", "Revision ID is required", "revision id is required")
			return
		}
		if assetName == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_preview_asset_invalid_name", "Revision asset name is invalid", "revision asset name for page %s revision %s is invalid", pageID, revisionID)
			return
		}

		asset, err := w.GetRevisionAsset(pageID, revisionID, assetName)
		if err != nil {
			respondWithRevisionError(c, err)
			return
		}
		if asset == nil {
			respondWithRevisionStatusError(c, http.StatusNotFound, "revision_preview_asset_not_found", "Revision asset not found", "revision asset %s for page %s revision %s not found", assetName, pageID, revisionID)
			return
		}

		contentType := asset.Asset.MIMEType
		if contentType == "" {
			contentType = http.DetectContentType(asset.Content)
		}
		c.Header("Content-Disposition", `inline; filename="`+path.Base(assetName)+`"`)
		c.Data(http.StatusOK, contentType, asset.Content)
	}
}

func GetLatestPageRevisionHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		if pageID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_page_id", "Page ID is required", "page id is required")
			return
		}

		rev, err := w.GetLatestRevision(pageID)
		if err != nil {
			respondWithRevisionError(c, err)
			return
		}
		if rev == nil {
			respondWithRevisionStatusError(c, http.StatusNotFound, "revision_not_found", "Revision not found", "revision for page %s not found", pageID)
			return
		}

		c.JSON(http.StatusOK, ToAPIRevision(rev, w.GetUserResolver()))
	}
}

func RestorePageRevisionHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		revisionID := strings.TrimSpace(c.Param("revisionId"))
		if pageID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_restore_invalid_page_id", "Failed to restore page", "failed to restore page %s", pageID)
			return
		}
		if revisionID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_restore_invalid_revision", "Restore revision is invalid", "restore revision %s for page %s is invalid", revisionID, pageID)
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		page, err := w.RestoreRevision(user.ID, pageID, revisionID)
		if err != nil {
			respondWithRevisionError(c, err)
			return
		}

		c.JSON(http.StatusOK, ToAPIPage(page, w.GetUserResolver()))
	}
}

func ListTrashHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		trash, err := w.ListTrash()
		if err != nil {
			respondWithRevisionError(c, err)
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
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_invalid_page_id", "Page ID is required", "page id is required")
			return
		}

		entry, err := w.GetTrashEntry(pageID)
		if err != nil {
			respondWithRevisionError(c, err)
			return
		}
		if entry == nil {
			respondWithRevisionStatusError(c, http.StatusNotFound, "revision_trash_not_found", "Trash entry not found", "trash entry for page %s not found", pageID)
			return
		}

		c.JSON(http.StatusOK, ToAPITrashEntry(entry, w.GetUserResolver()))
	}
}

func RestorePageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := strings.TrimSpace(c.Param("id"))
		if pageID == "" {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_restore_invalid_page_id", "Failed to restore page", "failed to restore page %s", pageID)
			return
		}

		var req RestorePageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondWithRevisionStatusError(c, http.StatusBadRequest, "revision_restore_invalid_request", "Restore request payload is invalid", "restore request payload for page %s is invalid", pageID)
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		page, err := w.RestorePage(user.ID, pageID, req.TargetParentID)
		if err != nil {
			respondWithRevisionError(c, err)
			return
		}

		c.JSON(http.StatusOK, ToAPIPage(page, w.GetUserResolver()))
	}
}
