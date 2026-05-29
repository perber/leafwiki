package mcp

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	wikiassets "github.com/perber/wiki/internal/wiki/assets"
	wikilinks "github.com/perber/wiki/internal/wiki/links"
	wikioauth "github.com/perber/wiki/internal/wiki/oauth"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	wikiproperties "github.com/perber/wiki/internal/wiki/properties"
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
	wikisearch "github.com/perber/wiki/internal/wiki/search"
	wikitags "github.com/perber/wiki/internal/wiki/tags"
)

const defaultToolListPageSize = 100

// Routes registers LeafWiki's local-only MCP Streamable HTTP endpoint.
type Routes struct {
	treeService  *tree.TreeService
	userResolver *coreauth.UserResolver
	userService  *coreauth.UserService
	oauthService *wikioauth.Service
	authDisabled bool
	createPage   *wikipages.CreatePageUseCase
	updatePage   *wikipages.UpdatePageUseCase
	getPage      *wikipages.GetPageUseCase
	findByPath   *wikipages.FindByPathUseCase
	lookupPath   *wikipages.LookupPagePathUseCase
	resolveLink  *wikipages.ResolvePermalinkUseCase
	suggestSlug  *wikipages.SuggestSlugUseCase
	deletePage   *wikipages.DeletePageUseCase
	movePage     *wikipages.MovePageUseCase
	sortPages    *wikipages.SortPagesUseCase
	ensurePath   *wikipages.EnsurePathUseCase
	convertPage  *wikipages.ConvertPageUseCase
	copyPage     *wikipages.CopyPageUseCase
	previewRef   *wikipages.PreviewPageRefactorUseCase
	applyRef     *wikipages.ApplyPageRefactorUseCase
	search       *wikisearch.SearchUseCase
	searchStatus *wikisearch.GetIndexingStatusUseCase
	getTags      *wikitags.GetTagsUseCase
	pagesByTags  *wikitags.GetPagesByTagsUseCase
	propertyKeys *wikiproperties.GetPropertyKeysUseCase
	pagesByProp  *wikiproperties.GetPagesByPropertyUseCase
	linkStatus   *wikilinks.GetLinkStatusUseCase
	uploadAsset  *wikiassets.UploadAssetUseCase
	getAsset     *wikiassets.GetAssetUseCase
	getAssets    *wikiassets.ListAssetsUseCase
	renameAsset  *wikiassets.RenameAssetUseCase
	deleteAsset  *wikiassets.DeleteAssetUseCase
	listRevs     *wikirevisions.ListRevisionsUseCase
	getRev       *wikirevisions.GetRevisionUseCase
	compareRevs  *wikirevisions.CompareRevisionsUseCase
	getRevAsset  *wikirevisions.GetRevisionAssetUseCase
	getLatestRev *wikirevisions.GetLatestRevisionUseCase
	restoreRev   *wikirevisions.RestoreRevisionUseCase
}

type RoutesConfig struct {
	TreeService  *tree.TreeService
	UserResolver *coreauth.UserResolver
	UserService  *coreauth.UserService
	OAuthService *wikioauth.Service
	CreatePage   *wikipages.CreatePageUseCase
	UpdatePage   *wikipages.UpdatePageUseCase
	GetPage      *wikipages.GetPageUseCase
	FindByPath   *wikipages.FindByPathUseCase
	LookupPath   *wikipages.LookupPagePathUseCase
	ResolveLink  *wikipages.ResolvePermalinkUseCase
	SuggestSlug  *wikipages.SuggestSlugUseCase
	DeletePage   *wikipages.DeletePageUseCase
	MovePage     *wikipages.MovePageUseCase
	SortPages    *wikipages.SortPagesUseCase
	EnsurePath   *wikipages.EnsurePathUseCase
	ConvertPage  *wikipages.ConvertPageUseCase
	CopyPage     *wikipages.CopyPageUseCase
	PreviewRef   *wikipages.PreviewPageRefactorUseCase
	ApplyRef     *wikipages.ApplyPageRefactorUseCase
	Search       *wikisearch.SearchUseCase
	SearchStatus *wikisearch.GetIndexingStatusUseCase
	GetTags      *wikitags.GetTagsUseCase
	PagesByTags  *wikitags.GetPagesByTagsUseCase
	PropertyKeys *wikiproperties.GetPropertyKeysUseCase
	PagesByProp  *wikiproperties.GetPagesByPropertyUseCase
	LinkStatus   *wikilinks.GetLinkStatusUseCase
	UploadAsset  *wikiassets.UploadAssetUseCase
	GetAsset     *wikiassets.GetAssetUseCase
	GetAssets    *wikiassets.ListAssetsUseCase
	RenameAsset  *wikiassets.RenameAssetUseCase
	DeleteAsset  *wikiassets.DeleteAssetUseCase
	ListRevs     *wikirevisions.ListRevisionsUseCase
	GetRev       *wikirevisions.GetRevisionUseCase
	CompareRevs  *wikirevisions.CompareRevisionsUseCase
	GetRevAsset  *wikirevisions.GetRevisionAssetUseCase
	GetLatestRev *wikirevisions.GetLatestRevisionUseCase
	RestoreRev   *wikirevisions.RestoreRevisionUseCase
}

func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		treeService:  cfg.TreeService,
		userResolver: cfg.UserResolver,
		userService:  cfg.UserService,
		oauthService: cfg.OAuthService,
		createPage:   cfg.CreatePage,
		updatePage:   cfg.UpdatePage,
		getPage:      cfg.GetPage,
		findByPath:   cfg.FindByPath,
		lookupPath:   cfg.LookupPath,
		resolveLink:  cfg.ResolveLink,
		suggestSlug:  cfg.SuggestSlug,
		deletePage:   cfg.DeletePage,
		movePage:     cfg.MovePage,
		sortPages:    cfg.SortPages,
		ensurePath:   cfg.EnsurePath,
		convertPage:  cfg.ConvertPage,
		copyPage:     cfg.CopyPage,
		previewRef:   cfg.PreviewRef,
		applyRef:     cfg.ApplyRef,
		search:       cfg.Search,
		searchStatus: cfg.SearchStatus,
		getTags:      cfg.GetTags,
		pagesByTags:  cfg.PagesByTags,
		propertyKeys: cfg.PropertyKeys,
		pagesByProp:  cfg.PagesByProp,
		linkStatus:   cfg.LinkStatus,
		uploadAsset:  cfg.UploadAsset,
		getAsset:     cfg.GetAsset,
		getAssets:    cfg.GetAssets,
		renameAsset:  cfg.RenameAsset,
		deleteAsset:  cfg.DeleteAsset,
		listRevs:     cfg.ListRevs,
		getRev:       cfg.GetRev,
		compareRevs:  cfg.CompareRevs,
		getRevAsset:  cfg.GetRevAsset,
		getLatestRev: cfg.GetLatestRev,
		restoreRev:   cfg.RestoreRev,
	}
}

func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	if !ctx.Opts.MCPEnabled || !httpinternal.IsLoopbackHost(ctx.Opts.MCPBindHost) {
		return
	}
	if !ctx.Opts.AuthDisabled && r.oauthService == nil {
		return
	}

	serverRoutes := *r
	serverRoutes.authDisabled = ctx.Opts.AuthDisabled
	server := serverRoutes.newServer(ctx.Opts)
	handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, &sdkmcp.StreamableHTTPOptions{
		Stateless:                  false,
		JSONResponse:               true,
		SessionTimeout:             30 * time.Minute,
		DisableLocalhostProtection: false,
	})

	var httpHandler http.Handler = handler
	if !ctx.Opts.AuthDisabled {
		httpHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			authenticated := sdkauth.RequireBearerToken(r.oauthService.VerifyBearerToken, &sdkauth.RequireBearerTokenOptions{
				ResourceMetadataURL: wikioauth.ProtectedResourceMetadataURL(req, ctx.Opts.BasePath),
				Scopes:              []string{wikioauth.ScopeMCP},
			})(handler)
			authenticated.ServeHTTP(w, req)
		})
	}

	wrapped := gin.WrapH(httpHandler)
	ctx.Base.GET("/mcp", wrapped)
	ctx.Base.POST("/mcp", wrapped)
	ctx.Base.DELETE("/mcp", wrapped)
}

func (r *Routes) newServer(opts httpinternal.RouterOptions) *sdkmcp.Server {
	pageSize := opts.MCPToolListPageSize
	if pageSize <= 0 {
		pageSize = defaultToolListPageSize
	}

	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "leafwiki",
		Version: "local",
	}, &sdkmcp.ServerOptions{PageSize: pageSize})

	r.registerConfigTools(server, opts)
	r.registerPageTools(server)
	r.registerSearchTools(server)
	r.registerTagTools(server)
	r.registerPropertyTools(server)
	r.registerLinkTools(server)
	r.registerAssetTools(server, opts)
	if opts.EnableRevision {
		r.registerRevisionTools(server)
	}
	if opts.EnableLinkRefactor {
		r.registerRefactorTools(server)
	}

	return server
}
