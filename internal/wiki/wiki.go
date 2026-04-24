package wiki

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/perber/wiki/internal/branding"
	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	coreimporter "github.com/perber/wiki/internal/importer"
	"github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/search"
	wikiassets "github.com/perber/wiki/internal/wiki/assets"
	wikiauth "github.com/perber/wiki/internal/wiki/auth"
	wikibranding "github.com/perber/wiki/internal/wiki/branding"
	wikiimporter "github.com/perber/wiki/internal/wiki/importer"
	wikilinks "github.com/perber/wiki/internal/wiki/links"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
	wikisearch "github.com/perber/wiki/internal/wiki/search"
)

type Wiki struct {
	tree         *tree.TreeService
	slug         *tree.SlugService
	auth         *auth.AuthService
	userResolver *auth.UserResolver
	user         *auth.UserService
	asset        *assets.AssetService
	branding     *branding.BrandingService
	searchIndex  *search.SQLiteIndex
	status       *search.IndexingStatus
	storageDir   string

	// Domain route registrars (populated by NewWiki).
	pagesRoutes     *wikipages.Routes
	authRoutes      *wikiauth.Routes
	assetsRoutes    *wikiassets.Routes
	revisionsRoutes *wikirevisions.Routes
	searchRoutes    *wikisearch.Routes
	linksRoutes     *wikilinks.Routes
	brandingRoutes  *wikibranding.Routes
	importerRoutes  *wikiimporter.Routes
	searchWatcher   *search.Watcher
	revision        *revision.Service
	links           *links.LinkService
	log             *slog.Logger
}

const SYSTEM_USER_ID = "system"

func searchRootDir(storageDir string) string {
	normalized := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	return filepath.Join(normalized, "root")
}

type WikiOptions struct {
	StorageDir              string        // Path to storage directory
	AdminPassword           string        // Initial admin password
	JWTSecret               string        // JWT secret for authentication
	AccessTokenTimeout      time.Duration // Access token timeout duration
	RefreshTokenTimeout     time.Duration // Refresh token timeout duration
	AuthDisabled            bool          // Whether authentication is disabled
	MaxRevisionHistory      int           // Max revisions kept per page; 0 = unlimited
	MaxAssetUploadSizeBytes int64         // Maximum allowed size in bytes for asset/import uploads; 0 = default
}

func NewWiki(options *WikiOptions) (*Wiki, error) {
	w := &Wiki{
		storageDir: options.StorageDir,
		log:        slog.Default().With("component", "Wiki"),
	}
	if err := w.initAuth(options); err != nil {
		return nil, err
	}
	if err := w.initCoreServices(options); err != nil {
		return nil, err
	}
	if err := w.initLinkService(); err != nil {
		return nil, err
	}
	if err := w.initSearch(); err != nil {
		return nil, err
	}
	if err := w.initBranding(); err != nil {
		return nil, err
	}
	// Welcome page must exist before the revision service starts recording.
	if err := w.EnsureWelcomePage(); err != nil {
		return nil, err
	}
	w.revision = revision.NewService(w.storageDir, w.tree, w.log,
		revision.ServiceOptions{MaxRevisions: options.MaxRevisionHistory})
	w.buildRoutes(options)
	return w, nil
}

// ─── Subsystem initializers ───────────────────────────────────────────────────

func (w *Wiki) initAuth(options *WikiOptions) error {
	store, err := auth.NewUserStore(w.storageDir)
	if err != nil {
		return err
	}
	w.user = auth.NewUserService(store)
	if !options.AuthDisabled {
		if err := w.user.InitDefaultAdmin(options.AdminPassword); err != nil {
			return err
		}
	}
	w.userResolver, err = auth.NewUserResolver(w.user)
	if err != nil {
		return err
	}
	if !options.AuthDisabled {
		sessionStore, err := auth.NewSessionStore(w.storageDir)
		if err != nil {
			return err
		}
		w.auth = auth.NewAuthService(w.user, sessionStore, options.JWTSecret, options.AccessTokenTimeout, options.RefreshTokenTimeout)
	}
	return nil
}

func (w *Wiki) initCoreServices(options *WikiOptions) error {
	w.tree = tree.NewTreeService(w.storageDir)
	if err := w.tree.LoadTree(); err != nil {
		return err
	}
	w.slug = tree.NewSlugService()
	w.asset = assets.NewAssetService(w.storageDir, w.slug)
	return nil
}

func (w *Wiki) initLinkService() error {
	linksStore, err := links.NewLinksStore(w.storageDir)
	if err != nil {
		return fmt.Errorf("failed to init links store: %w", err)
	}
	w.links = links.NewLinkService(w.storageDir, w.tree, linksStore)
	if err := w.links.IndexAllPages(); err != nil {
		w.log.Warn("failed to index links on startup", "error", err)
	}
	return nil
}

func (w *Wiki) initSearch() error {
	var err error
	w.searchIndex, err = search.NewSQLiteIndex(w.storageDir)
	if err != nil {
		return fmt.Errorf("failed to init search index: %w", err)
	}
	w.status = search.NewIndexingStatus()
	go func() {
		if err := search.BuildAndRunIndexer(w.tree, w.searchIndex, searchRootDir(w.storageDir), 4, w.status); err != nil {
			w.log.Warn("indexing failed", "error", err)
		}
	}()
	w.searchWatcher, err = search.NewWatcher(searchRootDir(w.storageDir), w.tree, w.searchIndex, w.status)
	if err != nil {
		w.log.Warn("failed to create file watcher", "error", err)
		return nil
	}
	go func() {
		if err := w.searchWatcher.Start(); err != nil {
			w.log.Warn("failed to start file watcher", "error", err)
		}
	}()
	return nil
}

func (w *Wiki) initBranding() error {
	var err error
	w.branding, err = branding.NewBrandingService(w.storageDir)
	if err != nil {
		return fmt.Errorf("failed to init branding service: %w", err)
	}
	return nil
}

func (w *Wiki) buildRoutes(options *WikiOptions) {
	w.pagesRoutes = w.buildPagesRoutes()
	w.authRoutes = w.buildAuthRoutes()
	w.assetsRoutes = w.buildAssetsRoutes()
	w.revisionsRoutes = w.buildRevisionsRoutes()
	w.searchRoutes = w.buildSearchRoutes()
	w.linksRoutes = w.buildLinksRoutes()
	w.brandingRoutes = w.buildBrandingRoutes()
	w.importerRoutes = w.buildImporterRoutes(options)
}

// ─── Domain route builder helpers ────────────────────────────────────────────

func (w *Wiki) buildPagesRoutes() *wikipages.Routes {
	return wikipages.NewRoutes(wikipages.RoutesConfig{
		TreeService:     w.tree,
		CreatePage:      wikipages.NewCreatePageUseCase(w.tree, w.slug, w.revision, w.links, w.log),
		UpdatePage:      wikipages.NewUpdatePageUseCase(w.tree, w.slug, w.revision, w.links, w.log),
		DeletePage:      wikipages.NewDeletePageUseCase(w.tree, w.revision, w.links, w.asset, w.log),
		MovePage:        wikipages.NewMovePageUseCase(w.tree, w.revision, w.links, w.log),
		ConvertPage:     wikipages.NewConvertPageUseCase(w.tree, w.revision, w.log),
		CopyPage:        wikipages.NewCopyPageUseCase(w.tree, w.slug, w.revision, w.links, w.asset, w.log),
		GetPage:         wikipages.NewGetPageUseCase(w.tree),
		FindByPath:      wikipages.NewFindByPathUseCase(w.tree),
		LookupPath:      wikipages.NewLookupPagePathUseCase(w.tree),
		SortPages:       wikipages.NewSortPagesUseCase(w.tree),
		EnsurePath:      wikipages.NewEnsurePathUseCase(w.tree, w.slug, w.revision, w.links, w.log),
		SuggestSlug:     wikipages.NewSuggestSlugUseCase(w.tree, w.slug),
		PreviewRefactor: wikipages.NewPreviewPageRefactorUseCase(w.tree, w.slug, w.links, w.log),
		ApplyRefactor:   wikipages.NewApplyPageRefactorUseCase(w.tree, w.slug, w.revision, w.links, w.log),
		UserResolver:    w.userResolver,
		AuthService:     w.auth,
	})
}

func (w *Wiki) buildAuthRoutes() *wikiauth.Routes {
	return wikiauth.NewRoutes(wikiauth.RoutesConfig{
		Login:             wikiauth.NewLoginUseCase(w.auth),
		Logout:            wikiauth.NewLogoutUseCase(w.auth),
		RefreshToken:      wikiauth.NewRefreshTokenUseCase(w.auth),
		CreateUser:        wikiauth.NewCreateUserUseCase(w.user, w.userResolver, w.log),
		UpdateUser:        wikiauth.NewUpdateUserUseCase(w.user, w.userResolver, w.log),
		ChangeOwnPassword: wikiauth.NewChangeOwnPasswordUseCase(w.user),
		DeleteUser:        wikiauth.NewDeleteUserUseCase(w.user, w.userResolver, w.log),
		GetUsers:          wikiauth.NewGetUsersUseCase(w.user),
		GetUserByID:       wikiauth.NewGetUserByIDUseCase(w.user),
		AuthService:       w.auth,
	})
}

func (w *Wiki) buildAssetsRoutes() *wikiassets.Routes {
	return wikiassets.NewRoutes(wikiassets.RoutesConfig{
		Upload:      wikiassets.NewUploadAssetUseCase(w.tree, w.asset, w.revision, w.log),
		List:        wikiassets.NewListAssetsUseCase(w.tree, w.asset),
		Rename:      wikiassets.NewRenameAssetUseCase(w.tree, w.asset, w.revision, w.log),
		Delete:      wikiassets.NewDeleteAssetUseCase(w.tree, w.asset, w.revision, w.log),
		AuthService: w.auth,
		AssetsDir:   w.asset.GetAssetsDir(),
		Log:         w.log,
	})
}

func (w *Wiki) buildRevisionsRoutes() *wikirevisions.Routes {
	return wikirevisions.NewRoutes(wikirevisions.RoutesConfig{
		ListRevisions:    wikirevisions.NewListRevisionsUseCase(w.revision),
		GetRevision:      wikirevisions.NewGetRevisionUseCase(w.revision),
		CompareRevisions: wikirevisions.NewCompareRevisionsUseCase(w.revision),
		GetRevisionAsset: wikirevisions.NewGetRevisionAssetUseCase(w.revision),
		GetLatest:        wikirevisions.NewGetLatestRevisionUseCase(w.revision),
		RestoreRevision:  wikirevisions.NewRestoreRevisionUseCase(w.revision, w.tree, w.links, w.log),
CheckIntegrity:   wikirevisions.NewCheckIntegrityUseCase(w.revision),
		UserResolver:     w.userResolver,
		AuthService:      w.auth,
	})
}

func (w *Wiki) buildSearchRoutes() *wikisearch.Routes {
	return wikisearch.NewRoutes(wikisearch.RoutesConfig{
		Search:            wikisearch.NewSearchUseCase(w.searchIndex),
		GetIndexingStatus: wikisearch.NewGetIndexingStatusUseCase(w.status),
		AuthService:       w.auth,
	})
}

func (w *Wiki) buildLinksRoutes() *wikilinks.Routes {
	return wikilinks.NewRoutes(wikilinks.RoutesConfig{
		GetLinkStatus: wikilinks.NewGetLinkStatusUseCase(w.links, w.tree),
		AuthService:   w.auth,
	})
}

func (w *Wiki) buildBrandingRoutes() *wikibranding.Routes {
	return wikibranding.NewRoutes(wikibranding.RoutesConfig{
		GetBranding:     wikibranding.NewGetBrandingUseCase(w.branding),
		UpdateBranding:  wikibranding.NewUpdateBrandingUseCase(w.branding),
		UploadLogo:      wikibranding.NewUploadLogoUseCase(w.branding),
		DeleteLogo:      wikibranding.NewDeleteLogoUseCase(w.branding),
		UploadFavicon:   wikibranding.NewUploadFaviconUseCase(w.branding),
		DeleteFavicon:   wikibranding.NewDeleteFaviconUseCase(w.branding),
		BrandingService: w.branding,
		AuthService:     w.auth,
		Log:             w.log,
	})
}

func (w *Wiki) buildImporterRoutes(options *WikiOptions) *wikiimporter.Routes {
	importerDir := filepath.Join(options.StorageDir, ".importer")
	adapter := NewWikiImportAdapter(w)
	planner := coreimporter.NewPlanner(adapter, w.slug)
	store := coreimporter.NewPlanStore(filepath.Join(importerDir, "current-plan.json"))
	svc := coreimporter.NewImporterService(planner, store, filepath.Join(importerDir, "workspaces"), options.MaxAssetUploadSizeBytes)
	return wikiimporter.NewRoutes(wikiimporter.RoutesConfig{
		CreatePlan:  wikiimporter.NewCreateImportPlanUseCase(svc),
		GetPlan:     wikiimporter.NewGetImportPlanUseCase(svc),
		Execute:     wikiimporter.NewExecuteImportUseCase(svc),
		ClearPlan:   wikiimporter.NewClearImportPlanUseCase(svc),
		AuthService: w.auth,
		Svc:         svc,
		Log:         w.log,
	})
}

// ─── Registrars / FrontendConfig ─────────────────────────────────────────────

// Registrars returns all domain route registrars in registration order.
func (w *Wiki) Registrars() []httpinternal.RouteRegistrar {
	return []httpinternal.RouteRegistrar{
		w.authRoutes,
		w.pagesRoutes,
		w.assetsRoutes,
		w.revisionsRoutes,
		w.searchRoutes,
		w.linksRoutes,
		w.brandingRoutes,
		w.importerRoutes,
	}
}

// FrontendConfig returns the minimal runtime data required by the router to serve the SPA.
func (w *Wiki) FrontendConfig() httpinternal.FrontendConfig {
	return httpinternal.FrontendConfig{
		StorageDir: w.storageDir,
		GetSiteName: func() string {
			cfg, err := w.branding.GetBranding()
			if err != nil || cfg == nil {
				return ""
			}
			return cfg.SiteName
		},
	}
}

func (w *Wiki) EnsureWelcomePage() error {
	if w.tree.HasPages() {
		w.log.Info("Welcome page already exists, skipping creation")
		return nil
	}
	k := tree.NodeKindPage
	createOut, err := wikipages.NewCreatePageUseCase(w.tree, w.slug, w.revision, w.links, w.log).Execute(
		context.Background(),
		wikipages.CreatePageInput{UserID: SYSTEM_USER_ID, Title: "Welcome to LeafWiki", Slug: "welcome-to-leafwiki", Kind: &k},
	)
	if err != nil {
		return err
	}
	p := createOut.Page

	// Set the content of the welcome page
	content := `# Welcome to LeafWiki!

LeafWiki – A fast wiki for people who think in folders, not feeds.
Single Go binary. Markdown on disk. No external database service.

LeafWiki is a lightweight, self-hosted wiki for runbooks, internal docs, and technical notes — built for fast writing and explicit structure. It keeps your content as plain Markdown on disk and gives you fast navigation, search, and editing — without running additional services.


---

## Features

- **Markdown-based** pages stored on disk (no database required)
- **Hierarchical navigation** with sections and pages
- **Full-text search** powered by SQLite FTS5
- **Asset management** (upload, rename, delete attachments)
- **Revision history** with snapshots and restore
- **Import** from Markdown zip archives
- **Branding** customization (site name, logo, favicon)
- **Multi-user** with role-based access control (admin / editor / viewer)
- **Public access mode** for read-only anonymous browsing

## Getting Started

1. Create your first page using the **+** button in the sidebar
2. Write in **Markdown** — headings, lists, code blocks, and links are all supported
3. Use **sections** to group related pages into a folder-like hierarchy
4. Upload files by dragging them into the editor

For more information, visit the [LeafWiki GitHub repository](https://github.com/perber/leafwiki).
`
	if _, err := wikipages.NewUpdatePageUseCase(w.tree, w.slug, w.revision, w.links, w.log).Execute(
		context.Background(),
		wikipages.UpdatePageInput{UserID: SYSTEM_USER_ID, ID: p.ID, Title: p.Title, Slug: p.Slug, Content: &content, Kind: &k},
	); err != nil {
		return err
	}

	return nil
}

// ─── Service getters (test infrastructure) ───────────────────────────────────

func (w *Wiki) GetStorageDir() string {
	return w.storageDir
}

func (w *Wiki) Close() error {
	w.status.Finish()
	if err := w.user.Close(); err != nil {
		return err
	}

	if w.links != nil {
		if err := w.links.Close(); err != nil {
			log.Printf("error closing links: %v", err)
		}
	}

	if w.searchWatcher != nil {
		if err := w.searchWatcher.Stop(); err != nil {
			log.Printf("error stopping search watcher: %v", err)
		}
	}

	return w.searchIndex.Close()
}
