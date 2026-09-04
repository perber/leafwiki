package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/favorites"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/usersettings"
)

// ErrAuthDisabled is returned when an auth operation is called while auth is disabled.
var ErrAuthDisabled = errors.New("authentication is disabled")

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+$`)

// ─── LoginUseCase ────────────────────────────────────────────────────────────

type LoginInput struct {
	Identifier string
	Password   string
}

type LoginOutput struct {
	Token *coreauth.AuthToken
}

type LoginUseCase struct {
	auth    *coreauth.AuthService
	metrics *httpmetrics.HTTPMetrics
}

func NewLoginUseCase(a *coreauth.AuthService, metrics *httpmetrics.HTTPMetrics) *LoginUseCase {
	return &LoginUseCase{auth: a, metrics: metrics}
}

func (uc *LoginUseCase) Execute(_ context.Context, in LoginInput) (*LoginOutput, error) {
	if uc.auth == nil {
		return nil, ErrAuthDisabled
	}
	token, err := uc.auth.Login(in.Identifier, in.Password)
	if err != nil {
		switch {
		case errors.Is(err, coreauth.ErrUserAccountLocked):
			uc.metrics.IncAuthLoginAttempt("locked")
		case errors.Is(err, coreauth.ErrUserInvalidCredentials):
			uc.metrics.IncAuthLoginAttempt("invalid_credentials")
		default:
			uc.metrics.IncAuthLoginAttempt("error")
		}
		return nil, err
	}
	if token.RequiresTOTP {
		uc.metrics.IncAuthLoginAttempt("totp_required")
	} else {
		uc.metrics.IncAuthLoginAttempt("success")
		uc.metrics.IncAuthSession("issued")
	}
	return &LoginOutput{Token: token}, nil
}

// ─── CompleteTOTPLoginUseCase ────────────────────────────────────────────────

type CompleteTOTPLoginInput struct {
	LoginChallengeToken string
	Code                string
}

type CompleteTOTPLoginOutput struct {
	Token *coreauth.AuthToken
}

type CompleteTOTPLoginUseCase struct {
	auth    *coreauth.AuthService
	metrics *httpmetrics.HTTPMetrics
}

func NewCompleteTOTPLoginUseCase(a *coreauth.AuthService, metrics *httpmetrics.HTTPMetrics) *CompleteTOTPLoginUseCase {
	return &CompleteTOTPLoginUseCase{auth: a, metrics: metrics}
}

func (uc *CompleteTOTPLoginUseCase) Execute(_ context.Context, in CompleteTOTPLoginInput) (*CompleteTOTPLoginOutput, error) {
	if uc.auth == nil {
		return nil, ErrAuthDisabled
	}
	token, err := uc.auth.CompleteTOTPLogin(in.LoginChallengeToken, in.Code)
	if err != nil {
		switch {
		case errors.Is(err, coreauth.ErrUserAccountLocked):
			uc.metrics.IncAuthTOTPVerification("locked")
		case isUserStoreUnavailable(err):
			// Not a real verification failure — the user store is suspended
			// for an in-progress live restore (see errUserStoreUnavailable).
			// Bucketing this as "invalid" would spike that metric with zero
			// actual bad codes during every restore.
			uc.metrics.IncAuthTOTPVerification("unavailable")
		default:
			uc.metrics.IncAuthTOTPVerification("invalid")
		}
		return nil, err
	}
	uc.metrics.IncAuthTOTPVerification("success")
	uc.metrics.IncAuthSession("issued")
	return &CompleteTOTPLoginOutput{Token: token}, nil
}

// ─── StartTOTPSetupUseCase ───────────────────────────────────────────────────

type StartTOTPSetupInput struct {
	UserID          string
	CurrentPassword string
}

type StartTOTPSetupOutput struct {
	Secret     string // manual-entry base32 secret
	OTPAuthURL string // otpauth:// URI for QR-code rendering
}

type StartTOTPSetupUseCase struct {
	auth *coreauth.AuthService
}

func NewStartTOTPSetupUseCase(a *coreauth.AuthService) *StartTOTPSetupUseCase {
	return &StartTOTPSetupUseCase{auth: a}
}

func (uc *StartTOTPSetupUseCase) Execute(_ context.Context, in StartTOTPSetupInput) (*StartTOTPSetupOutput, error) {
	if uc.auth == nil {
		return nil, ErrAuthDisabled
	}
	generated, err := uc.auth.StartTOTPSetup(in.UserID, in.CurrentPassword)
	if err != nil {
		return nil, err
	}
	return &StartTOTPSetupOutput{Secret: generated.Secret, OTPAuthURL: generated.OTPAuthURL}, nil
}

// ─── ConfirmTOTPSetupUseCase ─────────────────────────────────────────────────

type ConfirmTOTPSetupInput struct {
	UserID              string
	Code                string
	CurrentRefreshToken string
}

type ConfirmTOTPSetupOutput struct {
	RecoveryCodes []string
}

type ConfirmTOTPSetupUseCase struct {
	auth    *coreauth.AuthService
	metrics *httpmetrics.HTTPMetrics
}

func NewConfirmTOTPSetupUseCase(a *coreauth.AuthService, metrics *httpmetrics.HTTPMetrics) *ConfirmTOTPSetupUseCase {
	return &ConfirmTOTPSetupUseCase{auth: a, metrics: metrics}
}

func (uc *ConfirmTOTPSetupUseCase) Execute(_ context.Context, in ConfirmTOTPSetupInput) (*ConfirmTOTPSetupOutput, error) {
	if uc.auth == nil {
		return nil, ErrAuthDisabled
	}
	codes, err := uc.auth.ConfirmTOTPSetup(in.UserID, in.Code, in.CurrentRefreshToken)
	if err != nil {
		return nil, err
	}
	uc.metrics.IncAuthTOTPEnrollment("enabled")
	return &ConfirmTOTPSetupOutput{RecoveryCodes: codes}, nil
}

// ─── DisableTOTPUseCase ──────────────────────────────────────────────────────

type DisableTOTPInput struct {
	UserID              string
	CurrentPassword     string
	Code                string
	CurrentRefreshToken string
}

type DisableTOTPUseCase struct {
	auth    *coreauth.AuthService
	metrics *httpmetrics.HTTPMetrics
}

func NewDisableTOTPUseCase(a *coreauth.AuthService, metrics *httpmetrics.HTTPMetrics) *DisableTOTPUseCase {
	return &DisableTOTPUseCase{auth: a, metrics: metrics}
}

func (uc *DisableTOTPUseCase) Execute(_ context.Context, in DisableTOTPInput) error {
	if uc.auth == nil {
		return ErrAuthDisabled
	}
	if err := uc.auth.DisableTOTP(in.UserID, in.CurrentPassword, in.Code, in.CurrentRefreshToken); err != nil {
		return err
	}
	uc.metrics.IncAuthTOTPEnrollment("disabled")
	return nil
}

// ─── GetTOTPStatusUseCase ────────────────────────────────────────────────────

type GetTOTPStatusInput struct {
	UserID string
}

type GetTOTPStatusOutput struct {
	Enabled                bool
	RecoveryCodesRemaining int
}

type GetTOTPStatusUseCase struct {
	auth *coreauth.AuthService
}

func NewGetTOTPStatusUseCase(a *coreauth.AuthService) *GetTOTPStatusUseCase {
	return &GetTOTPStatusUseCase{auth: a}
}

func (uc *GetTOTPStatusUseCase) Execute(_ context.Context, in GetTOTPStatusInput) (*GetTOTPStatusOutput, error) {
	if uc.auth == nil {
		return nil, ErrAuthDisabled
	}
	status, err := uc.auth.GetTOTPStatus(in.UserID)
	if err != nil {
		return nil, err
	}
	return &GetTOTPStatusOutput{Enabled: status.Enabled, RecoveryCodesRemaining: status.RecoveryCodesRemaining}, nil
}

// ─── LogoutUseCase ───────────────────────────────────────────────────────────

type LogoutInput struct{ RefreshToken string }

type LogoutUseCase struct {
	auth    *coreauth.AuthService
	metrics *httpmetrics.HTTPMetrics
}

func NewLogoutUseCase(a *coreauth.AuthService, metrics *httpmetrics.HTTPMetrics) *LogoutUseCase {
	return &LogoutUseCase{auth: a, metrics: metrics}
}

func (uc *LogoutUseCase) Execute(_ context.Context, in LogoutInput) error {
	if uc.auth == nil {
		return ErrAuthDisabled
	}
	if err := uc.auth.RevokeRefreshToken(in.RefreshToken); err != nil {
		return err
	}
	uc.metrics.IncAuthSession("revoked")
	return nil
}

// ─── RefreshTokenUseCase ─────────────────────────────────────────────────────

type RefreshTokenInput struct{ RefreshToken string }

type RefreshTokenOutput struct {
	Token *coreauth.AuthToken
}

type RefreshTokenUseCase struct {
	auth    *coreauth.AuthService
	metrics *httpmetrics.HTTPMetrics
}

func NewRefreshTokenUseCase(a *coreauth.AuthService, metrics *httpmetrics.HTTPMetrics) *RefreshTokenUseCase {
	return &RefreshTokenUseCase{auth: a, metrics: metrics}
}

func (uc *RefreshTokenUseCase) Execute(_ context.Context, in RefreshTokenInput) (*RefreshTokenOutput, error) {
	if uc.auth == nil {
		return nil, ErrAuthDisabled
	}
	token, err := uc.auth.RefreshToken(in.RefreshToken)
	if err != nil {
		return nil, err
	}
	uc.metrics.IncAuthSession("refreshed")
	return &RefreshTokenOutput{Token: token}, nil
}

// ─── CreateUserUseCase ───────────────────────────────────────────────────────

type CreateUserInput struct {
	Username string
	Email    string
	Password string
	Role     string
}

type CreateUserOutput struct {
	User *coreauth.PublicUser
}

type CreateUserUseCase struct {
	// user is resolved on every call rather than cached at construction, so
	// this use case automatically tracks a live restore's
	// AuthService.ReplaceUserStore swap instead of going stale — see
	// Wiki.UserService().
	user     func() *coreauth.UserService
	resolver *coreauth.UserResolver
	log      *slog.Logger
}

func NewCreateUserUseCase(u func() *coreauth.UserService, r *coreauth.UserResolver, log *slog.Logger) *CreateUserUseCase {
	return &CreateUserUseCase{user: u, resolver: r, log: log}
}

func (uc *CreateUserUseCase) Execute(_ context.Context, in CreateUserInput) (*CreateUserOutput, error) {
	ve := sharederrors.NewValidationErrors()
	if in.Username == "" {
		ve.Add("username", "Username must not be empty")
	}
	if in.Email == "" {
		ve.Add("email", "Email must not be empty")
	} else if !emailRegex.MatchString(in.Email) {
		ve.Add("email", "Email is not valid")
	}
	if in.Password == "" {
		ve.Add("password", "Password must not be empty")
	} else if len(in.Password) < coreauth.MinPasswordLength {
		ve.Add("password", fmt.Sprintf("Password must be at least %d characters long", coreauth.MinPasswordLength))
	}
	if !coreauth.IsValidRole(in.Role) {
		ve.Add("role", "Invalid role")
	}
	if ve.HasErrors() {
		return nil, ve
	}

	user, err := uc.user().CreateUser(in.Username, in.Email, in.Password, in.Role)
	if err != nil {
		return nil, err
	}
	if err := uc.resolver.Reload(); err != nil {
		uc.log.Warn("failed to reload user resolver cache", "error", err)
	}
	return &CreateUserOutput{User: user.ToPublicUser()}, nil
}

// ─── UpdateUserUseCase ───────────────────────────────────────────────────────

type UpdateUserInput struct {
	ID               string
	Username         string
	Email            string
	Password         string
	Role             string
	RequesterIsAdmin bool
}

type UpdateUserOutput struct {
	User *coreauth.PublicUser
}

type UpdateUserUseCase struct {
	user     func() *coreauth.UserService
	resolver *coreauth.UserResolver
	log      *slog.Logger
}

func NewUpdateUserUseCase(u func() *coreauth.UserService, r *coreauth.UserResolver, log *slog.Logger) *UpdateUserUseCase {
	return &UpdateUserUseCase{user: u, resolver: r, log: log}
}

func (uc *UpdateUserUseCase) Execute(_ context.Context, in UpdateUserInput) (*UpdateUserOutput, error) {
	ve := sharederrors.NewValidationErrors()
	if in.Username == "" {
		ve.Add("username", "Username must not be empty")
	}
	if in.Email == "" {
		ve.Add("email", "Email must not be empty")
	} else if !emailRegex.MatchString(in.Email) {
		ve.Add("email", "Email is not valid")
	}
	role := in.Role
	roleProvided := strings.TrimSpace(in.Role) != ""
	if in.RequesterIsAdmin && roleProvided && !coreauth.IsValidRole(in.Role) {
		ve.Add("role", "Invalid role")
	}
	if ve.HasErrors() {
		return nil, ve
	}

	if !in.RequesterIsAdmin || !roleProvided {
		existing, err := uc.user().GetUserByID(in.ID)
		if err != nil {
			return nil, err
		}
		role = existing.Role
	}

	user, err := uc.user().UpdateUser(in.ID, in.Username, in.Email, in.Password, role)
	if err != nil {
		return nil, err
	}
	if err := uc.resolver.Reload(); err != nil {
		uc.log.Warn("failed to reload user resolver cache", "error", err)
	}
	return &UpdateUserOutput{User: user.ToPublicUser()}, nil
}

// ─── ChangeOwnPasswordUseCase ────────────────────────────────────────────────

type ChangeOwnPasswordInput struct {
	UserID      string
	OldPassword string
	NewPassword string
}

type ChangeOwnPasswordUseCase struct {
	user func() *coreauth.UserService
}

func NewChangeOwnPasswordUseCase(u func() *coreauth.UserService) *ChangeOwnPasswordUseCase {
	return &ChangeOwnPasswordUseCase{user: u}
}

func (uc *ChangeOwnPasswordUseCase) Execute(_ context.Context, in ChangeOwnPasswordInput) error {
	ve := sharederrors.NewValidationErrors()
	if in.NewPassword == "" {
		ve.Add("newPassword", "New password must not be empty")
	} else if len(in.NewPassword) < coreauth.MinPasswordLength {
		ve.Add("newPassword", fmt.Sprintf("New password must be at least %d characters long", coreauth.MinPasswordLength))
	}
	if _, err := uc.user().DoesIDAndPasswordMatch(in.UserID, in.OldPassword); err != nil {
		ve.Add("oldPassword", "Old password is incorrect")
	}
	if ve.HasErrors() {
		return ve
	}
	return uc.user().ChangeOwnPassword(in.UserID, in.OldPassword, in.NewPassword)
}

// ─── DeleteUserUseCase ───────────────────────────────────────────────────────

type DeleteUserInput struct{ ID string }

type DeleteUserUseCase struct {
	user         func() *coreauth.UserService
	resolver     *coreauth.UserResolver
	favorites    *favorites.FavoritesStore
	userSettings *usersettings.UserSettingsService
	apiKeys      *coreauth.APIKeyService // nil when API key management is disabled (see wiki.go initAuth)
	log          *slog.Logger
}

func NewDeleteUserUseCase(u func() *coreauth.UserService, r *coreauth.UserResolver, f *favorites.FavoritesStore, us *usersettings.UserSettingsService, ak *coreauth.APIKeyService, log *slog.Logger) *DeleteUserUseCase {
	return &DeleteUserUseCase{user: u, resolver: r, favorites: f, userSettings: us, apiKeys: ak, log: log}
}

func (uc *DeleteUserUseCase) Execute(_ context.Context, in DeleteUserInput) error {
	if err := uc.user().DeleteUser(in.ID); err != nil {
		return err
	}
	if err := uc.resolver.Reload(); err != nil {
		uc.log.Warn("failed to reload user resolver cache", "error", err)
	}
	if err := uc.favorites.DeleteAllForUser(in.ID); err != nil {
		uc.log.Warn("failed to delete favorites for deleted user", "userID", in.ID, "error", err)
	}
	if err := uc.userSettings.DeleteAllForUser(in.ID); err != nil {
		uc.log.Warn("failed to delete user settings for deleted user", "userID", in.ID, "error", err)
	}
	if uc.apiKeys != nil {
		if err := uc.apiKeys.DeleteAllForUser(in.ID); err != nil {
			uc.log.Warn("failed to delete api keys for deleted user", "userID", in.ID, "error", err)
		}
	}
	return nil
}

// ─── GetUsersUseCase ─────────────────────────────────────────────────────────

type GetUsersOutput struct {
	Users []*coreauth.PublicUser
}

type GetUsersUseCase struct {
	user func() *coreauth.UserService
}

func NewGetUsersUseCase(u func() *coreauth.UserService) *GetUsersUseCase {
	return &GetUsersUseCase{user: u}
}

func (uc *GetUsersUseCase) Execute(_ context.Context) (*GetUsersOutput, error) {
	users, err := uc.user().GetUsers()
	if err != nil {
		return nil, err
	}
	pub := make([]*coreauth.PublicUser, len(users))
	for i, u := range users {
		pub[i] = u.ToPublicUser()
	}
	return &GetUsersOutput{Users: pub}, nil
}

// ─── GetUserByIDUseCase ──────────────────────────────────────────────────────

type GetUserByIDInput struct{ ID string }

type GetUserByIDOutput struct {
	User *coreauth.PublicUser
}

type GetUserByIDUseCase struct {
	user func() *coreauth.UserService
}

func NewGetUserByIDUseCase(u func() *coreauth.UserService) *GetUserByIDUseCase {
	return &GetUserByIDUseCase{user: u}
}

func (uc *GetUserByIDUseCase) Execute(_ context.Context, in GetUserByIDInput) (*GetUserByIDOutput, error) {
	user, err := uc.user().GetUserByID(in.ID)
	if err != nil {
		return nil, err
	}
	return &GetUserByIDOutput{User: user.ToPublicUser()}, nil
}

// ─── RequestPasswordResetUseCase ─────────────────────────────────────────────

type RequestPasswordResetInput struct {
	Identifier string
}

type RequestPasswordResetUseCase struct {
	// emailTokens is resolved on every call (like user/resolver elsewhere in
	// this file) rather than cached at construction. It returns nil when
	// SMTP is unconfigured or auth is disabled — see wiki.initEmail.
	emailTokens func() *coreauth.EmailTokenService
}

func NewRequestPasswordResetUseCase(e func() *coreauth.EmailTokenService) *RequestPasswordResetUseCase {
	return &RequestPasswordResetUseCase{emailTokens: e}
}

// Execute always succeeds except when email isn't configured at all. It must
// never distinguish "identifier resolved to a user" from "it didn't" — that
// is the whole point of the enumeration-protection design here (see
// coreauth.EmailTokenService.RequestPasswordReset's doc comment) — so the
// actual send happens via RequestPasswordResetAsync, a background goroutine
// whose outcome this call does not wait for or report.
func (uc *RequestPasswordResetUseCase) Execute(_ context.Context, in RequestPasswordResetInput) error {
	tokens := uc.emailTokens()
	if tokens == nil {
		return coreauth.ErrEmailDisabled
	}
	tokens.RequestPasswordResetAsync(in.Identifier)
	return nil
}

// ─── ConfirmPasswordResetUseCase ─────────────────────────────────────────────

type ConfirmPasswordResetInput struct {
	Token       string
	NewPassword string
}

type ConfirmPasswordResetOutput struct {
	User *coreauth.PublicUser
}

type ConfirmPasswordResetUseCase struct {
	emailTokens func() *coreauth.EmailTokenService
}

func NewConfirmPasswordResetUseCase(e func() *coreauth.EmailTokenService) *ConfirmPasswordResetUseCase {
	return &ConfirmPasswordResetUseCase{emailTokens: e}
}

// Execute does not issue a session on success — unlike ConfirmInviteUseCase.
// A reset just revoked every existing session for this user (see
// EmailTokenService.ConfirmPasswordReset); auto-logging back in immediately
// after would undercut that. The frontend sends the user to the login page
// instead.
func (uc *ConfirmPasswordResetUseCase) Execute(_ context.Context, in ConfirmPasswordResetInput) (*ConfirmPasswordResetOutput, error) {
	tokens := uc.emailTokens()
	if tokens == nil {
		return nil, coreauth.ErrEmailDisabled
	}

	ve := sharederrors.NewValidationErrors()
	if in.NewPassword == "" {
		ve.Add("newPassword", "New password must not be empty")
	} else if len(in.NewPassword) < coreauth.MinPasswordLength {
		ve.Add("newPassword", fmt.Sprintf("New password must be at least %d characters long", coreauth.MinPasswordLength))
	}
	if ve.HasErrors() {
		return nil, ve
	}

	user, err := tokens.ConfirmPasswordReset(in.Token, in.NewPassword)
	if err != nil {
		return nil, err
	}
	return &ConfirmPasswordResetOutput{User: user.ToPublicUser()}, nil
}

// ─── InviteUserUseCase ────────────────────────────────────────────────────────

type InviteUserInput struct {
	Username string
	Email    string
	Role     string
}

type InviteUserOutput struct {
	User      *coreauth.PublicUser
	EmailSent bool
}

type InviteUserUseCase struct {
	user        func() *coreauth.UserService
	emailTokens func() *coreauth.EmailTokenService
	resolver    *coreauth.UserResolver
	log         *slog.Logger
}

func NewInviteUserUseCase(u func() *coreauth.UserService, e func() *coreauth.EmailTokenService, r *coreauth.UserResolver, log *slog.Logger) *InviteUserUseCase {
	return &InviteUserUseCase{user: u, emailTokens: e, resolver: r, log: log}
}

// Execute mirrors CreateUserUseCase's validation (minus a password field —
// InviteUser generates one internally, see coreauth.UserService.InviteUser).
// A send failure does not fail the request or roll back user creation: the
// admin already has a real user account to work with, and EmailSent=false
// tells the frontend to offer a resend (see ResendInviteUseCase).
func (uc *InviteUserUseCase) Execute(ctx context.Context, in InviteUserInput) (*InviteUserOutput, error) {
	tokens := uc.emailTokens()
	if tokens == nil {
		return nil, coreauth.ErrEmailDisabled
	}

	ve := sharederrors.NewValidationErrors()
	if in.Username == "" {
		ve.Add("username", "Username must not be empty")
	}
	if in.Email == "" {
		ve.Add("email", "Email must not be empty")
	} else if !emailRegex.MatchString(in.Email) {
		ve.Add("email", "Email is not valid")
	}
	if !coreauth.IsValidRole(in.Role) {
		ve.Add("role", "Invalid role")
	}
	if ve.HasErrors() {
		return nil, ve
	}

	user, err := uc.user().InviteUser(in.Username, in.Email, in.Role)
	if err != nil {
		return nil, err
	}
	if err := uc.resolver.Reload(); err != nil {
		uc.log.Warn("failed to reload user resolver cache", "error", err)
	}

	emailSent := true
	if err := tokens.IssueInvite(ctx, user); err != nil {
		uc.log.Warn("failed to send invite email", "userID", user.ID, "error", err)
		emailSent = false
	}

	return &InviteUserOutput{User: user.ToPublicUser(), EmailSent: emailSent}, nil
}

// ─── ResendInviteUseCase ──────────────────────────────────────────────────────

type ResendInviteInput struct {
	UserID string
}

type ResendInviteUseCase struct {
	user        func() *coreauth.UserService
	emailTokens func() *coreauth.EmailTokenService
}

func NewResendInviteUseCase(u func() *coreauth.UserService, e func() *coreauth.EmailTokenService) *ResendInviteUseCase {
	return &ResendInviteUseCase{user: u, emailTokens: e}
}

func (uc *ResendInviteUseCase) Execute(ctx context.Context, in ResendInviteInput) error {
	tokens := uc.emailTokens()
	if tokens == nil {
		return coreauth.ErrEmailDisabled
	}

	user, err := uc.user().GetUserByID(in.UserID)
	if err != nil {
		return err
	}
	if !user.MustSetPassword {
		return coreauth.ErrInviteAlreadyAccepted
	}

	return tokens.IssueInvite(ctx, user)
}

// ─── ConfirmInviteUseCase ─────────────────────────────────────────────────────

type ConfirmInviteInput struct {
	Token       string
	NewPassword string
}

type ConfirmInviteOutput struct {
	Token *coreauth.AuthToken
}

type ConfirmInviteUseCase struct {
	emailTokens func() *coreauth.EmailTokenService
	auth        *coreauth.AuthService
}

func NewConfirmInviteUseCase(e func() *coreauth.EmailTokenService, a *coreauth.AuthService) *ConfirmInviteUseCase {
	return &ConfirmInviteUseCase{emailTokens: e, auth: a}
}

// Execute consumes the invite token, sets the user's password, and — unlike
// ConfirmPasswordResetUseCase — issues a fresh session immediately: a freshly
// invited user has no prior sessions to worry about, so there's no reason to
// make them log in a second time right after setting their first password.
func (uc *ConfirmInviteUseCase) Execute(_ context.Context, in ConfirmInviteInput) (*ConfirmInviteOutput, error) {
	tokens := uc.emailTokens()
	if tokens == nil {
		return nil, coreauth.ErrEmailDisabled
	}

	ve := sharederrors.NewValidationErrors()
	if in.NewPassword == "" {
		ve.Add("newPassword", "New password must not be empty")
	} else if len(in.NewPassword) < coreauth.MinPasswordLength {
		ve.Add("newPassword", fmt.Sprintf("New password must be at least %d characters long", coreauth.MinPasswordLength))
	}
	if ve.HasErrors() {
		return nil, ve
	}

	user, err := tokens.ConfirmInvite(in.Token, in.NewPassword)
	if err != nil {
		return nil, err
	}

	token, err := uc.auth.IssueSessionForUser(user.ID)
	if err != nil {
		return nil, err
	}
	return &ConfirmInviteOutput{Token: token}, nil
}
