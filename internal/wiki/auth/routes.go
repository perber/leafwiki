package auth

import (
	"errors"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
	"github.com/perber/wiki/internal/http/middleware/utils"
)

// Routes is the RouteRegistrar for the auth domain.
type Routes struct {
	login            *LoginUseCase
	logout           *LogoutUseCase
	refreshToken     *RefreshTokenUseCase
	createUser       *CreateUserUseCase
	updateUser       *UpdateUserUseCase
	changeOwnPassword *ChangeOwnPasswordUseCase
	deleteUser       *DeleteUserUseCase
	getUsers         *GetUsersUseCase
	getUserByID      *GetUserByIDUseCase
	authService      *coreauth.AuthService
}

// RoutesConfig holds the dependencies to build an auth Routes instance.
type RoutesConfig struct {
	Login             *LoginUseCase
	Logout            *LogoutUseCase
	RefreshToken      *RefreshTokenUseCase
	CreateUser        *CreateUserUseCase
	UpdateUser        *UpdateUserUseCase
	ChangeOwnPassword *ChangeOwnPasswordUseCase
	DeleteUser        *DeleteUserUseCase
	GetUsers          *GetUsersUseCase
	GetUserByID       *GetUserByIDUseCase
	AuthService       *coreauth.AuthService
}

// NewRoutes constructs the auth RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		login:            cfg.Login,
		logout:           cfg.Logout,
		refreshToken:     cfg.RefreshToken,
		createUser:       cfg.CreateUser,
		updateUser:       cfg.UpdateUser,
		changeOwnPassword: cfg.ChangeOwnPassword,
		deleteUser:       cfg.DeleteUser,
		getUsers:         cfg.GetUsers,
		getUserByID:      cfg.GetUserByID,
		authService:      cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	loginRateLimiter := security.NewRateLimiter(10, 5*time.Minute, true)
	refreshRateLimiter := security.NewRateLimiter(30, time.Minute, false)

	nonAuth := ctx.Base.Group("/api")
	nonAuth.POST("/auth/login", loginRateLimiter, r.handleLogin(ctx))
	nonAuth.POST("/auth/refresh-token", refreshRateLimiter, r.handleRefreshToken(ctx))

	// Config endpoint also lives here as it issues the CSRF cookie.
	nonAuth.GET("/config", r.handleConfig(ctx))

	authGroup := ctx.Base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)

	authGroup.POST("/auth/logout", r.handleLogout(ctx))

	authGroup.POST("/users", authmw.RequireAdmin(opts.AuthDisabled), r.handleCreateUser)
	authGroup.GET("/users", authmw.RequireAdmin(opts.AuthDisabled), r.handleGetUsers)
	authGroup.PUT("/users/:id", authmw.RequireSelfOrAdmin(opts.AuthDisabled), r.handleUpdateUser)
	authGroup.DELETE("/users/:id", authmw.RequireAdmin(opts.AuthDisabled), r.handleDeleteUser)

	if !opts.AuthDisabled {
		authGroup.PUT("/users/me/password", r.handleChangeOwnPassword)
	}
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func writeAuthCookieError(c *gin.Context, err error, httpsMsg, internalMsg, logMsg string) {
	if errors.Is(err, utils.ErrHTTPSRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": httpsMsg})
		return
	}
	slog.Default().Error(logMsg, "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": internalMsg})
}

func (r *Routes) handleConfig(ctx httpinternal.RouterContext) gin.HandlerFunc {
	opts := ctx.Opts
	return func(c *gin.Context) {
		if _, err := ctx.CSRFCookie.Issue(c); err != nil {
			writeAuthCookieError(c, err,
				"HTTPS is required for auth cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.",
				"Failed to issue CSRF cookie",
				"failed to issue config CSRF cookie",
			)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"publicAccess":            opts.PublicAccess,
			"hideLinkMetadataSection": opts.HideLinkMetadataSection,
			"authDisabled":            opts.AuthDisabled,
			"basePath":                opts.BasePath,
			"maxAssetUploadSizeBytes": opts.MaxAssetUploadSizeBytes,
			"enableRevision":          opts.EnableRevision,
			"enableLinkRefactor":      opts.EnableLinkRefactor,
		})
	}
}

func (r *Routes) handleLogin(rctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Identifier string `json:"identifier" binding:"required"`
			Password   string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid login payload"})
			return
		}
		out, err := r.login.Execute(c.Request.Context(), LoginInput{
			Identifier: req.Identifier, Password: req.Password,
		})
		if err != nil {
			respondWithAuthError(c, err)
			return
		}
		if _, err := rctx.CSRFCookie.Issue(c); err != nil {
			writeAuthCookieError(c, err,
				"HTTPS is required for login cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.",
				"Failed to issue CSRF cookie",
				"failed to issue login CSRF cookie",
			)
			return
		}
		if err := rctx.AuthCookies.Set(c, out.Token.Token, out.Token.RefreshToken); err != nil {
			if errors.Is(err, utils.ErrHTTPSRequired) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "HTTPS is required for auth cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.",
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to set authentication cookies"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Login successful", "user": out.Token.User})
	}
}

func (r *Routes) handleLogout(rctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		refreshToken, _ := rctx.AuthCookies.ReadRefresh(c)
		if refreshToken != "" {
			if err := r.logout.Execute(c.Request.Context(), LogoutInput{RefreshToken: refreshToken}); err != nil {
				log.Printf("[INFO] Unable to revoke the refresh token: %v", err)
			}
		}
		if err := rctx.AuthCookies.Clear(c); err != nil {
			log.Printf("[INFO] Unable to clear auth cookies: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := rctx.CSRFCookie.Clear(c); err != nil {
			log.Printf("[INFO] Unable to clear CSRF cookie: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to clear CSRF cookie"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
	}
}

func (r *Routes) handleRefreshToken(rctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		rt, err := rctx.AuthCookies.ReadRefresh(c)
		if err != nil || rt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid refresh token"})
			return
		}
		out, err := r.refreshToken.Execute(c.Request.Context(), RefreshTokenInput{RefreshToken: rt})
		if err != nil {
			respondWithAuthError(c, err)
			return
		}
		if _, err := rctx.CSRFCookie.Issue(c); err != nil {
			writeAuthCookieError(c, err,
				"HTTPS is required for auth cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.",
				"Failed to issue CSRF cookie",
				"failed to issue refresh CSRF cookie",
			)
			return
		}
		if err := rctx.AuthCookies.Set(c, out.Token.Token, out.Token.RefreshToken); err != nil {
			if errors.Is(err, utils.ErrHTTPSRequired) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "HTTPS is required for auth cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set authentication cookies"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Token refreshed", "user": out.Token.User})
	}
}

func (r *Routes) handleCreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	out, err := r.createUser.Execute(c.Request.Context(), CreateUserInput{
		Username: req.Username, Email: req.Email, Password: req.Password, Role: req.Role,
	})
	if err != nil {
		respondWithAuthError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out.User)
}

func (r *Routes) handleGetUsers(c *gin.Context) {
	out, err := r.getUsers.Execute(c.Request.Context())
	if err != nil {
		respondWithAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.Users)
}

func (r *Routes) handleUpdateUser(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	out, err := r.updateUser.Execute(c.Request.Context(), UpdateUserInput{
		ID: id, Username: req.Username, Email: req.Email, Password: req.Password, Role: req.Role,
	})
	if err != nil {
		respondWithAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, out.User)
}

func (r *Routes) handleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	if err := r.deleteUser.Execute(c.Request.Context(), DeleteUserInput{ID: id}); err != nil {
		respondWithAuthError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Routes) handleChangeOwnPassword(c *gin.Context) {
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := r.changeOwnPassword.Execute(c.Request.Context(), ChangeOwnPasswordInput{
		UserID: user.ID, OldPassword: req.OldPassword, NewPassword: req.NewPassword,
	}); err != nil {
		respondWithAuthError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
