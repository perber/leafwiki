package pages

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
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
	authGroup.POST("/pages/:id/refactor/preview", authmw.RequireEditorOrAdmin(), r.handleRefactorPreview)
	authGroup.POST("/pages/:id/refactor/apply", authmw.RequireEditorOrAdmin(), r.handleRefactorApply)
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
	c.JSON(http.StatusOK, dto.ToAPIPage(out.Page, r.userResolver))
}

func (r *Routes) handleGetByPath(c *gin.Context) {
	routePath := strings.TrimSpace(c.Query("path"))
	if routePath == "" {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageMissingPath, "Missing path", "missing path")
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
	c.JSON(http.StatusOK, dto.ToAPIPageWithDepth(out.Page, r.userResolver, depth))
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
	title := strings.TrimSpace(c.Query("title"))
	if title == "" {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageMissingTitle, "Title query param is required", "title query param is required")
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
	kind := kindFromString(req.Kind)
	out, err := r.createPage.Execute(c.Request.Context(), CreatePageInput{
		UserID: user.ID, ParentID: req.ParentID, Title: req.Title, Slug: req.Slug, Kind: &kind,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToAPIPage(out.Page, r.userResolver))
}

func (r *Routes) handleUpdate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Title   string  `json:"title" binding:"required"`
		Slug    string  `json:"slug" binding:"required"`
		Content *string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	kind := tree.NodeKindPage
	out, err := r.updatePage.Execute(c.Request.Context(), UpdatePageInput{
		UserID: user.ID, ID: id, Title: req.Title, Slug: req.Slug, Content: req.Content, Kind: &kind,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToAPIPage(out.Page, r.userResolver))
}

func (r *Routes) handleDelete(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	recursive := c.DefaultQuery("recursive", "false") == "true"
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	if err := r.deletePage.Execute(c.Request.Context(), DeletePageInput{
		UserID: user.ID, ID: id, Recursive: recursive,
	}); err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Page deleted"})
}

func (r *Routes) handleMove(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
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
		UserID: user.ID, ID: id, ParentID: req.ParentID,
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
	kind := kindFromString(req.Kind)
	out, err := r.ensurePath.Execute(c.Request.Context(), EnsurePathInput{
		UserID: user.ID, TargetPath: req.Path, TargetTitle: req.Title, Kind: &kind,
	})
	if err != nil {
		respondWithPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToAPIPage(out.Page, r.userResolver))
}

func (r *Routes) handleConvert(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Kind string `json:"targetKind" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidRequest, "Invalid request", "invalid request")
		return
	}
	if req.Kind != "page" && req.Kind != "section" {
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageInvalidTargetKind, "Invalid targetKind", "invalid target kind")
		return
	}
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	if err := r.convertPage.Execute(c.Request.Context(), ConvertPageInput{
		UserID: user.ID, ID: id, TargetKind: tree.NodeKind(req.Kind),
	}); err != nil {
		respondWithPageError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
	c.JSON(http.StatusCreated, dto.ToAPIPage(out.Page, r.userResolver))
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
		UserID: user.ID,
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
	c.JSON(http.StatusOK, dto.ToAPIPage(page, r.userResolver))
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// kindFromString converts an optional string pointer to a NodeKind.
// Defaults to NodeKindPage when nil or unrecognized.
func kindFromString(s *string) tree.NodeKind {
	if s != nil && *s == string(tree.NodeKindSection) {
		return tree.NodeKindSection
	}
	return tree.NodeKindPage
}
