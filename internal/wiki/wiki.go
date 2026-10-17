package wiki

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/perber/wiki/internal/avatar"
	"github.com/perber/wiki/internal/branding"
	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/email"
	"github.com/perber/wiki/internal/core/ignore"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/favorites"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/properties"
	"github.com/perber/wiki/internal/search"
	"github.com/perber/wiki/internal/tags"
	"github.com/perber/wiki/internal/usersettings"
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
	wikiproperties "github.com/perber/wiki/internal/wiki/properties"
	wikirestore "github.com/perber/wiki/internal/wiki/restore"
	wikiresync "github.com/perber/wiki/internal/wiki/resync"
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
	wikisearch "github.com/perber/wiki/internal/wiki/search"
	wikisnapshot "github.com/perber/wiki/internal/wiki/snapshot"
	wikitags "github.com/perber/wiki/internal/wiki/tags"
	wikiusersettings "github.com/perber/wiki/internal/wiki/usersettings"
)

type Wiki struct {
	tree              *tree.TreeService
	slug              *tree.SlugService
	auth              *auth.AuthService
	userResolver      *auth.UserResolver
	user              *auth.UserService
	apiKeys           *auth.APIKeyService
	totp              *auth.TOTPService
	emailTokenStore   *auth.EmailTokenStore
	emailTokenService *auth.EmailTokenService
	asset             *assets.AssetService
	branding          *branding.BrandingService
	avatar            *avatar.AvatarService
	searchIndex       *search.SQLiteIndex
	status            *search.IndexingStatus
	storageDir        string

	// Domain route registrars (populated by NewWiki).
	pagesRoutes            *wikipages.Routes
	authRoutes             *wikiauth.Routes
	assetsRoutes           *wikiassets.Routes
	revisionsRoutes        *wikirevisions.Routes
	searchRoutes           *wikisearch.Routes
	linksRoutes            *wikilinks.Routes
	tagsRoutes             *wikitags.Routes
	propertiesRoutes       *wikiproperties.Routes
	brandingRoutes         *wikibranding.Routes
	avatarRoutes           *wikiavatar.Routes
	apiKeysRoutes          *wikiapikeys.Routes
	importerRoutes         *wikiimporter.Routes
	healthRoutes           *wikihealth.Routes
	revision               *revision.Service
	links                  *links.LinkService
	tags                   *tags.TagsService
	props                  *properties.PropertiesService
	favorites              *favorites.FavoritesStore
	userSettings           *usersettings.UserSettingsService
	userSettingsRoutes     *wikiusersettings.Routes
	backupRoutes           *wikibackup.Routes
	snapshotRoutes         *wikisnapshot.Routes
	restoreRoutes          *wikirestore.Routes
	instanceSettingsRoutes *wikiinstancesettings.Routes
	resyncRoutes           *wikiresync.Routes
	resyncJob              *wikiresync.ResyncJob
	ignoreCache            *ignore.Cache
	reloadMu               sync.Mutex
	reloadWG               sync.WaitGroup
	shutdownCtx            context.Context
	shutdownCancel         context.CancelFunc
	log                    *slog.Logger
	metrics                *httpmetrics.HTTPMetrics
}

const SYSTEM_USER_ID = "system"

type WikiOptions struct {
	StorageDir              string        // Path to storage directory
	AdminUsername           string        // Initial admin username (optional; defaults to "admin")
	AdminEmail              string        // Initial admin email (optional; defaults to "admin@localhost")
	AdminPassword           string        // Initial admin password
	JWTSecret               string        // JWT secret for authentication
	AccessTokenTimeout      time.Duration // Access token timeout duration
	RefreshTokenTimeout     time.Duration // Refresh token timeout duration
	AuthDisabled            bool          // Whether authentication is disabled
	EnableRevision          bool          // Whether revision recording/storage is enabled
	EnableAPIKeyManagement  bool          // Whether the experimental API key management feature is enabled
	MaxRevisionHistory      int           // Max revisions kept per page; 0 = unlimited
	EditorLimit             int           // Max admin+editor users allowed; 0 = unlimited
	MaxAssetUploadSizeBytes int64         // Maximum allowed size in bytes for asset/import uploads; 0 = default
	RevisionCoalesceWindow  time.Duration // Window for coalescing rapid successive saves; 0 = disabled
	TOTPEncryptionKey       string        // Key used to encrypt per-user TOTP secrets at rest; empty disables TOTP self-service
	SMTP                    email.Config  // SMTP config for password-reset/invite email; SMTP.Enabled()==false disables the feature entirely
	Metrics                 *httpmetrics.HTTPMetrics
}

func NewWiki(options *WikiOptions) (*Wiki, error) {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	w := &Wiki{
		storageDir:     options.StorageDir,
		log:            slog.Default().With("component", "Wiki"),
		resyncJob:      wikiresync.NewResyncJob(),
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		metrics:        options.Metrics,
	}
	if err := w.initAuth(options); err != nil {
		return nil, err
	}
	if err := w.initEmail(options); err != nil {
		return nil, err
	}
	if err := w.initCoreServices(options); err != nil {
		return nil, err
	}
	if err := w.initLinkService(); err != nil {
		return nil, err
	}
	if err := w.initTagsService(); err != nil {
		return nil, err
	}
	if err := w.initPropertiesService(); err != nil {
		return nil, err
	}
	if err := w.initFavoritesService(); err != nil {
		return nil, err
	}
	if err := w.initUserSettingsService(); err != nil {
		return nil, err
	}
	w.bootstrapTagsAndProperties()
	if err := w.initSearch(); err != nil {
		return nil, err
	}
	if err := w.initBranding(); err != nil {
		return nil, err
	}
	if err := w.initAvatarService(); err != nil {
		return nil, err
	}
	// Welcome page must exist before the revision service starts recording.
	if err := w.EnsureWelcomePage(); err != nil {
		return nil, err
	}
	if options.EnableRevision {
		w.revision = revision.NewService(w.storageDir, w.tree, w.log,
			revision.ServiceOptions{
				MaxRevisions:   options.MaxRevisionHistory,
				CoalesceWindow: options.RevisionCoalesceWindow,
			})
		w.ensureBaselineRevisions()
	}
	w.metrics.RegisterRuntimeStats(runtimeStatsSource{w: w})
	w.buildRoutes(options)
	return w, nil
}

// AuthService returns the authentication service.
func (w *Wiki) AuthService() *auth.AuthService {
	return w.auth
}

// BrandingService returns the branding service.
func (w *Wiki) BrandingService() *branding.BrandingService {
	return w.branding
}

// TOTPService returns the TOTP service, or nil if no --totp-encryption-key /
// LEAFWIKI_TOTP_ENCRYPTION_KEY was configured (TOTP self-service is then
// unavailable until an operator sets one).
func (w *Wiki) TOTPService() *auth.TOTPService {
	return w.totp
}

// ─── Service getters (test infrastructure) ───────────────────────────────────

func (w *Wiki) GetStorageDir() string {
	return w.storageDir
}

// UserService returns the current *auth.UserService: AuthService's live,
// restore-swap-tracking one when auth is enabled (see AuthService.UserService),
// falling back to the UserService constructed in initAuth when auth is
// disabled — in that mode there's no AuthService and, per
// internal/restore/manager.go's AuthService-nil guards, a live restore never
// swaps this store either, so the fallback is never stale.
func (w *Wiki) UserService() *auth.UserService {
	if w.auth != nil {
		return w.auth.UserService()
	}
	return w.user
}

// UserResolver returns the author-label resolver shared across page/tag/
// property routes and the user-management use cases.
func (w *Wiki) UserResolver() *auth.UserResolver {
	return w.userResolver
}

// APIKeyService returns the API key service used for Bearer authentication.
func (w *Wiki) APIKeyService() *auth.APIKeyService {
	return w.apiKeys
}

// Favorites returns the favorites store, e.g. for wiring into restore.Config.
func (w *Wiki) Favorites() *favorites.FavoritesStore {
	return w.favorites
}

// UserSettingsService returns the user settings service, e.g. for wiring
// into restore.Config.
func (w *Wiki) UserSettingsService() *usersettings.UserSettingsService {
	return w.userSettings
}

// EmailTokenService returns the password-reset/invite token service, or nil
// if SMTP isn't configured or auth is disabled (see initEmail). Every use
// case that resolves this via the func()-based pattern (like UserService)
// nil-checks it and returns ErrEmailDisabled.
func (w *Wiki) EmailTokenService() *auth.EmailTokenService {
	return w.emailTokenService
}

func (w *Wiki) Close() error {
	w.shutdownCancel() // signal in-flight reloads to abort
	w.reloadWG.Wait()  // drain goroutines before closing stores
	w.status.Finish()
	if w.auth != nil {
		// When auth is enabled, AuthService owns both the session store and user store.
		if err := w.auth.Close(); err != nil {
			return err
		}
	} else if w.user != nil {
		if err := w.user.Close(); err != nil {
			return err
		}
	}

	if w.apiKeys != nil {
		if err := w.apiKeys.Close(); err != nil {
			return err
		}
	}

	if w.emailTokenService != nil {
		w.emailTokenService.Close() // drains in-flight fire-and-forget password-reset sends
	}
	if w.emailTokenStore != nil {
		if err := w.emailTokenStore.Close(); err != nil {
			w.log.Error("error closing email token store", "error", err)
		}
	}

	if w.links != nil {
		if err := w.links.Close(); err != nil {
			w.log.Error("error closing links", "error", err)
		}
	}

	if w.userSettings != nil {
		if err := w.userSettings.Close(); err != nil {
			w.log.Error("error closing user settings store", "error", err)
		}
	}

	if w.favorites != nil {
		if err := w.favorites.Close(); err != nil {
			w.log.Error("error closing favorites store", "error", err)
		}
	}

	return w.searchIndex.Close()
}
