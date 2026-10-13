package wiki

import (
	"path/filepath"

	httpinternal "github.com/perber/wiki/internal/http"
	coreimporter "github.com/perber/wiki/internal/importer"
	wikiapikeys "github.com/perber/wiki/internal/wiki/apikeys"
	wikiassets "github.com/perber/wiki/internal/wiki/assets"
	wikiauth "github.com/perber/wiki/internal/wiki/auth"
	wikiavatar "github.com/perber/wiki/internal/wiki/avatar"
	wikibackup "github.com/perber/wiki/internal/wiki/backup"
	wikibranding "github.com/perber/wiki/internal/wiki/branding"
	wikihealth "github.com/perber/wiki/internal/wiki/health"
	wikiimporter "github.com/perber/wiki/internal/wiki/importer"
	wikiinstancesettings "github.com/perber/wiki/internal/wiki/instancesettings"
	wikilinks "github.com/perber/wiki/internal/wiki/links"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	"github.com/perber/wiki/internal/wiki/pagesave"
	wikiproperties "github.com/perber/wiki/internal/wiki/properties"
	wikirestore "github.com/perber/wiki/internal/wiki/restore"
	wikiresync "github.com/perber/wiki/internal/wiki/resync"
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
	wikisearch "github.com/perber/wiki/internal/wiki/search"
	wikisnapshot "github.com/perber/wiki/internal/wiki/snapshot"
	wikitags "github.com/perber/wiki/internal/wiki/tags"
	wikiusersettings "github.com/perber/wiki/internal/wiki/usersettings"
)

func (w *Wiki) buildRoutes(options *WikiOptions) {
	w.pagesRoutes = w.buildPagesRoutes()
	w.authRoutes = w.buildAuthRoutes()
	w.assetsRoutes = w.buildAssetsRoutes()
	w.revisionsRoutes = w.buildRevisionsRoutes()
	w.searchRoutes = w.buildSearchRoutes()
	w.linksRoutes = w.buildLinksRoutes()
	w.tagsRoutes = w.buildTagsRoutes()
	w.propertiesRoutes = w.buildPropertiesRoutes()
	w.brandingRoutes = w.buildBrandingRoutes()
	w.avatarRoutes = w.buildAvatarRoutes()
	w.userSettingsRoutes = w.buildUserSettingsRoutes()
	w.apiKeysRoutes = w.buildAPIKeysRoutes()
	w.importerRoutes = w.buildImporterRoutes(options)
	w.healthRoutes = wikihealth.NewRoutes(wikihealth.RoutesConfig{
		Index:      w.searchIndex,
		Status:     w.status,
		StorageDir: w.storageDir,
	})
	w.resyncRoutes = wikiresync.NewRoutes(
		wikiresync.NewTriggerResyncUseCase(w.resyncJob, w.launchReloadWithProgress, w.metrics),
		wikiresync.NewGetResyncStatusUseCase(w.resyncJob),
		w.auth,
	)
}

// ─── Domain route builder helpers ────────────────────────────────────────────

func (w *Wiki) newPageOrchestrator() *pagesave.PageSaveOrchestrator {
	return pagesave.NewPageSaveOrchestrator(
		w.metrics,
		pagesave.NewSearchIndexSideEffect(w.searchIndex, w.tree, w.log, w.metrics),
		pagesave.NewLinkIndexSideEffect(w.links, w.log, w.metrics),
		pagesave.NewRevisionSideEffect(w.revision, w.log, w.metrics),
		pagesave.NewTagsSideEffect(w.tags, w.log, w.metrics),
		pagesave.NewPropertiesSideEffect(w.props, w.log, w.metrics),
	)
}

func (w *Wiki) buildPagesRoutes() *wikipages.Routes {
	o := w.newPageOrchestrator()
	return wikipages.NewRoutes(wikipages.RoutesConfig{
		TreeService:      w.tree,
		CreatePage:       wikipages.NewCreatePageUseCase(w.tree, w.slug, o, w.log, w.metrics),
		UpdatePage:       wikipages.NewUpdatePageUseCase(w.tree, w.slug, o, w.log, w.metrics),
		DeletePage:       wikipages.NewDeletePageUseCase(w.tree, w.revision, w.asset, w.favorites, o, w.log, w.metrics),
		MovePage:         wikipages.NewMovePageUseCase(w.tree, o, w.log, w.metrics),
		ConvertPage:      wikipages.NewConvertPageUseCase(w.tree, w.revision, w.log),
		CopyPage:         wikipages.NewCopyPageUseCase(w.tree, w.slug, o, w.asset, w.log),
		GetPage:          wikipages.NewGetPageUseCase(w.tree),
		FindByPath:       wikipages.NewFindByPathUseCase(w.tree),
		FindByTitle:      wikipages.NewFindByTitleUseCase(w.tree),
		LookupPath:       wikipages.NewLookupPagePathUseCase(w.tree),
		ResolvePermalink: wikipages.NewResolvePermalinkUseCase(w.tree),
		SortPages:        wikipages.NewSortPagesUseCase(w.tree),
		EnsurePath:       wikipages.NewEnsurePathUseCase(w.tree, w.slug, o, w.log),
		SuggestSlug:      wikipages.NewSuggestSlugUseCase(w.tree, w.slug),
		PreviewRefactor:  wikipages.NewPreviewPageRefactorUseCase(w.tree, w.slug, w.links, w.log),
		ApplyRefactor:    wikipages.NewApplyPageRefactorUseCase(w.tree, w.slug, w.revision, w.links, w.log, w.metrics),
		PinPage:          wikipages.NewPinPageUseCase(w.tree, w.log),
		AddFavorite:      wikipages.NewAddFavoriteUseCase(w.tree, w.favorites),
		RemoveFavorite:   wikipages.NewRemoveFavoriteUseCase(w.favorites),
		ListFavorites:    wikipages.NewListFavoritesUseCase(w.tree, w.favorites, w.log),
		UserResolver:     w.userResolver,
		AuthService:      w.auth,
	})
}

func (w *Wiki) buildAuthRoutes() *wikiauth.Routes {
	return wikiauth.NewRoutes(wikiauth.RoutesConfig{
		Login:             wikiauth.NewLoginUseCase(w.auth, w.metrics),
		CompleteTOTPLogin: wikiauth.NewCompleteTOTPLoginUseCase(w.auth, w.metrics),
		Logout:            wikiauth.NewLogoutUseCase(w.auth, w.metrics),
		RefreshToken:      wikiauth.NewRefreshTokenUseCase(w.auth, w.metrics),
		CreateUser:        wikiauth.NewCreateUserUseCase(w.UserService, w.userResolver, w.log),
		UpdateUser:        wikiauth.NewUpdateUserUseCase(w.UserService, w.userResolver, w.auth, w.log),
		ChangeOwnPassword: wikiauth.NewChangeOwnPasswordUseCase(w.UserService, w.auth, w.log),
		DeleteUser:        wikiauth.NewDeleteUserUseCase(w.UserService, w.userResolver, w.favorites, w.userSettings, w.apiKeys, w.log),
		GetUsers:          wikiauth.NewGetUsersUseCase(w.UserService),
		GetUserByID:       wikiauth.NewGetUserByIDUseCase(w.UserService),
		StartTOTPSetup:    wikiauth.NewStartTOTPSetupUseCase(w.auth),
		ConfirmTOTPSetup:  wikiauth.NewConfirmTOTPSetupUseCase(w.auth, w.metrics),
		DisableTOTP:       wikiauth.NewDisableTOTPUseCase(w.auth, w.metrics),
		GetTOTPStatus:     wikiauth.NewGetTOTPStatusUseCase(w.auth),
		AuthService:       w.auth,

		RequestPasswordReset: wikiauth.NewRequestPasswordResetUseCase(w.EmailTokenService),
		ConfirmPasswordReset: wikiauth.NewConfirmPasswordResetUseCase(w.EmailTokenService),
		InviteUser:           wikiauth.NewInviteUserUseCase(w.UserService, w.EmailTokenService, w.userResolver, w.log),
		ResendInvite:         wikiauth.NewResendInviteUseCase(w.UserService, w.EmailTokenService),
		ConfirmInvite:        wikiauth.NewConfirmInviteUseCase(w.EmailTokenService, w.auth),
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
		RestoreRevision:  wikirevisions.NewRestoreRevisionUseCase(w.revision, w.tree, w.newPageOrchestrator(), w.log, w.metrics),
		CheckIntegrity:   wikirevisions.NewCheckIntegrityUseCase(w.revision),
		UserResolver:     w.userResolver,
		AuthService:      w.auth,
	})
}

func (w *Wiki) buildSearchRoutes() *wikisearch.Routes {
	return wikisearch.NewRoutes(wikisearch.RoutesConfig{
		Search:            wikisearch.NewSearchUseCase(w.searchIndex, w.tags, w.tree),
		GetIndexingStatus: wikisearch.NewGetIndexingStatusUseCase(w.status),
		AuthService:       w.auth,
	})
}

func (w *Wiki) buildLinksRoutes() *wikilinks.Routes {
	return wikilinks.NewRoutes(wikilinks.RoutesConfig{
		GetLinkStatus:  wikilinks.NewGetLinkStatusUseCase(w.links, w.tree),
		GetBrokenLinks: wikilinks.NewGetBrokenLinksUseCase(w.links),
		AuthService:    w.auth,
	})
}

func (w *Wiki) buildTagsRoutes() *wikitags.Routes {
	return wikitags.NewRoutes(wikitags.RoutesConfig{
		GetTags:        wikitags.NewGetTagsUseCase(w.tags),
		GetPagesByTags: wikitags.NewGetPagesByTagsUseCase(w.tags, w.tree, w.userResolver),
		AuthService:    w.auth,
	})
}

func (w *Wiki) buildPropertiesRoutes() *wikiproperties.Routes {
	return wikiproperties.NewRoutes(wikiproperties.RoutesConfig{
		GetPropertyKeys:    wikiproperties.NewGetPropertyKeysUseCase(w.props),
		GetPagesByProperty: wikiproperties.NewGetPagesByPropertyUseCase(w.props, w.tree, w.userResolver),
		AuthService:        w.auth,
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

func (w *Wiki) buildAvatarRoutes() *wikiavatar.Routes {
	return wikiavatar.NewRoutes(wikiavatar.RoutesConfig{
		UploadAvatar:  wikiavatar.NewUploadAvatarUseCase(w.avatar),
		DeleteAvatar:  wikiavatar.NewDeleteAvatarUseCase(w.avatar),
		AvatarService: w.avatar,
		AuthService:   w.auth,
	})
}

func (w *Wiki) buildUserSettingsRoutes() *wikiusersettings.Routes {
	return wikiusersettings.NewRoutes(wikiusersettings.RoutesConfig{
		GetUserSettings:    wikiusersettings.NewGetUserSettingsUseCase(w.userSettings),
		UpdateUserSettings: wikiusersettings.NewUpdateUserSettingsUseCase(w.userSettings),
		AuthService:        w.auth,
	})
}

func (w *Wiki) buildAPIKeysRoutes() *wikiapikeys.Routes {
	return wikiapikeys.NewRoutes(wikiapikeys.RoutesConfig{
		CreateAPIKey: wikiapikeys.NewCreateAPIKeyUseCase(w.apiKeys),
		ListAPIKeys:  wikiapikeys.NewListAPIKeysUseCase(w.apiKeys),
		RevokeAPIKey: wikiapikeys.NewRevokeAPIKeyUseCase(w.apiKeys),
		AuthService:  w.auth,
	})
}

func (w *Wiki) buildImporterRoutes(options *WikiOptions) *wikiimporter.Routes {
	importerDir := filepath.Join(options.StorageDir, ".importer")
	adapter := NewWikiImportAdapter(w)
	planner := coreimporter.NewPlanner(adapter, w.slug, options.StorageDir)
	planner.SetIgnoreCache(w.ignoreCache)

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
	registrars := []httpinternal.RouteRegistrar{
		w.authRoutes,
		w.pagesRoutes,
		w.assetsRoutes,
		w.revisionsRoutes,
		w.searchRoutes,
		w.linksRoutes,
		w.tagsRoutes,
		w.propertiesRoutes,
		w.brandingRoutes,
		w.avatarRoutes,
		w.userSettingsRoutes,
		w.apiKeysRoutes,
		w.importerRoutes,
		w.healthRoutes,
		w.resyncRoutes,
	}
	if w.backupRoutes != nil {
		registrars = append(registrars, w.backupRoutes)
	}
	if w.snapshotRoutes != nil {
		registrars = append(registrars, w.snapshotRoutes)
	}
	if w.restoreRoutes != nil {
		registrars = append(registrars, w.restoreRoutes)
	}
	if w.instanceSettingsRoutes != nil {
		registrars = append(registrars, w.instanceSettingsRoutes)
	}
	return registrars
}

// SetBackupRoutes sets the backup routes and must be called before router creation.
func (w *Wiki) SetBackupRoutes(r *wikibackup.Routes) {
	w.backupRoutes = r
}

// SetSnapshotRoutes sets the full-backup (snapshot) routes and must be called before router creation.
func (w *Wiki) SetSnapshotRoutes(r *wikisnapshot.Routes) {
	w.snapshotRoutes = r
}

// SetRestoreRoutes sets the live-restore routes and must be called before router creation.
func (w *Wiki) SetRestoreRoutes(r *wikirestore.Routes) {
	w.restoreRoutes = r
}

// SetInstanceSettingsRoutes sets the runtime instance-settings routes and must be called before router creation.
func (w *Wiki) SetInstanceSettingsRoutes(r *wikiinstancesettings.Routes) {
	w.instanceSettingsRoutes = r
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
		GetFaviconFile: func() string {
			cfg, err := w.branding.GetBranding()
			if err != nil || cfg == nil {
				return ""
			}
			return cfg.FaviconFile
		},
	}
}
