package branding

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corebanding "github.com/perber/wiki/internal/branding"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the branding domain.
type Routes struct {
	getBranding     *GetBrandingUseCase
	updateBranding  *UpdateBrandingUseCase
	uploadLogo      *UploadLogoUseCase
	deleteLogo      *DeleteLogoUseCase
	uploadFavicon   *UploadFaviconUseCase
	deleteFavicon   *DeleteFaviconUseCase
	brandingService *corebanding.BrandingService
	authService     *coreauth.AuthService
	log             *slog.Logger
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	GetBranding     *GetBrandingUseCase
	UpdateBranding  *UpdateBrandingUseCase
	UploadLogo      *UploadLogoUseCase
	DeleteLogo      *DeleteLogoUseCase
	UploadFavicon   *UploadFaviconUseCase
	DeleteFavicon   *DeleteFaviconUseCase
	BrandingService *corebanding.BrandingService
	AuthService     *coreauth.AuthService
	Log             *slog.Logger
}

// NewRoutes constructs the branding RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		getBranding:     cfg.GetBranding,
		updateBranding:  cfg.UpdateBranding,
		uploadLogo:      cfg.UploadLogo,
		deleteLogo:      cfg.DeleteLogo,
		uploadFavicon:   cfg.UploadFavicon,
		deleteFavicon:   cfg.DeleteFavicon,
		brandingService: cfg.BrandingService,
		authService:     cfg.AuthService,
		log:             cfg.Log,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts
	base := ctx.Base

	// Public branding API (always accessible).
	nonAuth := base.Group("/api")
	nonAuth.GET("/branding", r.handleGetBranding)

	// Branding static file server (logos, favicons) — path traversal protected.
	base.GET("/branding/:filename", r.handleServeBrandingAsset)
	base.GET("/favicon.ico", r.handleServeCurrentFavicon)

	// Auth-gated branding mutations (admin only).
	authGroup := base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)
	authGroup.PUT("/branding", authmw.RequireAdmin(opts.AuthDisabled), r.handleUpdateBranding)
	authGroup.POST("/branding/logo", authmw.RequireAdmin(opts.AuthDisabled), r.handleUploadLogo)
	authGroup.POST("/branding/favicon", authmw.RequireAdmin(opts.AuthDisabled), r.handleUploadFavicon)
	authGroup.DELETE("/branding/logo", authmw.RequireAdmin(opts.AuthDisabled), r.handleDeleteLogo)
	authGroup.DELETE("/branding/favicon", authmw.RequireAdmin(opts.AuthDisabled), r.handleDeleteFavicon)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleGetBranding(c *gin.Context) {
	out, err := r.getBranding.Execute(c.Request.Context())
	if err != nil {
		respondWithBrandingError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Config)
}

func (r *Routes) handleUpdateBranding(c *gin.Context) {
	var req struct {
		SiteName string `json:"siteName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithBrandingStatusError(c, http.StatusBadRequest, ErrCodeBrandingInvalidPayload, "Invalid payload", "invalid payload")
		return
	}
	out, err := r.updateBranding.Execute(c.Request.Context(), UpdateBrandingInput{SiteName: req.SiteName})
	if err != nil {
		respondWithBrandingError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Config)
}

func (r *Routes) handleUploadLogo(c *gin.Context) {
	constraints, err := r.brandingService.GetBranding()
	if err != nil {
		respondWithBrandingError(c, err)
		return
	}
	maxSize := constraints.BrandingConstraints.MaxLogoSize
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
	if err := c.Request.ParseMultipartForm(maxSize); err != nil {
		respondWithBrandingStatusError(c, http.StatusRequestEntityTooLarge, ErrCodeBrandingLogoTooLarge, "File too large", "file too large")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		respondWithBrandingStatusError(c, http.StatusBadRequest, ErrCodeBrandingLogoMissing, "Missing file", "missing file")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			r.log.Error("could not close logo file", "error", err)
		}
	}()
	out, err := r.uploadLogo.Execute(c.Request.Context(), UploadLogoInput{File: file, Filename: header.Filename})
	if err != nil {
		respondWithBrandingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": out.Path, "branding": out.Config})
}

func (r *Routes) handleDeleteLogo(c *gin.Context) {
	out, err := r.deleteLogo.Execute(c.Request.Context())
	if err != nil {
		respondWithBrandingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"branding": out.Config})
}

func (r *Routes) handleUploadFavicon(c *gin.Context) {
	constraints, err := r.brandingService.GetBranding()
	if err != nil {
		respondWithBrandingError(c, err)
		return
	}
	maxSize := constraints.BrandingConstraints.MaxFaviconSize
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
	if err := c.Request.ParseMultipartForm(maxSize); err != nil {
		respondWithBrandingStatusError(c, http.StatusRequestEntityTooLarge, ErrCodeBrandingFaviconTooLarge, "File too large", "file too large")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		respondWithBrandingStatusError(c, http.StatusBadRequest, ErrCodeBrandingFaviconMissing, "Missing file", "missing file")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			r.log.Error("could not close favicon file", "error", err)
		}
	}()
	out, err := r.uploadFavicon.Execute(c.Request.Context(), UploadFaviconInput{File: file, Filename: header.Filename})
	if err != nil {
		respondWithBrandingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": out.Path, "branding": out.Config})
}

func (r *Routes) handleDeleteFavicon(c *gin.Context) {
	out, err := r.deleteFavicon.Execute(c.Request.Context())
	if err != nil {
		respondWithBrandingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"branding": out.Config})
}

func (r *Routes) handleServeBrandingAsset(c *gin.Context) {
	cfg, err := r.brandingService.GetBranding()
	if err != nil {
		r.log.Error("failed to get branding constraints", "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	cleanPath, status := r.resolveBrandingAssetPath(c.Param("filename"), cfg)
	if status != http.StatusOK {
		c.Status(status)
		return
	}

	disableClientCache(c)
	c.File(cleanPath)
}

func (r *Routes) handleServeCurrentFavicon(c *gin.Context) {
	cfg, err := r.brandingService.GetBranding()
	if err != nil {
		r.log.Error("failed to get branding config", "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	if cfg.FaviconFile != "" {
		cleanPath, status := r.resolveBrandingAssetPath(cfg.FaviconFile, cfg)
		if status == http.StatusOK {
			disableClientCache(c)
			c.File(cleanPath)
			return
		}
		if status == http.StatusInternalServerError {
			c.Status(status)
			return
		}
	}

	disableClientCache(c)
	c.Data(http.StatusOK, "image/svg+xml", []byte(httpinternal.DefaultFaviconSVG))
}

func containsInvalidBrandingAssetPath(filename string) bool {
	return strings.Contains(filename, "..") ||
		strings.Contains(filename, "/") ||
		strings.Contains(filename, "\\") ||
		filepath.IsAbs(filename) ||
		filepath.VolumeName(filename) != "" ||
		strings.Contains(filename, "\x00")
}

func (r *Routes) resolveBrandingAssetPath(filename string, cfg *corebanding.BrandingConfigResponse) (string, int) {
	// Prevent path traversal or poisoned config values.
	if containsInvalidBrandingAssetPath(filename) {
		return "", http.StatusForbidden
	}

	allowedExts := make(map[string]bool)
	for _, ext := range cfg.BrandingConstraints.LogoExts {
		allowedExts[ext] = true
	}
	for _, ext := range cfg.BrandingConstraints.FaviconExts {
		allowedExts[ext] = true
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExts[ext] {
		return "", http.StatusForbidden
	}

	brandingDir := r.brandingService.GetBrandingAssetsDir()
	filePath := filepath.Join(brandingDir, filename)
	cleanPath := filepath.Clean(filePath)
	cleanBrandingDir := filepath.Clean(brandingDir)

	rel, err := filepath.Rel(cleanBrandingDir, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", http.StatusForbidden
	}

	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return "", http.StatusNotFound
	} else if err != nil {
		r.log.Error("error checking branding file", "error", err, "path", cleanPath)
		return "", http.StatusInternalServerError
	}

	return cleanPath, http.StatusOK
}

func disableClientCache(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", time.Unix(0, 0).UTC().Format(http.TimeFormat))
}
