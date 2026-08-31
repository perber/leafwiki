package pages

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/draft"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/http/dto"
	"github.com/perber/wiki/internal/http/middleware/auth"
)

type draftResponse struct {
	Page        *dto.Page `json:"page"`
	BaseVersion string    `json:"baseVersion"`
}

type pendingDraftResponse struct {
	Page     *dto.Page `json:"page"`
	Pending  bool      `json:"pending"`
	ParentID string    `json:"parentId"`
}

type draftSummaryResponse struct {
	Drafts  []draftSummary        `json:"drafts"`
	Pending []pendingDraftSummary `json:"pending"`
}

type draftSummary struct {
	PageID string `json:"pageId"`
}

type pendingDraftSummary struct {
	ID       string `json:"id"`
	ParentID string `json:"parentId"`
	Title    string `json:"title"`
	Slug     string `json:"slug"`
}

func (r *Routes) handleListDrafts(c *gin.Context) {
	active := r.drafts.List()
	pending := r.drafts.ListPending()
	response := draftSummaryResponse{
		Drafts:  make([]draftSummary, 0, len(active)),
		Pending: make([]pendingDraftSummary, 0, len(pending)),
	}
	for _, d := range active {
		if _, err := r.treeService.GetPage(d.PageID); err == nil {
			response.Drafts = append(response.Drafts, draftSummary{PageID: d.PageID})
		}
	}
	for _, d := range pending {
		response.Pending = append(response.Pending, pendingDraftSummary{ID: d.ID, ParentID: d.ParentID, Title: d.Title, Slug: d.Slug})
	}
	c.JSON(http.StatusOK, response)
}

func (r *Routes) pendingResponse(d *draft.PendingDraft) pendingDraftResponse {
	path := d.Slug
	if d.ParentID != "" && d.ParentID != "root" {
		if parent, err := r.treeService.GetPage(d.ParentID); err == nil && parent != nil {
			path = dto.BuildPathFromNode(parent.PageNode) + "/" + d.Slug
		}
	}
	return pendingDraftResponse{Page: &dto.Page{Node: &dto.Node{ID: d.ID, Title: d.Title, Slug: d.Slug, Path: path, Version: "pending", Kind: tree.NodeKindPage, Children: []*dto.Node{}}, Content: d.Content, Path: path, Tags: d.Tags, Properties: d.Properties}, Pending: true, ParentID: d.ParentID}
}

func (r *Routes) handleCreatePendingDraft(c *gin.Context) {
	var req struct {
		ParentID *string `json:"parentId"`
		Title    string  `json:"title" binding:"required"`
		Slug     string  `json:"slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
		return
	}
	parentID := ""
	if req.ParentID != nil {
		parentID = strings.TrimSpace(*req.ParentID)
	}
	if parentID == "root" {
		parentID = ""
	}
	if strings.TrimSpace(req.Title) == "" {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageMissingTitle, "Title is required", "title is required")
		return
	}
	if err := tree.NewSlugService().IsValidSlug(req.Slug); err != nil {
		respondWithPageError(c, err)
		return
	}
	if parentID != "" {
		if _, err := r.treeService.FindPageByID(parentID); err != nil {
			respondWithPageError(c, err)
			return
		}
	}
	targetPath := req.Slug
	if parentID != "" {
		parent, err := r.treeService.GetPage(parentID)
		if err != nil {
			respondWithPageError(c, err)
			return
		}
		targetPath = dto.BuildPathFromNode(parent.PageNode) + "/" + req.Slug
	}
	if _, err := r.treeService.FindPageByRoutePath(targetPath); err == nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageSlugConflict, "Page already exists", "page already exists")
		return
	} else if !errors.Is(err, tree.ErrPageNotFound) {
		respondWithPageError(c, err)
		return
	}
	d, err := r.drafts.CreatePending(parentID, req.Title, req.Slug)
	if err != nil {
		respondWithDraftError(c, err)
		return
	}
	c.JSON(http.StatusCreated, r.pendingResponse(d))
}

func (r *Routes) handleGetPendingDraft(c *gin.Context) {
	d, err := r.drafts.GetPending(strings.TrimSpace(c.Param("id")))
	if err != nil {
		respondWithDraftError(c, err)
		return
	}
	c.JSON(http.StatusOK, r.pendingResponse(d))
}

func (r *Routes) handleSavePendingDraft(c *gin.Context) {
	var req struct {
		Title      string            `json:"title" binding:"required"`
		Slug       string            `json:"slug" binding:"required"`
		Content    string            `json:"content"`
		Tags       []string          `json:"tags"`
		Properties map[string]string `json:"properties"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
		return
	}
	if err := validatePageMetadataInput(req.Tags, req.Properties); err != nil {
		respondWithPageError(c, err)
		return
	}
	if err := tree.NewSlugService().IsValidSlug(req.Slug); err != nil {
		respondWithPageError(c, err)
		return
	}
	d, err := r.drafts.GetPending(strings.TrimSpace(c.Param("id")))
	if err != nil {
		respondWithDraftError(c, err)
		return
	}
	if conflict, err := r.pendingDraftPathConflict(d.ID, d.ParentID, req.Slug); err != nil {
		respondWithPageError(c, err)
		return
	} else if conflict {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageSlugConflict, "Page already exists", "page already exists")
		return
	}
	d.Title, d.Slug, d.Content, d.Tags = req.Title, req.Slug, req.Content, normalizeTagInputs(req.Tags)
	if req.Properties == nil {
		d.Properties = map[string]string{}
	} else {
		d.Properties = req.Properties
	}
	if err := r.drafts.SavePending(*d); err != nil {
		respondWithDraftError(c, err)
		return
	}
	c.JSON(http.StatusOK, r.pendingResponse(d))
}

func (r *Routes) pendingDraftPathConflict(excludeID, parentID, slug string) (bool, error) {
	path := slug
	if parentID != "" {
		parent, err := r.treeService.GetPage(parentID)
		if err == nil {
			path = dto.BuildPathFromNode(parent.PageNode) + "/" + slug
		} else if !errors.Is(err, tree.ErrPageNotFound) {
			return false, err
		}
	}
	if _, err := r.treeService.FindPageByRoutePath(path); err == nil {
		return true, nil
	} else if !errors.Is(err, tree.ErrPageNotFound) {
		return false, err
	}
	for _, pending := range r.drafts.ListPending() {
		if pending.ID != excludeID && pending.ParentID == parentID && pending.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

func (r *Routes) handlePublishPendingDraft(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	d, err := r.drafts.GetPending(id)
	if err != nil {
		respondWithDraftError(c, err)
		return
	}
	if d.ParentID != "" && !r.ensureNoDraft(c, d.ParentID) {
		return
	}
	user := auth.MustGetUser(c)
	if user == nil {
		return
	}
	kind, content := tree.NodeKindPage, d.Content
	parentID := d.ParentID
	out, err := r.createPage.Execute(c.Request.Context(), CreatePageInput{UserID: user.ID, ParentID: &parentID, Title: d.Title, Slug: d.Slug, Kind: &kind, Content: &content, Tags: d.Tags, Properties: d.Properties})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	if err := r.drafts.DeletePending(id); err != nil {
		respondWithDraftError(c, err)
		return
	}
	r.respondPage(c, http.StatusOK, out.Page)
}

func (r *Routes) handleDiscardPendingDraft(c *gin.Context) {
	if err := r.drafts.DeletePending(strings.TrimSpace(c.Param("id"))); err != nil {
		respondWithDraftError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Routes) handleStartDraft(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	page, err := r.treeService.GetPage(id)
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	if page.Kind != tree.NodeKindPage || page.HasChildren() {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodeDraftBlocked, "Drafts are only available for existing leaf pages", "drafts are only available for existing leaf pages")
		return
	}
	exists, err := r.drafts.Exists(id)
	if err != nil {
		respondWithDraftError(c, err)
		return
	}
	if exists {
		respondWithPageStatusError(c, http.StatusConflict, ErrCodeDraftExists, "A draft already exists", "a draft already exists")
		return
	}
	apiPage := r.apiPage(page)
	d := draft.Draft{PageID: id, BaseVersion: page.Version(), Title: page.Title, Content: page.Content, Tags: apiPage.Tags, Properties: apiPage.Properties}
	if err := r.drafts.Save(d); err != nil {
		respondWithDraftError(c, err)
		return
	}
	c.JSON(http.StatusCreated, r.draftResponse(page, d))
}

func (r *Routes) handleGetDraft(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	page, err := r.treeService.GetPage(id)
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	d, err := r.drafts.Get(id)
	if err != nil {
		respondWithDraftError(c, err)
		return
	}
	c.JSON(http.StatusOK, r.draftResponse(page, *d))
}

func (r *Routes) handleSaveDraft(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Title      string            `json:"title" binding:"required"`
		Content    string            `json:"content"`
		Tags       []string          `json:"tags"`
		Properties map[string]string `json:"properties"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
		return
	}
	if err := validatePageMetadataInput(req.Tags, req.Properties); err != nil {
		respondWithPageError(c, err)
		return
	}
	d, err := r.drafts.Get(id)
	if err != nil {
		respondWithDraftError(c, err)
		return
	}
	d.Title = req.Title
	d.Content = req.Content
	d.Tags = normalizeTagInputs(req.Tags)
	if req.Properties == nil {
		d.Properties = map[string]string{}
	} else {
		d.Properties = req.Properties
	}
	if err := r.drafts.Save(*d); err != nil {
		respondWithDraftError(c, err)
		return
	}
	page, err := r.treeService.GetPage(id)
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, r.draftResponse(page, *d))
}

func (r *Routes) handlePublishDraft(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	d, err := r.drafts.Get(id)
	if err != nil {
		respondWithDraftError(c, err)
		return
	}
	page, err := r.treeService.GetPage(id)
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	if page.Kind != tree.NodeKindPage || page.HasChildren() {
		respondWithPageStatusError(c, http.StatusConflict, ErrCodeDraftConflict, "The page changed and the draft cannot be published", "the page changed and the draft cannot be published")
		return
	}
	if page.Version() != d.BaseVersion {
		respondWithPageStatusError(c, http.StatusConflict, ErrCodeDraftConflict, "The published page changed while this draft was open", "the published page changed while this draft was open")
		return
	}
	user := auth.MustGetUser(c)
	if user == nil {
		return
	}
	kind := tree.NodeKindPage
	out, err := r.updatePage.Execute(c.Request.Context(), UpdatePageInput{
		UserID: user.ID, ID: id, Version: page.Version(), Title: d.Title, Slug: page.Slug,
		Content: &d.Content, Kind: &kind, Tags: d.Tags, Properties: d.Properties,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	if err := r.drafts.Delete(id); err != nil {
		// The canonical write succeeded. Preserve the draft for explicit recovery
		// or discard rather than silently losing it when cleanup fails.
		respondWithDraftError(c, err)
		return
	}
	r.respondPage(c, http.StatusOK, out.Page)
}

func (r *Routes) handleDiscardDraft(c *gin.Context) {
	if err := r.drafts.Delete(strings.TrimSpace(c.Param("id"))); err != nil {
		respondWithDraftError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Routes) draftResponse(page *tree.Page, d draft.Draft) draftResponse {
	apiPage := r.apiPage(page)
	apiPage.Title = d.Title
	apiPage.Content = d.Content
	apiPage.Tags = d.Tags
	apiPage.Properties = d.Properties
	return draftResponse{Page: apiPage, BaseVersion: d.BaseVersion}
}

func (r *Routes) apiPage(page *tree.Page) *dto.Page {
	apiPage := dto.ToAPIPage(page, r.userResolver)
	r.enrichPageMetadata(apiPage)
	return apiPage
}

func respondWithDraftError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, draft.ErrNotFound):
		respondWithPageStatusError(c, http.StatusNotFound, ErrCodeDraftNotFound, "Draft not found", "draft not found")
	case errors.Is(err, draft.ErrInvalidPageID), errors.Is(err, draft.ErrCorrupt):
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid draft data", "invalid draft data")
	default:
		respondWithPageStatusError(c, http.StatusInternalServerError, ErrCodePageInternalError, "Draft storage failed", "draft storage failed")
	}
}

func (r *Routes) ensureNoDraft(c *gin.Context, ids ...string) bool {
	for _, id := range ids {
		exists, err := r.drafts.Exists(id)
		if err != nil {
			respondWithDraftError(c, err)
			return false
		}
		if exists {
			respondWithPageStatusError(c, http.StatusConflict, ErrCodeDraftBlocked, "Discard or publish the draft before changing this page", "discard or publish the draft before changing this page")
			return false
		}
	}
	return true
}
