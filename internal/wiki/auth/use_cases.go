package auth

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"regexp"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
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
	auth *coreauth.AuthService
}

func NewLoginUseCase(a *coreauth.AuthService) *LoginUseCase { return &LoginUseCase{auth: a} }

func (uc *LoginUseCase) Execute(_ context.Context, in LoginInput) (*LoginOutput, error) {
	if uc.auth == nil {
		return nil, ErrAuthDisabled
	}
	token, err := uc.auth.Login(in.Identifier, in.Password)
	if err != nil {
		return nil, err
	}
	return &LoginOutput{Token: token}, nil
}

// ─── LogoutUseCase ───────────────────────────────────────────────────────────

type LogoutInput struct{ RefreshToken string }

type LogoutUseCase struct {
	auth *coreauth.AuthService
}

func NewLogoutUseCase(a *coreauth.AuthService) *LogoutUseCase { return &LogoutUseCase{auth: a} }

func (uc *LogoutUseCase) Execute(_ context.Context, in LogoutInput) error {
	if uc.auth == nil {
		return ErrAuthDisabled
	}
	return uc.auth.RevokeRefreshToken(in.RefreshToken)
}

// ─── RefreshTokenUseCase ─────────────────────────────────────────────────────

type RefreshTokenInput struct{ RefreshToken string }

type RefreshTokenOutput struct {
	Token *coreauth.AuthToken
}

type RefreshTokenUseCase struct {
	auth *coreauth.AuthService
}

func NewRefreshTokenUseCase(a *coreauth.AuthService) *RefreshTokenUseCase {
	return &RefreshTokenUseCase{auth: a}
}

func (uc *RefreshTokenUseCase) Execute(_ context.Context, in RefreshTokenInput) (*RefreshTokenOutput, error) {
	if uc.auth == nil {
		return nil, ErrAuthDisabled
	}
	token, err := uc.auth.RefreshToken(in.RefreshToken)
	if err != nil {
		return nil, err
	}
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
	user     *coreauth.UserService
	resolver *coreauth.UserResolver
	log      *slog.Logger
}

func NewCreateUserUseCase(u *coreauth.UserService, r *coreauth.UserResolver, log *slog.Logger) *CreateUserUseCase {
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
	} else if len(in.Password) < 8 {
		ve.Add("password", "Password must be at least 8 characters long")
	}
	if !coreauth.IsValidRole(in.Role) {
		ve.Add("role", "Invalid role")
	}
	if ve.HasErrors() {
		return nil, ve
	}

	user, err := uc.user.CreateUser(in.Username, in.Email, in.Password, in.Role)
	if err != nil {
		return nil, err
	}
	if err := uc.resolver.Reload(); err != nil {
		log.Printf("warning: could not reload user resolver cache: %v", err)
	}
	return &CreateUserOutput{User: user.ToPublicUser()}, nil
}

// ─── UpdateUserUseCase ───────────────────────────────────────────────────────

type UpdateUserInput struct {
	ID       string
	Username string
	Email    string
	Password string
	Role     string
}

type UpdateUserOutput struct {
	User *coreauth.PublicUser
}

type UpdateUserUseCase struct {
	user     *coreauth.UserService
	resolver *coreauth.UserResolver
	log      *slog.Logger
}

func NewUpdateUserUseCase(u *coreauth.UserService, r *coreauth.UserResolver, log *slog.Logger) *UpdateUserUseCase {
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
	if !coreauth.IsValidRole(in.Role) {
		ve.Add("role", "Invalid role")
	}
	if ve.HasErrors() {
		return nil, ve
	}

	user, err := uc.user.UpdateUser(in.ID, in.Username, in.Email, in.Password, in.Role)
	if err != nil {
		return nil, err
	}
	if err := uc.resolver.Reload(); err != nil {
		log.Printf("warning: could not reload user resolver cache: %v", err)
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
	user *coreauth.UserService
}

func NewChangeOwnPasswordUseCase(u *coreauth.UserService) *ChangeOwnPasswordUseCase {
	return &ChangeOwnPasswordUseCase{user: u}
}

func (uc *ChangeOwnPasswordUseCase) Execute(_ context.Context, in ChangeOwnPasswordInput) error {
	ve := sharederrors.NewValidationErrors()
	if in.NewPassword == "" {
		ve.Add("newPassword", "New password must not be empty")
	} else if len(in.NewPassword) < 8 {
		ve.Add("newPassword", "New password must be at least 8 characters long")
	}
	if _, err := uc.user.DoesIDAndPasswordMatch(in.UserID, in.OldPassword); err != nil {
		ve.Add("oldPassword", "Old password is incorrect")
	}
	if ve.HasErrors() {
		return ve
	}
	return uc.user.ChangeOwnPassword(in.UserID, in.OldPassword, in.NewPassword)
}

// ─── DeleteUserUseCase ───────────────────────────────────────────────────────

type DeleteUserInput struct{ ID string }

type DeleteUserUseCase struct {
	user     *coreauth.UserService
	resolver *coreauth.UserResolver
	log      *slog.Logger
}

func NewDeleteUserUseCase(u *coreauth.UserService, r *coreauth.UserResolver, log *slog.Logger) *DeleteUserUseCase {
	return &DeleteUserUseCase{user: u, resolver: r, log: log}
}

func (uc *DeleteUserUseCase) Execute(_ context.Context, in DeleteUserInput) error {
	if err := uc.user.DeleteUser(in.ID); err != nil {
		return err
	}
	if err := uc.resolver.Reload(); err != nil {
		log.Printf("warning: could not reload user resolver cache: %v", err)
	}
	return nil
}

// ─── GetUsersUseCase ─────────────────────────────────────────────────────────

type GetUsersOutput struct {
	Users []*coreauth.PublicUser
}

type GetUsersUseCase struct {
	user *coreauth.UserService
}

func NewGetUsersUseCase(u *coreauth.UserService) *GetUsersUseCase { return &GetUsersUseCase{user: u} }

func (uc *GetUsersUseCase) Execute(_ context.Context) (*GetUsersOutput, error) {
	users, err := uc.user.GetUsers()
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
	user *coreauth.UserService
}

func NewGetUserByIDUseCase(u *coreauth.UserService) *GetUserByIDUseCase {
	return &GetUserByIDUseCase{user: u}
}

func (uc *GetUserByIDUseCase) Execute(_ context.Context, in GetUserByIDInput) (*GetUserByIDOutput, error) {
	user, err := uc.user.GetUserByID(in.ID)
	if err != nil {
		return nil, err
	}
	return &GetUserByIDOutput{User: user.ToPublicUser()}, nil
}
