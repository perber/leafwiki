package avatar

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	coreavatar "github.com/perber/wiki/internal/avatar"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the avatar domain.
type Routes struct {
	uploadAvatar  *UploadAvatarUseCase
	deleteAvatar  *DeleteAvatarUseCase
	avatarService *coreavatar.AvatarService
	authService   *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	UploadAvatar  *UploadAvatarUseCase
	DeleteAvatar  *DeleteAvatarUseCase
	AvatarService *coreavatar.AvatarService
	AuthService   *coreauth.AuthService
}

// NewRoutes constructs the avatar RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		uploadAvatar:  cfg.UploadAvatar,
		deleteAvatar:  cfg.DeleteAvatar,
		avatarService: cfg.AvatarService,
		authService:   cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar. Any authenticated user manages
// only their own avatar — there is no admin override (self-service only).
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts
	base := ctx.Base

	// Public avatar static file server — unauthenticated, path-traversal
	// protected. A missing avatar returns a bare 404 so the frontend's
	// AvatarImage -> AvatarFallback degrades to initials automatically.
	base.GET("/avatars/:userId", r.handleServeAvatar)

	authGroup := base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)
	authGroup.POST("/user/avatar", r.handleUploadAvatar)
	authGroup.DELETE("/user/avatar", r.handleDeleteAvatar)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleUploadAvatar(c *gin.Context) {
	user := authmw.MustGetUser(c)

	file, header, err := httpinternal.ParseUploadedFile(c, coreavatar.MaxUploadSize, "file")
	if err != nil {
		if errors.Is(err, httpinternal.ErrUploadTooLarge) {
			respondWithAvatarStatusError(c, http.StatusRequestEntityTooLarge, ErrCodeAvatarUploadFailed, "File too large", "file too large")
		} else {
			respondWithAvatarStatusError(c, http.StatusBadRequest, ErrCodeAvatarUploadFailed, "Missing file", "missing file")
		}
		return
	}
	defer func() { _ = file.Close() }()

	if err := r.uploadAvatar.Execute(c.Request.Context(), user.ID, file, header.Filename); err != nil {
		respondWithAvatarError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

func (r *Routes) handleDeleteAvatar(c *gin.Context) {
	user := authmw.MustGetUser(c)

	if err := r.deleteAvatar.Execute(c.Request.Context(), user.ID); err != nil {
		respondWithAvatarError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

func (r *Routes) handleServeAvatar(c *gin.Context) {
	userID := c.Param("userId")

	// Defense in depth against path traversal — AvatarService.AvatarPath
	// already rejects unsafe ids internally, but reject obviously unsafe
	// values before even asking.
	if strings.ContainsAny(userID, "/\\") || strings.Contains(userID, "..") {
		c.Status(http.StatusNotFound)
		return
	}

	path, found := r.avatarService.AvatarPath(userID)
	if !found {
		c.Status(http.StatusNotFound)
		return
	}

	disableClientCache(c)
	c.File(path)
}

func disableClientCache(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", time.Unix(0, 0).UTC().Format(http.TimeFormat))
}
