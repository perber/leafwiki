package pages

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/markdown"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/http/dto"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the pages domain.
type Routes struct {
	treeService      *tree.TreeService
	createPage       *CreatePageUseCase
	updatePage       *UpdatePageUseCase
	deletePage       *DeletePageUseCase
	movePage         *MovePageUseCase
	convertPage      *ConvertPageUseCase
	copyPage         *CopyPageUseCase
	getPage          *GetPageUseCase
	findByPath       *FindByPathUseCase
	lookupPath       *LookupPagePathUseCase
	resolvePermalink *ResolvePermalinkUseCase
	sortPages        *SortPagesUseCase
	ensurePath       *EnsurePathUseCase
	suggestSlug      *SuggestSlugUseCase
	previewRefactor  *PreviewPageRefactorUseCase
	applyRefactor    *ApplyPageRefactorUseCase
	userResolver     *coreauth.UserResolver
	authService      *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	TreeService      *tree.TreeService
	CreatePage       *CreatePageUseCase
	UpdatePage       *UpdatePageUseCase
	DeletePage       *DeletePageUseCase
	MovePage         *MovePageUseCase
	ConvertPage      *ConvertPageUseCase
	CopyPage         *CopyPageUseCase
	GetPage          *GetPageUseCase
	FindByPath       *FindByPathUseCase
	LookupPath       *LookupPagePathUseCase
	ResolvePermalink *ResolvePermalinkUseCase
	SortPages        *SortPagesUseCase
	EnsurePath       *EnsurePathUseCase
	SuggestSlug      *SuggestSlugUseCase
	PreviewRefactor  *PreviewPageRefactorUseCase
	ApplyRefactor    *ApplyPageRefactorUseCase
	UserResolver     *coreauth.UserResolver
	AuthService      *coreauth.AuthService
}

// NewRoutes constructs the pages RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		treeService:      cfg.TreeService,
		createPage:       cfg.CreatePage,
		updatePage:       cfg.UpdatePage,
		deletePage:       cfg.DeletePage,
		movePage:         cfg.MovePage,
		convertPage:      cfg.ConvertPage,
		copyPage:         cfg.CopyPage,
		getPage:          cfg.GetPage,
		findByPath:       cfg.FindByPath,
		lookupPath:       cfg.LookupPath,
		resolvePermalink: cfg.ResolvePermalink,
		sortPages:        cfg.SortPages,
		ensurePath:       cfg.EnsurePath,
		suggestSlug:      cfg.SuggestSlug,
		previewRefactor:  cfg.PreviewRefactor,
		applyRefactor:    cfg.ApplyRefactor,
		userResolver:     cfg.UserResolver,
		authService:      cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	if opts.PublicAccess {
		pub := ctx.Base.Group("/api")
		pub.GET("/tree", r.handleGetTree)
		pub.GET("/pages/by-path", r.handleGetByPath)
		pub.GET("/pages/lookup", r.handleLookupPath)
		pub.GET("/pages/permalink/:id", r.handleResolvePermalink)
		pub.GET("/pages/:id", r.handleGetPage)
	}

	authGroup := ctx.Base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)

	if !opts.PublicAccess {
		authGroup.GET("/tree", r.handleGetTree)
		authGroup.GET("/pages/:id", r.handleGetPage)
		authGroup.GET("/pages/lookup", r.handleLookupPath)
		authGroup.GET("/pages/by-path", r.handleGetByPath)
		authGroup.GET("/pages/permalink/:id", r.handleResolvePermalink)
	}

	authGroup.GET("/pages/slug-suggestion", authmw.RequireEditorOrAdmin(), r.handleSuggestSlug)
	authGroup.POST("/pages", authmw.RequireEditorOrAdmin(), r.handleCreate)
	authGroup.PUT("/pages/:id", authmw.RequireEditorOrAdmin(), r.handleUpdate)
	authGroup.DELETE("/pages/:id", authmw.RequireEditorOrAdmin(), r.handleDelete)
	authGroup.PUT("/pages/:id/move", authmw.RequireEditorOrAdmin(), r.handleMove)
	authGroup.PUT("/pages/:id/sort", authmw.RequireEditorOrAdmin(), r.handleSort)
	authGroup.POST("/pages/ensure", authmw.RequireEditorOrAdmin(), r.handleEnsurePath)
	authGroup.POST("/pages/convert/:id", authmw.RequireEditorOrAdmin(), r.handleConvert)
	authGroup.POST("/pages/copy/:id", authmw.RequireEditorOrAdmin(), r.handleCopy)
	if opts.EnableLinkRefactor {
		authGroup.POST("/pages/:id/refactor/preview", authmw.RequireEditorOrAdmin(), r.handleRefactorPreview)
		authGroup.POST("/pages/:id/refactor/apply", authmw.RequireEditorOrAdmin(), r.handleRefactorApply)
	}
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleGetTree(c *gin.Context) {
	root := r.treeService.GetTree()
	depthStr := strings.TrimSpace(c.Query("depth"))
	if depthStr == "" {
		c.JSON(http.StatusOK, dto.ToAPINode(root, "", r.userResolver))
		return
	}
	depth, err := strconv.Atoi(depthStr)
	if err != nil {
		depth = -1
	}
	c.JSON(http.StatusOK, dto.ToAPINodeWithDepth(root, "", r.userResolver, depth))
}

func (r *Routes) handleGetPage(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	out, err := r.getPage.Execute(c.Request.Context(), GetPageInput{ID: id})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	r.respondPage(c, http.StatusOK, out.Page)
}

func (r *Routes) handleGetByPath(c *gin.Context) {
	routePath, err := ValidatePageRoutePath(c.Query("path"))
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	out, err := r.findByPath.Execute(c.Request.Context(), FindByPathInput{RoutePath: routePath})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	depth := 0
	if out.Page.Kind == tree.NodeKindSection {
		depth = 1
	}
	r.respondPageWithDepth(c, http.StatusOK, out.Page, depth)
}

func (r *Routes) handleLookupPath(c *gin.Context) {
	path := strings.TrimSpace(c.Query("path"))
	out, err := r.lookupPath.Execute(c.Request.Context(), LookupPagePathInput{Path: path})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Lookup)
}

func (r *Routes) handleResolvePermalink(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageMissingID, "Page ID is required", "page id is required")
		return
	}
	out, err := r.resolvePermalink.Execute(c.Request.Context(), ResolvePermalinkInput{ID: id})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Target)
}

func (r *Routes) handleSuggestSlug(c *gin.Context) {
	title, err := ValidateSuggestSlugTitle(c.Query("title"))
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	out, err := r.suggestSlug.Execute(c.Request.Context(), SuggestSlugInput{
		ParentID:  strings.TrimSpace(c.Query("parentId")),
		CurrentID: strings.TrimSpace(c.Query("currentId")),
		Title:     title,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"slug": out.Slug})
}

func ValidateSuggestSlugTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", sharederrors.NewLocalizedError(
			ErrCodePageMissingTitle,
			"Title query param is required",
			"title query param is required",
			nil,
		)
	}
	if tree.NewSlugService().GenerateValidSlug(title) == "" {
		return "", sharederrors.NewLocalizedError(
			ErrCodePageInvalidTitle,
			"Title must include at least one slug character",
			"title must include at least one slug character",
			nil,
		)
	}
	return title, nil
}

func (r *Routes) handleCreate(c *gin.Context) {
	var req struct {
		ParentID *string `json:"parentId"`
		Title    string  `json:"title" binding:"required"`
		Slug     string  `json:"slug" binding:"required"`
		Kind     *string `json:"kind"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	kind, err := ValidatePageKind(req.Kind)
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	out, err := r.createPage.Execute(c.Request.Context(), CreatePageInput{
		UserID: user.ID, ParentID: req.ParentID, Title: req.Title, Slug: req.Slug, Kind: &kind,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	r.respondPage(c, http.StatusCreated, out.Page)
}

func (r *Routes) handleUpdate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Version    string            `json:"version" binding:"required"`
		Title      string            `json:"title" binding:"required"`
		Slug       string            `json:"slug" binding:"required"`
		Content    *string           `json:"content"`
		Tags       []string          `json:"tags"`
		Properties map[string]string `json:"properties"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	if err := ValidatePageMetadataInput(req.Tags, req.Properties); err != nil {
		respondWithPageError(c, err)
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}

	contentToSave := req.Content
	fromImport := false
	if req.Content != nil {
		extraFields := BuildExtraFields(req.Tags, req.Properties)
		combined, err := markdown.BuildMarkdownWithExtraFrontmatter(extraFields, *req.Content)
		if err != nil {
			respondWithPageStatusError(c, http.StatusInternalServerError, ErrCodePageInternalError, "Failed to build frontmatter", "failed to build frontmatter")
			return
		}
		contentToSave = &combined
		fromImport = true
	}

	kind := tree.NodeKindPage
	out, err := r.updatePage.Execute(c.Request.Context(), UpdatePageInput{
		UserID: user.ID, ID: id, Version: req.Version, Title: req.Title, Slug: req.Slug,
		Content: contentToSave, Kind: &kind, FromImport: fromImport,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	r.respondPage(c, http.StatusOK, out.Page)
}

func BuildExtraFields(tags []string, properties map[string]string) map[string]interface{} {
	extra := make(map[string]interface{}, len(properties)+1)
	for k, v := range properties {
		extra[k] = v
	}
	normalizedTags := normalizeTagInputs(tags)
	list := make([]interface{}, len(normalizedTags))
	for i, t := range normalizedTags {
		list[i] = t
	}
	extra["tags"] = list
	return extra
}

func (r *Routes) handleDelete(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	recursive := c.DefaultQuery("recursive", "false") == "true"
	version := c.Query("version")
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	if err := r.deletePage.Execute(c.Request.Context(), DeletePageInput{
		UserID: user.ID, ID: id, Version: version, Recursive: recursive,
	}); err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Page deleted"})
}

func (r *Routes) handleMove(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Version  string `json:"version" binding:"required"`
		ParentID string `json:"parentId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidPayload, "Invalid payload", "invalid payload")
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	if err := r.movePage.Execute(c.Request.Context(), MovePageInput{
		UserID: user.ID, ID: id, Version: req.Version, ParentID: req.ParentID,
	}); err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Page moved"})
}

func (r *Routes) handleSort(c *gin.Context) {
	parentID := strings.TrimSpace(c.Param("id"))
	var req struct {
		OrderedIDs []string `json:"orderedIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	if err := r.sortPages.Execute(c.Request.Context(), SortPagesInput{
		ParentID: parentID, OrderedIDs: req.OrderedIDs,
	}); err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pages sorted successfully"})
}

func (r *Routes) handleEnsurePath(c *gin.Context) {
	var req struct {
		Path  string  `json:"path" binding:"required"`
		Title string  `json:"title" binding:"required"`
		Kind  *string `json:"kind"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	kind, err := ValidatePageKind(req.Kind)
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	out, err := r.ensurePath.Execute(c.Request.Context(), EnsurePathInput{
		UserID: user.ID, TargetPath: req.Path, TargetTitle: req.Title, Kind: &kind,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	r.respondPage(c, http.StatusOK, out.Page)
}

func (r *Routes) handleConvert(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Kind    string `json:"targetKind" binding:"required"`
		Version string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	targetKind, err := ValidateConvertTargetKind(req.Kind)
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	if err := r.convertPage.Execute(c.Request.Context(), ConvertPageInput{
		UserID: user.ID, ID: id, Version: req.Version, TargetKind: targetKind,
	}); err != nil {
		respondWithPageError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func ValidateConvertTargetKind(kind string) (tree.NodeKind, error) {
	if kind != string(tree.NodeKindPage) && kind != string(tree.NodeKindSection) {
		return "", sharederrors.NewLocalizedError(
			ErrCodePageInvalidTargetKind,
			"Invalid targetKind",
			"invalid target kind",
			nil,
		)
	}
	return tree.NodeKind(kind), nil
}

func (r *Routes) handleCopy(c *gin.Context) {
	sourceID := strings.TrimSpace(c.Param("id"))
	var req struct {
		ParentID *string `json:"targetParentId"`
		Title    string  `json:"title" binding:"required"`
		Slug     string  `json:"slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	out, err := r.copyPage.Execute(c.Request.Context(), CopyPageInput{
		UserID: user.ID, SourcePageID: sourceID, TargetParentID: req.ParentID,
		Title: req.Title, Slug: req.Slug,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	r.respondPage(c, http.StatusCreated, out.Page)
}

func (r *Routes) handleRefactorPreview(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Kind        string  `json:"kind" binding:"required"`
		Title       string  `json:"title"`
		Slug        string  `json:"slug"`
		Content     *string `json:"content"`
		NewParentID *string `json:"parentId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	out, err := r.previewRefactor.Execute(c.Request.Context(), RefactorPreviewInput{
		PageID: id, Kind: req.Kind, Title: req.Title, Slug: req.Slug,
		Content: req.Content, NewParentID: req.NewParentID,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (r *Routes) handleRefactorApply(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Version      string  `json:"version" binding:"required"`
		Kind         string  `json:"kind" binding:"required"`
		Title        string  `json:"title"`
		Slug         string  `json:"slug"`
		Content      *string `json:"content"`
		NewParentID  *string `json:"parentId"`
		RewriteLinks bool    `json:"rewriteLinks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	page, err := r.applyRefactor.Execute(c.Request.Context(), RefactorApplyInput{
		Version: req.Version,
		UserID:  user.ID,
		RefactorPreviewInput: RefactorPreviewInput{
			PageID: id, Kind: req.Kind, Title: req.Title, Slug: req.Slug,
			Content: req.Content, NewParentID: req.NewParentID,
		},
		RewriteLinks: req.RewriteLinks,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	r.respondPage(c, http.StatusOK, page)
}

func (r *Routes) respondPage(c *gin.Context, status int, page *tree.Page) {
	apiPage := dto.ToAPIPage(page, r.userResolver)
	r.enrichPageMetadata(apiPage)
	c.JSON(status, apiPage)
}

func (r *Routes) respondPageWithDepth(c *gin.Context, status int, page *tree.Page, depth int) {
	apiPage := dto.ToAPIPageWithDepth(page, r.userResolver, depth)
	r.enrichPageMetadata(apiPage)
	c.JSON(status, apiPage)
}

func (r *Routes) enrichPageMetadata(page *dto.Page) {
	EnrichPageMetadata(page, r.treeService.ReadPageRaw)
}

func ValidatePageMetadataInput(tags []string, properties map[string]string) error {
	ve := sharederrors.NewValidationErrors()
	seenTags := map[string]struct{}{}

	for index, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		field := "tags[" + strconv.Itoa(index) + "]"
		if trimmed == "" {
			ve.Add(field, "Tag must not be empty")
			continue
		}
		if trimmed != tag {
			ve.Add(field, "Tag must not contain leading or trailing whitespace")
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seenTags[key]; exists {
			ve.Add(field, "Tag must be unique")
			continue
		}
		seenTags[key] = struct{}{}
	}

	for rawKey := range properties {
		key := strings.TrimSpace(rawKey)
		field := "properties." + rawKey
		switch {
		case key == "":
			ve.Add(field, "Property key must not be empty")
		case key != rawKey:
			ve.Add(field, "Property key must not contain leading or trailing whitespace")
		case strings.HasPrefix(strings.ToLower(key), "leafwiki_"):
			ve.Add(field, "Property key uses a reserved prefix")
		case strings.ToLower(key) == "tags" || strings.ToLower(key) == "title":
			ve.Add(field, "Property key is reserved")
		}
	}

	if ve.HasErrors() {
		return ve
	}

	return nil
}
