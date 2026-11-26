package revisions

import (
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/http/dto"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the revisions domain.
const (
	errPageIDRequiredUserMsg   = "Page ID is required"
	errPageIDRequiredLogMsg    = "page id is required"
	errRevisionNotFoundUserMsg = "Revision not found"
)

type Routes struct {
	listRevisions    *ListRevisionsUseCase
	getRevision      *GetRevisionUseCase
	compareRevisions *CompareRevisionsUseCase
	getRevisionAsset *GetRevisionAssetUseCase
	getLatest        *GetLatestRevisionUseCase
	restoreRevision  *RestoreRevisionUseCase
	checkIntegrity   *CheckIntegrityUseCase
	userResolver     *coreauth.UserResolver
	authService      *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	ListRevisions    *ListRevisionsUseCase
	GetRevision      *GetRevisionUseCase
	CompareRevisions *CompareRevisionsUseCase
	GetRevisionAsset *GetRevisionAssetUseCase
	GetLatest        *GetLatestRevisionUseCase
	RestoreRevision  *RestoreRevisionUseCase
	CheckIntegrity   *CheckIntegrityUseCase
	UserResolver     *coreauth.UserResolver
	AuthService      *coreauth.AuthService
}

// NewRoutes constructs the revisions RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		listRevisions:    cfg.ListRevisions,
		getRevision:      cfg.GetRevision,
		compareRevisions: cfg.CompareRevisions,
		getRevisionAsset: cfg.GetRevisionAsset,
		getLatest:        cfg.GetLatest,
		restoreRevision:  cfg.RestoreRevision,
		checkIntegrity:   cfg.CheckIntegrity,
		userResolver:     cfg.UserResolver,
		authService:      cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	authGroup := ctx.APIAuthGroup(r.authService)

	// Revision routes are behind the EnableRevision feature flag.
	if opts.EnableRevision {
		authGroup.GET("/pages/:id/revisions", r.handleListRevisions)
		authGroup.GET("/pages/:id/revisions/latest", r.handleGetLatestRevision)
		authGroup.GET("/pages/:id/revisions/compare", r.handleCompareRevisions)
		authGroup.GET("/pages/:id/revisions/:revisionId/assets/*name", r.handleGetRevisionAsset)
		authGroup.GET("/pages/:id/revisions/:revisionId", r.handleGetRevision)
		authGroup.POST("/pages/:id/revisions/:revisionId/restore", authmw.RequireEditorOrAdmin(), r.handleRestoreRevision)
	}

}

// ─── Handlers ───────────────────────────────────────────────────────────────

// handleListRevisions handles GET /api/pages/{id}/revisions
//
//	@Summary		List the revisions of a page
//	@Description	Returns one page of the page's revisions. Pass nextCursor of a response as cursor to get the next page; it is empty on the last page. A page without revisions, an unknown page ID and an unknown cursor all answer 200 with an empty list.
//	@Description	Access: a signed-in user of any role (session cookie, API key or reverse-proxy identity). Public access does not open it: anonymous callers get 401 also when public access is on. With authentication disabled (--disable-auth) anyone can read it. Requires --enable-revision; without it the route does not exist (404).
//	@Tags			revisions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string							true	"Page ID"
//	@Param			limit	query		integer							false	"Maximum number of revisions, 1 to 200; blank means 50, any other value answers 400"	default(50)	minimum(1)	maximum(200)
//	@Param			cursor	query		string							false	"nextCursor of the previous response; omit for the first page"
//	@Success		200		{object}	revisions.RevisionListResponse	"One page of revisions; an empty list when there are none"
//	@Failure		400		{object}	dto.ErrorResponse				"Blank page ID (code revision_invalid_page_id), or limit not an integer from 1 to 200 (code revision_invalid_limit)"
//	@Failure		401		{object}	dto.LegacyError					"No valid session (also when public access is on), an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		429		{object}	dto.LegacyError					"API-key attempts throttled"
//	@Failure		500		{object}	object							"Reading the revisions failed, also for a malformed page ID: the body is an ErrorResponse (code revision_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503		{object}	dto.LegacyError					"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/pages/{id}/revisions [get]
func (r *Routes) handleListRevisions(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	if pageID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionInvalidPageID, errPageIDRequiredUserMsg, errPageIDRequiredLogMsg)
		return
	}

	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 200 {
			respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionInvalidLimit, "Revision list limit is invalid", "revision list limit for page %s is invalid", pageID)
			return
		}
		limit = parsed
	}

	out, err := r.listRevisions.Execute(c.Request.Context(), ListRevisionsInput{
		PageID: pageID,
		Cursor: strings.TrimSpace(c.Query("cursor")),
		Limit:  limit,
	})
	if err != nil {
		respondWithRevisionError(c, err)
		return
	}

	c.JSON(http.StatusOK, toRevisionListResponse(out, r.userResolver))
}

// handleGetRevision handles GET /api/pages/{id}/revisions/{revisionId}
//
//	@Summary		Get a revision of a page
//	@Description	Returns the revision as a snapshot: its metadata, the Markdown content of the page at that revision and the list of its assets. An empty revision ID (a path ending in /revisions//) never reaches this operation: the router answers 301 Moved Permanently to /revisions/, the list route. A blank one (only spaces) answers 400.
//	@Description	Access: a signed-in user of any role (session cookie, API key or reverse-proxy identity). Public access does not open it: anonymous callers get 401 also when public access is on. With authentication disabled (--disable-auth) anyone can read it. Requires --enable-revision; without it the route does not exist (404).
//	@Tags			revisions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		string								true	"Page ID"
//	@Param			revisionId	path		string								true	"Revision ID"
//	@Success		200			{object}	revisions.RevisionSnapshotResponse	"The revision snapshot"
//	@Failure		400			{object}	dto.ErrorResponse					"Blank page ID (code revision_invalid_page_id) or blank revision ID (code revision_invalid_revision_id)"
//	@Failure		401			{object}	dto.LegacyError						"No valid session (also when public access is on), an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		404			{object}	dto.ErrorResponse					"No such revision for this page (code revision_not_found)"
//	@Failure		429			{object}	dto.LegacyError						"API-key attempts throttled"
//	@Failure		500			{object}	object								"Reading the revision failed, also for a malformed page ID: the body is an ErrorResponse (code revision_internal_error, revision_preview_content_unavailable or revision_preview_assets_unavailable). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503			{object}	dto.LegacyError						"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/pages/{id}/revisions/{revisionId} [get]
func (r *Routes) handleGetRevision(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	revisionID := strings.TrimSpace(c.Param("revisionId"))
	if pageID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionInvalidPageID, errPageIDRequiredUserMsg, errPageIDRequiredLogMsg)
		return
	}
	if revisionID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionInvalidRevisionID, "Revision ID is required", "revision id is required")
		return
	}

	out, err := r.getRevision.Execute(c.Request.Context(), GetRevisionInput{PageID: pageID, RevisionID: revisionID})
	if err != nil {
		respondWithRevisionError(c, err)
		return
	}
	if out.Snapshot == nil || out.Snapshot.Revision == nil {
		respondWithRevisionStatusError(c, http.StatusNotFound, ErrCodeRevisionNotFound, errRevisionNotFoundUserMsg, "revision %s for page %s not found", revisionID, pageID)
		return
	}
	c.JSON(http.StatusOK, toSnapshotResponse(out.Snapshot, r.userResolver))
}

// handleGetLatestRevision handles GET /api/pages/{id}/revisions/latest
//
//	@Summary		Get the latest revision of a page
//	@Description	Returns the metadata of the page's newest revision, without its content or assets.
//	@Description	Access: a signed-in user of any role (session cookie, API key or reverse-proxy identity). Public access does not open it: anonymous callers get 401 also when public access is on. With authentication disabled (--disable-auth) anyone can read it. Requires --enable-revision; without it the route does not exist (404).
//	@Tags			revisions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string						true	"Page ID"
//	@Success		200	{object}	revisions.RevisionResponse	"The latest revision"
//	@Failure		400	{object}	dto.ErrorResponse			"Blank page ID (code revision_invalid_page_id)"
//	@Failure		401	{object}	dto.LegacyError				"No valid session (also when public access is on), an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		404	{object}	dto.ErrorResponse			"No revision for this page ID (code revision_not_found)"
//	@Failure		429	{object}	dto.LegacyError				"API-key attempts throttled"
//	@Failure		500	{object}	object						"Reading the revision failed, also for a malformed page ID: the body is an ErrorResponse (code revision_internal_error). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503	{object}	dto.LegacyError				"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/pages/{id}/revisions/latest [get]
func (r *Routes) handleGetLatestRevision(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	if pageID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionInvalidPageID, errPageIDRequiredUserMsg, errPageIDRequiredLogMsg)
		return
	}

	out, err := r.getLatest.Execute(c.Request.Context(), GetLatestRevisionInput{PageID: pageID})
	if err != nil {
		respondWithRevisionError(c, err)
		return
	}
	if out.Revision == nil {
		respondWithRevisionStatusError(c, http.StatusNotFound, ErrCodeRevisionNotFound, errRevisionNotFoundUserMsg, "revision for page %s not found", pageID)
		return
	}
	c.JSON(http.StatusOK, toRevisionResponse(out.Revision, r.userResolver))
}

// handleCompareRevisions handles GET /api/pages/{id}/revisions/compare
//
//	@Summary		Compare two revisions of a page
//	@Description	Returns both revisions as snapshots (metadata, Markdown content and asset list), whether the content differs, and the asset changes from base to target, each with status added, removed or modified.
//	@Description	Access: a signed-in user of any role (session cookie, API key or reverse-proxy identity). Public access does not open it: anonymous callers get 401 also when public access is on. With authentication disabled (--disable-auth) anyone can read it. Requires --enable-revision; without it the route does not exist (404).
//	@Tags			revisions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string									true	"Page ID"
//	@Param			base	query		string									true	"ID of the base revision"
//	@Param			target	query		string									true	"ID of the target revision"
//	@Success		200		{object}	revisions.RevisionComparisonResponse	"The comparison"
//	@Failure		400		{object}	dto.ErrorResponse						"Blank page ID (code revision_invalid_page_id), or base or target missing (code revision_compare_invalid_request)"
//	@Failure		401		{object}	dto.LegacyError							"No valid session (also when public access is on), an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		404		{object}	dto.ErrorResponse						"base or target is not a revision of this page (code revision_not_found)"
//	@Failure		429		{object}	dto.LegacyError							"API-key attempts throttled"
//	@Failure		500		{object}	object									"Reading a revision failed, also for a malformed page ID: the body is an ErrorResponse (code revision_internal_error, revision_preview_content_unavailable or revision_preview_assets_unavailable). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503		{object}	dto.LegacyError							"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/pages/{id}/revisions/compare [get]
func (r *Routes) handleCompareRevisions(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	baseRevisionID := strings.TrimSpace(c.Query("base"))
	targetRevisionID := strings.TrimSpace(c.Query("target"))
	if pageID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionInvalidPageID, errPageIDRequiredUserMsg, errPageIDRequiredLogMsg)
		return
	}
	if baseRevisionID == "" || targetRevisionID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionCompareInvalidRequest, "Revision compare request is invalid", "revision compare request for page %s is invalid", pageID)
		return
	}

	out, err := r.compareRevisions.Execute(c.Request.Context(), CompareRevisionsInput{
		PageID: pageID, BaseRevisionID: baseRevisionID, TargetRevisionID: targetRevisionID,
	})
	if err != nil {
		respondWithRevisionError(c, err)
		return
	}
	if out.Comparison == nil || out.Comparison.Base == nil || out.Comparison.Target == nil {
		respondWithRevisionStatusError(c, http.StatusNotFound, ErrCodeRevisionNotFound, errRevisionNotFoundUserMsg, "revision compare resource for page %s not found", pageID)
		return
	}
	c.JSON(http.StatusOK, toComparisonResponse(out.Comparison, r.userResolver))
}

// handleGetRevisionAsset handles GET /api/pages/{id}/revisions/{revisionId}/assets/{name}
//
//	@Summary		Read an asset of a revision
//	@Description	Returns an asset as it was stored with the revision. The name is the rest of the path and may contain "/", which Swagger 2.0 cannot express for a path parameter; revisions store only top-level files, so such a name answers 404. Responses carry X-Content-Type-Options: nosniff, Content-Security-Policy: sandbox except for PDFs, and Content-Disposition inline for raster images, audio, video and PDFs, attachment otherwise; script types are served as application/octet-stream. Range requests are supported.
//	@Description	Access: a signed-in user of any role (session cookie, API key or reverse-proxy identity). Public access does not open it: anonymous callers get 401 also when public access is on. With authentication disabled (--disable-auth) anyone can read it. Requires --enable-revision; without it the route does not exist (404).
//	@Tags			revisions
//	@Produce		octet-stream
//	@Security		BearerAuth
//	@Param			id			path		string				true	"Page ID"
//	@Param			revisionId	path		string				true	"Revision ID"
//	@Param			name		path		string				true	"Asset name; may contain /"
//	@Success		200			{file}		binary				"The asset"
//	@Failure		400			{object}	dto.ErrorResponse	"Blank page ID (code revision_invalid_page_id), empty or blank revision ID (code revision_invalid_revision_id), or empty asset name (code revision_preview_asset_invalid_name)"
//	@Failure		401			{object}	dto.LegacyError		"No valid session (also when public access is on), an invalid, revoked or expired API key (when API-key management is enabled), or a rejected trusted-proxy identity (when reverse-proxy authentication is enabled)"
//	@Failure		404			{object}	dto.ErrorResponse	"No such revision for this page (code revision_not_found), or the revision has no asset with this name (code revision_preview_asset_not_found)"
//	@Failure		429			{object}	dto.LegacyError		"API-key attempts throttled"
//	@Failure		500			{object}	object				"Reading the asset failed, also for a malformed page ID: the body is an ErrorResponse (code revision_internal_error, revision_preview_assets_unavailable or revision_preview_asset_blob_unavailable). Authentication middleware failures return a LegacyError instead. Swagger 2.0 cannot express this alternative, so the schema is a broad object."
//	@Failure		503			{object}	dto.LegacyError		"API-key store unavailable during a live restore (when API-key management is enabled)"
//	@Router			/pages/{id}/revisions/{revisionId}/assets/{name} [get]
func (r *Routes) handleGetRevisionAsset(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	revisionID := strings.TrimSpace(c.Param("revisionId"))
	assetName := strings.TrimSpace(strings.TrimPrefix(c.Param("name"), "/"))
	if pageID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionInvalidPageID, errPageIDRequiredUserMsg, errPageIDRequiredLogMsg)
		return
	}
	if revisionID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionInvalidRevisionID, "Revision ID is required", "revision id is required")
		return
	}
	if assetName == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionPreviewAssetInvalidName, "Revision asset name is invalid", "revision asset name for page %s revision %s is invalid", pageID, revisionID)
		return
	}

	out, err := r.getRevisionAsset.Execute(c.Request.Context(), GetRevisionAssetInput{
		PageID: pageID, RevisionID: revisionID, AssetName: assetName,
	})
	if err != nil {
		respondWithRevisionError(c, err)
		return
	}
	if out.Asset == nil {
		respondWithRevisionStatusError(c, http.StatusNotFound, ErrCodeRevisionPreviewAssetNotFound, "Revision asset not found", "revision asset %s for page %s revision %s not found", assetName, pageID, revisionID)
		return
	}

	f, err := os.Open(out.Asset.Path)
	if err != nil {
		respondWithRevisionStatusError(c, http.StatusInternalServerError, ErrCodeRevisionPreviewAssetBlobUnavailable, "Revision asset blob is unavailable", "revision asset blob %s for page %s revision %s is unavailable", assetName, pageID, revisionID)
		return
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		respondWithRevisionStatusError(c, http.StatusInternalServerError, ErrCodeRevisionInternalError, "Failed to load revision asset", "failed to load revision asset")
		return
	}

	contentType := out.Asset.Asset.MIMEType
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(path.Ext(assetName)))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Writer.Header().Set("Content-Type", contentType)
	security.SetUntrustedFileHeaders(c.Writer.Header(), assetName) // after Content-Type: it may replace a script type
	http.ServeContent(c.Writer, c.Request, path.Base(assetName), stat.ModTime(), f)
}

func (r *Routes) handleRestoreRevision(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	revisionID := strings.TrimSpace(c.Param("revisionId"))
	if pageID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionRestoreInvalidPageID, "Failed to restore page", "failed to restore page %s", pageID)
		return
	}
	if revisionID == "" {
		respondWithRevisionStatusError(c, http.StatusBadRequest, ErrCodeRevisionRestoreInvalidRevision, "Restore revision is invalid", "restore revision %s for page %s is invalid", revisionID, pageID)
		return
	}

	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}

	out, err := r.restoreRevision.Execute(c.Request.Context(), RestoreRevisionInput{
		UserID: user.ID, PageID: pageID, RevisionID: revisionID,
	})
	if err != nil {
		respondWithRevisionError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToAPIPage(out.Page, r.userResolver))
}
