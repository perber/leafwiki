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

const (
	httpsRequiredLogMsg        = "https required for auth cookies use allow insecure"
	httpsRequiredUserMsg       = "HTTPS is required for auth cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups."
	errInvalidRequestUserMsg   = "Invalid request"
	errInvalidRequestLogMsg    = "invalid request"
	errFailedToIssueCSRFCookie = "Failed to issue CSRF cookie"
)

// DisableRefreshTokenRateLimit can be set via ldflags for E2E/debug builds.
var DisableRefreshTokenRateLimit = "false"

// Routes is the RouteRegistrar for the auth domain.
type Routes struct {
	login             *LoginUseCase
	completeTOTPLogin *CompleteTOTPLoginUseCase
	logout            *LogoutUseCase
	refreshToken      *RefreshTokenUseCase
	createUser        *CreateUserUseCase
	updateUser        *UpdateUserUseCase
	changeOwnPassword *ChangeOwnPasswordUseCase
	deleteUser        *DeleteUserUseCase
	getUsers          *GetUsersUseCase
	getUserByID       *GetUserByIDUseCase
	startTOTPSetup    *StartTOTPSetupUseCase
	confirmTOTPSetup  *ConfirmTOTPSetupUseCase
	disableTOTP       *DisableTOTPUseCase
	getTOTPStatus     *GetTOTPStatusUseCase
	authService       *coreauth.AuthService
}

// RoutesConfig holds the dependencies to build an auth Routes instance.
type RoutesConfig struct {
	Login             *LoginUseCase
	CompleteTOTPLogin *CompleteTOTPLoginUseCase
	Logout            *LogoutUseCase
	RefreshToken      *RefreshTokenUseCase
	CreateUser        *CreateUserUseCase
	UpdateUser        *UpdateUserUseCase
	ChangeOwnPassword *ChangeOwnPasswordUseCase
	DeleteUser        *DeleteUserUseCase
	GetUsers          *GetUsersUseCase
	GetUserByID       *GetUserByIDUseCase
	StartTOTPSetup    *StartTOTPSetupUseCase
	ConfirmTOTPSetup  *ConfirmTOTPSetupUseCase
	DisableTOTP       *DisableTOTPUseCase
	GetTOTPStatus     *GetTOTPStatusUseCase
	AuthService       *coreauth.AuthService
}

// NewRoutes constructs the auth RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		login:             cfg.Login,
		completeTOTPLogin: cfg.CompleteTOTPLogin,
		logout:            cfg.Logout,
		refreshToken:      cfg.RefreshToken,
		createUser:        cfg.CreateUser,
		updateUser:        cfg.UpdateUser,
		changeOwnPassword: cfg.ChangeOwnPassword,
		deleteUser:        cfg.DeleteUser,
		getUsers:          cfg.GetUsers,
		getUserByID:       cfg.GetUserByID,
		startTOTPSetup:    cfg.StartTOTPSetup,
		confirmTOTPSetup:  cfg.ConfirmTOTPSetup,
		disableTOTP:       cfg.DisableTOTP,
		getTOTPStatus:     cfg.GetTOTPStatus,
		authService:       cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	loginRateLimiter := security.NewRateLimiter(10, 5*time.Minute, true)

	nonAuth := ctx.Base.Group("/api")
	nonAuth.POST("/auth/login", loginRateLimiter, r.handleLogin(ctx))
	// Shares loginRateLimiter's per-IP budget with /auth/login: both steps of the
	// same handshake draw from one bucket, since both are exposed to credential/code
	// guessing before a session exists.
	nonAuth.POST("/auth/login/totp", loginRateLimiter, r.handleLoginTOTP(ctx))
	if DisableRefreshTokenRateLimit == "true" {
		nonAuth.POST("/auth/refresh-token", r.handleRefreshToken(ctx))
	} else {
		refreshRateLimiter := security.NewRateLimiter(30, time.Minute, false)
		nonAuth.POST("/auth/refresh-token", refreshRateLimiter, r.handleRefreshToken(ctx))
	}

	// Config endpoint also lives here as it issues the CSRF cookie.
	nonAuth.GET("/config", r.handleConfig(ctx))

	// /auth/me uses optional auth so that unauthenticated callers get 200+null
	// instead of 401, which would cause browsers behind a Basic Auth reverse
	// proxy to discard their cached credentials.
	meGroup := ctx.Base.Group("/api")
	meGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.OptionalAuth(r.authService, ctx.AuthCookies),
	)
	meGroup.GET("/auth/me", r.handleMe)

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

		// Setup/confirm/disable all guess a secret (password, TOTP code, or
		// recovery code); share one rate-limit budget separate from the
		// pre-auth login endpoints above.
		totpSetupRateLimiter := security.NewRateLimiter(10, 5*time.Minute, true)
		authGroup.POST("/users/me/totp/setup/start", totpSetupRateLimiter, r.handleStartTOTPSetup)
		authGroup.POST("/users/me/totp/setup/confirm", totpSetupRateLimiter, r.handleConfirmTOTPSetup(ctx))
		authGroup.POST("/users/me/totp/disable", totpSetupRateLimiter, r.handleDisableTOTP(ctx))
		authGroup.GET("/users/me/totp/status", r.handleTOTPStatus)
	}
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func writeAuthCookieError(c *gin.Context, err error, httpsMsg, internalMsg, logMsg string) {
	if errors.Is(err, utils.ErrHTTPSRequired) {
		respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthCookieFailed, httpsMsg, httpsRequiredLogMsg)
		return
	}
	slog.Default().Error(logMsg, "error", err)
	respondWithAuthStatusError(c, http.StatusInternalServerError, ErrCodeAuthInternalError, internalMsg, "failed to issue auth cookie")
}

func (r *Routes) handleConfig(ctx httpinternal.RouterContext) gin.HandlerFunc {
	opts := ctx.Opts
	return func(c *gin.Context) {
		if _, err := ctx.CSRFCookie.Issue(c); err != nil {
			writeAuthCookieError(c, err,
				httpsRequiredUserMsg,
				errFailedToIssueCSRFCookie,
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
			"gitBackupEnabled":        opts.GitBackupEnabled,
			"httpRemoteUserEnabled":   opts.HTTPRemoteUser.Enabled,
			"loginUrl":                opts.LoginURL,
			"logoutUrl":               opts.LogoutURL,
			"userManagementUrl":       opts.UserManagementURL,
		})
	}
}

// handleMe returns the currently authenticated user or null.
// Uses TryGetUser (not MustGetUser) so unauthenticated callers receive 200+null
// instead of 401, avoiding the Basic Auth credential-reset issue (RFC 9110 §15.5.2).
// Cache headers prevent reverse proxies from caching the identity response.
func (r *Routes) handleMe(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", time.Unix(0, 0).UTC().Format(http.TimeFormat))

	user := authmw.TryGetUser(c)
	if user == nil {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"role":        user.Role,
		"totpEnabled": user.TOTPEnabled,
	})
}

func (r *Routes) handleLogin(rctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Identifier string `json:"identifier" binding:"required"`
			Password   string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidPayload, "Invalid login payload", "invalid login payload")
			return
		}
		out, err := r.login.Execute(c.Request.Context(), LoginInput{
			Identifier: req.Identifier, Password: req.Password,
		})
		if err != nil {
			respondWithAuthError(c, err)
			return
		}
		if out.Token.RequiresTOTP {
			// Password verified, but no cookies may be issued until the TOTP
			// step completes via POST /auth/login/totp.
			c.JSON(http.StatusOK, gin.H{
				"requiresTotp":        true,
				"loginChallengeToken": out.Token.LoginChallengeToken,
			})
			return
		}
		if _, err := rctx.CSRFCookie.Issue(c); err != nil {
			writeAuthCookieError(c, err,
				"HTTPS is required for login cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.",
				errFailedToIssueCSRFCookie,
				"failed to issue login CSRF cookie",
			)
			return
		}
		if err := rctx.AuthCookies.Set(c, out.Token.Token, out.Token.RefreshToken); err != nil {
			if errors.Is(err, utils.ErrHTTPSRequired) {
				respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthCookieFailed,
					httpsRequiredUserMsg,
					httpsRequiredLogMsg)
				return
			}
			respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthCookieFailed, "Failed to set authentication cookies", "failed to set authentication cookies")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":              "Login successful",
			"user":                 out.Token.User,
			"accessTokenExpiresAt": out.Token.AccessTokenExpiresAt,
		})
	}
}

// handleLoginTOTP completes a login handshake started by handleLogin when the
// account has TOTP enabled. Only on a valid TOTP/recovery code are auth
// cookies issued; the challenge token itself is single-use and short-lived.
func (r *Routes) handleLoginTOTP(rctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			LoginChallengeToken string `json:"loginChallengeToken" binding:"required"`
			Code                string `json:"code" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidPayload, "Invalid login payload", "invalid login payload")
			return
		}
		out, err := r.completeTOTPLogin.Execute(c.Request.Context(), CompleteTOTPLoginInput{
			LoginChallengeToken: req.LoginChallengeToken, Code: req.Code,
		})
		if err != nil {
			respondWithAuthError(c, err)
			return
		}
		if _, err := rctx.CSRFCookie.Issue(c); err != nil {
			writeAuthCookieError(c, err,
				"HTTPS is required for login cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.",
				errFailedToIssueCSRFCookie,
				"failed to issue login CSRF cookie",
			)
			return
		}
		if err := rctx.AuthCookies.Set(c, out.Token.Token, out.Token.RefreshToken); err != nil {
			if errors.Is(err, utils.ErrHTTPSRequired) {
				respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthCookieFailed,
					httpsRequiredUserMsg,
					httpsRequiredLogMsg)
				return
			}
			respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthCookieFailed, "Failed to set authentication cookies", "failed to set authentication cookies")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":              "Login successful",
			"user":                 out.Token.User,
			"accessTokenExpiresAt": out.Token.AccessTokenExpiresAt,
		})
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
			respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthCookieFailed, "Failed to clear authentication cookies", "failed to clear authentication cookies")
			return
		}
		if err := rctx.CSRFCookie.Clear(c); err != nil {
			log.Printf("[INFO] Unable to clear CSRF cookie: %v", err)
			respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthCsrfFailed, "Failed to clear CSRF cookie", "failed to clear csrf cookie")
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
	}
}

func (r *Routes) handleRefreshToken(rctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		rt, err := rctx.AuthCookies.ReadRefresh(c)
		if err != nil || rt == "" {
			respondWithAuthStatusError(c, http.StatusUnprocessableEntity, ErrCodeAuthInvalidRefreshToken, "Missing or invalid refresh token", "missing or invalid refresh token")
			return
		}
		out, err := r.refreshToken.Execute(c.Request.Context(), RefreshTokenInput{RefreshToken: rt})
		if err != nil {
			respondWithAuthError(c, err)
			return
		}
		if _, err := rctx.CSRFCookie.Issue(c); err != nil {
			writeAuthCookieError(c, err,
				httpsRequiredUserMsg,
				errFailedToIssueCSRFCookie,
				"failed to issue refresh CSRF cookie",
			)
			return
		}
		if err := rctx.AuthCookies.Set(c, out.Token.Token, out.Token.RefreshToken); err != nil {
			if errors.Is(err, utils.ErrHTTPSRequired) {
				respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthCookieFailed,
					httpsRequiredUserMsg,
					httpsRequiredLogMsg)
				return
			}
			respondWithAuthStatusError(c, http.StatusInternalServerError, ErrCodeAuthCookieFailed, "Failed to set authentication cookies", "failed to set authentication cookies")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":              "Token refreshed",
			"user":                 out.Token.User,
			"accessTokenExpiresAt": out.Token.AccessTokenExpiresAt,
		})
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
		respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
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
	requester := authmw.MustGetUser(c)
	if requester == nil {
		return
	}
	var req struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
		return
	}
	out, err := r.updateUser.Execute(c.Request.Context(), UpdateUserInput{
		ID: id, Username: req.Username, Email: req.Email, Password: req.Password, Role: req.Role,
		RequesterIsAdmin: requester.HasRole(coreauth.RoleAdmin),
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
		respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
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

// handleStartTOTPSetup begins TOTP enrollment for the current user: verifies
// their current password and returns a fresh, not-yet-enabled secret for the
// frontend to render as a QR code (and show for manual entry).
func (r *Routes) handleStartTOTPSetup(c *gin.Context) {
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	var req struct {
		CurrentPassword string `json:"currentPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
		return
	}
	out, err := r.startTOTPSetup.Execute(c.Request.Context(), StartTOTPSetupInput{
		UserID: user.ID, CurrentPassword: req.CurrentPassword,
	})
	if err != nil {
		respondWithAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"secret":     out.Secret,
		"otpAuthUrl": out.OTPAuthURL,
	})
}

// handleConfirmTOTPSetup completes TOTP enrollment: verifies a code against
// the pending secret from handleStartTOTPSetup, enables TOTP, and returns the
// one-time plaintext recovery codes. Every other session for the user is
// revoked; the session making this request is identified via its own refresh
// cookie and left intact.
func (r *Routes) handleConfirmTOTPSetup(rctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authmw.MustGetUser(c)
		if user == nil {
			return
		}
		var req struct {
			Code string `json:"code" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
			return
		}
		refreshToken, _ := rctx.AuthCookies.ReadRefresh(c)
		out, err := r.confirmTOTPSetup.Execute(c.Request.Context(), ConfirmTOTPSetupInput{
			UserID: user.ID, Code: req.Code, CurrentRefreshToken: refreshToken,
		})
		if err != nil {
			respondWithAuthError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"recoveryCodes": out.RecoveryCodes,
		})
	}
}

// handleDisableTOTP disables TOTP for the current user after verifying their
// current password plus a TOTP or recovery code. Every other session for the
// user is revoked; the session making this request is left intact.
func (r *Routes) handleDisableTOTP(rctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authmw.MustGetUser(c)
		if user == nil {
			return
		}
		var req struct {
			CurrentPassword string `json:"currentPassword" binding:"required"`
			Code            string `json:"code" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondWithAuthStatusError(c, http.StatusBadRequest, ErrCodeAuthInvalidRequest, errInvalidRequestUserMsg, errInvalidRequestLogMsg)
			return
		}
		refreshToken, _ := rctx.AuthCookies.ReadRefresh(c)
		if err := r.disableTOTP.Execute(c.Request.Context(), DisableTOTPInput{
			UserID: user.ID, CurrentPassword: req.CurrentPassword, Code: req.Code, CurrentRefreshToken: refreshToken,
		}); err != nil {
			respondWithAuthError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// handleTOTPStatus returns the current user's own TOTP status. Never exposes
// the secret or recovery codes themselves.
func (r *Routes) handleTOTPStatus(c *gin.Context) {
	user := authmw.MustGetUser(c)
	if user == nil {
		return
	}
	out, err := r.getTOTPStatus.Execute(c.Request.Context(), GetTOTPStatusInput{UserID: user.ID})
	if err != nil {
		respondWithAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":                out.Enabled,
		"recoveryCodesRemaining": out.RecoveryCodesRemaining,
	})
}
