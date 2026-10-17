package wiki

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/perber/wiki/internal/avatar"
	"github.com/perber/wiki/internal/branding"
	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/email"
	"github.com/perber/wiki/internal/core/ignore"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/favorites"
	"github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/properties"
	"github.com/perber/wiki/internal/search"
	"github.com/perber/wiki/internal/tags"
	"github.com/perber/wiki/internal/usersettings"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	"github.com/perber/wiki/internal/wiki/pagesave"
)

// ─── Subsystem initializers ───────────────────────────────────────────────────

func (w *Wiki) initAuth(options *WikiOptions) error {
	store, err := auth.NewUserStore(w.storageDir)
	if err != nil {
		return err
	}
	w.user = auth.NewUserService(store)
	w.user.SetEditorLimit(options.EditorLimit)
	if !options.AuthDisabled {
		if err := w.user.InitDefaultAdmin(options.AdminUsername, options.AdminEmail, options.AdminPassword); err != nil {
			return err
		}
	}
	w.userResolver, err = auth.NewUserResolver(w.UserService)
	if err != nil {
		return err
	}
	if options.TOTPEncryptionKey != "" {
		totpService, err := auth.NewTOTPService([]byte(options.TOTPEncryptionKey))
		if err != nil {
			return fmt.Errorf("invalid TOTP encryption key: %w", err)
		}
		w.totp = totpService
	}
	if !options.AuthDisabled {
		sessionStore, err := auth.NewSessionStore(w.storageDir)
		if err != nil {
			return err
		}
		sessions := auth.NewSessionManager(sessionStore, options.JWTSecret, options.AccessTokenTimeout, options.RefreshTokenTimeout)
		w.auth = auth.NewAuthService(w.user, sessions, w.totp)

		// API keys are only meaningful when authentication is meaningful:
		// key management is admin-only and RequireAdmin already hard-blocks
		// admin operations when auth is disabled, so no key can be created
		// in that mode anyway. Keeping construction inside this block (like
		// sessionStore/w.auth) means w.APIKeyService() is nil when
		// AuthDisabled, which in turn keeps the Bearer middleware from being
		// registered at all — a key created before a later --disable-auth
		// restart must not keep authenticating (and narrowing/blocking)
		// requests in a mode where auth is supposed to be irrelevant.
		//
		// The feature is additionally gated behind EnableAPIKeyManagement:
		// it ships experimental and off by default, so w.apiKeys stays nil
		// (and the Bearer middleware/admin routes stay disabled) until an
		// operator explicitly opts in.
		if options.EnableAPIKeyManagement {
			apiKeyStore, err := auth.NewAPIKeyStore(w.storageDir)
			if err != nil {
				return err
			}
			w.apiKeys = auth.NewAPIKeyService(apiKeyStore, w.auth)
		}
	}
	return nil
}

// initEmail constructs the password-reset/invite token service when both
// SMTP is configured and auth is enabled — mirroring the EnableAPIKeyManagement
// block above, this keeps w.emailTokenService nil (and the corresponding
// routes returning ErrEmailDisabled) whenever either precondition is false,
// rather than constructing something unusable. Must run after initAuth,
// which sets w.user and w.auth.
func (w *Wiki) initEmail(options *WikiOptions) error {
	if !options.SMTP.Enabled() || options.AuthDisabled {
		return nil
	}

	store, err := auth.NewEmailTokenStore(w.storageDir)
	if err != nil {
		return err
	}
	w.emailTokenStore = store

	mailer := email.NewService(options.SMTP)
	w.emailTokenService = auth.NewEmailTokenService(store, w.user, w.auth, mailer, options.SMTP.PublicURL)
	return nil
}

func (w *Wiki) initCoreServices(options *WikiOptions) error {
	// Create a shared ignore cache for multi-level .leafwikiignore resolution.
	rootDir := filepath.Join(w.storageDir, "root")
	w.ignoreCache = ignore.NewCache(rootDir)

	w.tree = tree.NewTreeService(w.storageDir)
	w.tree.SetIgnoreCache(w.ignoreCache)
	if err := w.tree.LoadTree(); err != nil {
		return err
	}
	w.slug = tree.NewSlugService()
	w.asset = assets.NewAssetService(w.storageDir, w.slug)
	w.asset.SetIgnoreCache(w.ignoreCache)

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

func (w *Wiki) initTagsService() error {
	tagsStore, err := tags.NewTagsStore(w.storageDir)
	if err != nil {
		return fmt.Errorf("failed to init tags store: %w", err)
	}
	w.tags = tags.NewTagsService(tagsStore)
	return nil
}

func (w *Wiki) initPropertiesService() error {
	propsStore, err := properties.NewPropertiesStore(w.storageDir)
	if err != nil {
		return fmt.Errorf("failed to init properties store: %w", err)
	}
	w.props = properties.NewPropertiesService(propsStore)
	return nil
}

func (w *Wiki) initFavoritesService() error {
	store, err := favorites.NewFavoritesStore(w.storageDir, w.log)
	if err != nil {
		return fmt.Errorf("failed to init favorites store: %w", err)
	}
	w.favorites = store
	return nil
}

func (w *Wiki) initUserSettingsService() error {
	store, err := usersettings.NewUserSettingsStore(w.storageDir, w.log)
	if err != nil {
		return fmt.Errorf("failed to init user settings store: %w", err)
	}
	w.userSettings = usersettings.NewUserSettingsService(store)
	return nil
}

// bootstrapTagsAndProperties clears and rebuilds tag and property indexes in a single
// parallel GetPages pass — avoids two sequential ReadPageRaw loops at startup.
func (w *Wiki) bootstrapTagsAndProperties() {
	if err := w.tags.ClearIndex(); err != nil {
		w.log.Warn("failed to clear tags index before bootstrap", "error", err)
		return
	}
	if err := w.props.ClearIndex(); err != nil {
		w.log.Warn("failed to clear properties index before bootstrap", "error", err)
		return
	}
	var ids []string
	if err := w.tree.WalkNodes(func(id string) error {
		ids = append(ids, id)
		return nil
	}); err != nil {
		w.log.Warn("failed to walk pages for tags/properties bootstrap", "error", err)
		return
	}
	pages, errs := w.tree.GetPages(ids)
	for i, page := range pages {
		if errs[i] != nil {
			w.log.Warn("skipping page during bootstrap", "pageID", ids[i], "error", errs[i])
			continue
		}
		if err := w.tags.IndexPageContent(page.ID, page.RawContent); err != nil {
			w.log.Warn("failed to index tags", "pageID", page.ID, "error", err)
		}
		if err := w.props.IndexPageContent(page.ID, page.RawContent); err != nil {
			w.log.Warn("failed to index properties", "pageID", page.ID, "error", err)
		}
	}
}

func (w *Wiki) initSearch() error {
	var err error
	w.searchIndex, err = search.NewSQLiteIndex(w.storageDir)
	if err != nil {
		return fmt.Errorf("failed to init search index: %w", err)
	}
	w.status = search.NewIndexingStatus()
	searchEffect := pagesave.NewSearchIndexSideEffect(w.searchIndex, w.tree, w.log, w.metrics)
	w.log.Info("search indexing started")
	w.reloadWG.Add(1)
	go func() {
		defer w.reloadWG.Done()
		w.status.Start()
		defer w.status.Finish()
		if err := searchEffect.IndexAllPagesContext(w.shutdownCtx); err != nil {
			w.log.Warn("search bootstrap failed", "error", err)
			w.status.Fail()
		} else {
			w.log.Info("search indexing completed")
			w.status.Success()
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

func (w *Wiki) initAvatarService() error {
	var err error
	w.avatar, err = avatar.NewAvatarService(w.storageDir)
	if err != nil {
		return fmt.Errorf("failed to init avatar service: %w", err)
	}
	return nil
}

func (w *Wiki) ensureBaselineRevisions() {
	var ids []string
	if err := w.tree.WalkNodes(func(id string) error {
		ids = append(ids, id)
		return nil
	}); err != nil {
		w.log.Warn("failed to enumerate pages for baseline revisions", "error", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	pages, pageErrs := w.tree.GetPages(ids)
	var valid []*tree.Page
	for i, p := range pages {
		if pageErrs[i] != nil {
			w.log.Warn("failed to load page for baseline revision", "pageID", ids[i], "error", pageErrs[i])
			continue
		}
		if p != nil {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return
	}
	errs := w.revision.RecordContentUpdates(valid, SYSTEM_USER_ID, "baseline")
	for i, err := range errs {
		if err != nil {
			w.log.Warn("baseline revision failed", "pageID", valid[i].ID, "error", err)
		}
	}
}

func (w *Wiki) EnsureWelcomePage() error {
	if w.tree.HasPages() {
		w.log.Info("Welcome page already exists, skipping creation")
		return nil
	}
	o := w.newPageOrchestrator()
	k := tree.NodeKindPage
	createOut, err := wikipages.NewCreatePageUseCase(w.tree, w.slug, o, w.log, w.metrics).Execute(
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
	current, err := w.tree.GetPage(p.ID)
	if err != nil {
		return err
	}
	if _, err := wikipages.NewUpdatePageUseCase(w.tree, w.slug, o, w.log, w.metrics).Execute(
		context.Background(),
		wikipages.UpdatePageInput{UserID: SYSTEM_USER_ID, ID: p.ID, Version: current.Version(), Title: p.Title, Slug: p.Slug, Content: &content, Kind: &k},
	); err != nil {
		return err
	}

	return nil
}
