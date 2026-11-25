package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/perber/wiki/internal/core/shared"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAdminUsername = "admin"
	defaultAdminEmail    = "admin@localhost"

	// MinPasswordLength is the minimum accepted length for user-chosen
	// passwords, enforced consistently across every password entry point:
	// user creation, password change/reset, invites, and the initial admin
	// bootstrap password.
	MinPasswordLength = 8
)

type UserService struct {
	store *UserStore
	log   *slog.Logger
	// editorLimit caps how many admin+editor users may exist at once; 0
	// (the default) means unlimited, matching today's self-hosted behavior.
	// Set via SetEditorLimit rather than a constructor parameter so the
	// dozens of existing NewUserService(store) call sites (production and
	// tests) stay untouched — only wiki.WikiOptions.EditorLimit opts in.
	editorLimit int
}

func NewUserService(store *UserStore) *UserService {
	return &UserService{
		store: store,
		log:   slog.Default().With("component", "UserService"),
	}
}

// SetEditorLimit sets the max number of admin+editor users CreateUser/
// UpdateUser will allow; 0 means unlimited. See the editorLimit field doc.
func (s *UserService) SetEditorLimit(limit int) {
	s.editorLimit = limit
}

// checkEditorLimit returns ErrEditorLimitReached if creating/promoting a
// user into role would exceed editorLimit. Viewers are always allowed
// (they never count against the limit), and a limit <= 0 means unlimited.
func (s *UserService) checkEditorLimit(role string) error {
	if s.editorLimit <= 0 || (role != RoleAdmin && role != RoleEditor) {
		return nil
	}
	count, err := s.store.CountEditorUsers()
	if err != nil {
		return err
	}
	if count >= s.editorLimit {
		return ErrEditorLimitReached
	}
	return nil
}

func (s *UserService) InitDefaultAdmin(username, email, newPassword string) error {
	// Check if admin user already exists

	if _, err := s.store.GetAdminUser(); err == nil {
		// Admin user already exists, no need to create a new one
		return nil
	}

	if len(newPassword) < MinPasswordLength {
		return fmt.Errorf("%w: initial admin password must be at least %d characters long", ErrPasswordTooShort, MinPasswordLength)
	}

	username = defaultIfEmpty(username, defaultAdminUsername)
	email = defaultIfEmpty(email, defaultAdminEmail)

	if _, err := s.CreateUser(username, email, newPassword, "admin"); err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	return nil
}

// defaultIfEmpty trims value and returns fallback if the result is empty.
func defaultIfEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func (s *UserService) CreateUser(username, email, password, role string) (*User, error) {
	// Check if user already exists
	_, err := s.store.GetUserByUsername(username)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}

	// Check if email already exists
	_, err = s.store.GetUserByEmail(email)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}

	// Validate role
	if !IsValidRole(role) {
		return nil, ErrUserInvalidRole
	}

	if err := s.checkEditorLimit(role); err != nil {
		return nil, err
	}

	// hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Generate unique ID
	id, err := shared.GenerateUniqueID()
	if err != nil {
		return nil, err
	}

	// Create new user
	user := &User{
		ID:       id,
		Username: username,
		Email:    email,
		Password: string(hashedPassword),
		Role:     role,
	}

	// Save user to store
	err = s.store.CreateUser(user)
	if err != nil {
		return nil, err
	}

	s.log.Info("user created", "userID", user.ID, "role", user.Role)
	return user, nil
}

// mapUserLookupErr maps a UserStore lookup error to ErrUserNotFound, except
// when err is the store's own "suspended for live restore" LocalizedError
// (see errUserStoreUnavailable), which is passed through unchanged so
// callers can distinguish "restore in progress, retry" from "genuinely no
// such user" instead of collapsing both into a 404.
func mapUserLookupErr(err error) error {
	if loc, ok := sharederrors.AsLocalizedError(err); ok && loc.Code == userStoreUnavailableCode {
		return err
	}
	return ErrUserNotFound
}

func (s *UserService) GetUserByID(id string) (*User, error) {
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return nil, mapUserLookupErr(err)
	}

	return user, nil
}

func (s *UserService) UpdateUser(id, username, email, password, role string) (*User, error) {
	// Check if user exists
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return nil, mapUserLookupErr(err)
	}

	// Check if username already exists (but if it's the same user, ignore)
	existingUser, err := s.store.GetUserByUsername(username)
	if err == nil && existingUser.ID != id {
		return nil, ErrUserAlreadyExists
	}

	// Check if email already exists (but if it's the same user, ignore)
	existingUser, err = s.store.GetUserByEmail(email)
	if err == nil && existingUser.ID != id {
		return nil, ErrUserAlreadyExists
	}

	if strings.TrimSpace(role) == "" {
		role = user.Role
	}

	// Validate role
	if !IsValidRole(role) {
		return nil, ErrUserInvalidRole
	}

	// Prevent demoting the last admin
	if user.HasRole(RoleAdmin) && role != RoleAdmin {
		count, err := s.store.CountAdminUsers()
		if err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, ErrLastAdminCannotBeDemoted
		}
	}

	// Only a promotion into admin/editor from a role that wasn't already
	// counted (viewer) claims a new editor slot — reassigning someone who
	// was already admin/editor (e.g. admin->editor) stays neutral and must
	// not be blocked as if it were a third new editor.
	wasCounted := user.Role == RoleAdmin || user.Role == RoleEditor
	if !wasCounted {
		if err := s.checkEditorLimit(role); err != nil {
			return nil, err
		}
	}

	// Update user fields
	oldRole := user.Role
	user.Username = username
	user.Email = email
	user.Role = role

	if password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Password = string(hashedPassword)
	}

	// Save updated user to store
	err = s.store.UpdateUser(user)
	if err != nil {
		return nil, err
	}

	// An admin setting a real password out-of-band (e.g. via the "Change
	// Password" dialog) completes an outstanding invite: the account now has a
	// usable password, so it must stop showing as "invitation pending". This is
	// the admin-driven equivalent of CompleteInvite, which handles the email
	// accept flow. must_set_password lives outside UserStore.UpdateUser's
	// UPDATE, so it has to be cleared explicitly.
	if password != "" && user.MustSetPassword {
		if err := s.store.SetMustSetPassword(user.ID, false); err != nil {
			return nil, err
		}
		user.MustSetPassword = false
		s.log.Info("invite completed via admin password change", "userID", user.ID)
	}

	if oldRole != user.Role {
		s.log.Info("user role changed", "userID", user.ID, "oldRole", oldRole, "newRole", user.Role)
	} else {
		s.log.Info("user updated", "userID", user.ID)
	}
	return user, nil
}

func (s *UserService) UpdatePassword(id string, newpassword string) error {
	// Check if user exists
	_, err := s.store.GetUserByID(id)
	if err != nil {
		return err
	}

	// hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newpassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Save updated user to store
	err = s.store.UpdatePassword(id, string(hashedPassword))
	if err != nil {
		return err
	}

	return nil
}

// DoesIDAndPasswordMatch verifies that password matches the stored hash for
// id, returning the fetched user on success so callers that need it right
// afterward (e.g. StartTOTPSetup, DisableTOTP) don't have to re-fetch it.
func (s *UserService) DoesIDAndPasswordMatch(id, password string) (*User, error) {
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return nil, mapUserLookupErr(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrUserInvalidCredentials
	}

	return user, nil
}

func (s *UserService) DeleteUser(id string) error {
	// Check if user exists
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return mapUserLookupErr(err)
	}
	// Check if user is admin
	if user.HasRole(RoleAdmin) {
		return ErrUserAdminCannotBeDeleted
	}
	// Delete user from store
	err = s.store.DeleteUser(id)
	if err != nil {
		return err
	}
	s.log.Info("user deleted", "userID", id)
	return nil
}

func (s *UserService) GetUsers() ([]*User, error) {
	users, err := s.store.GetAllUsers()
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (s *UserService) GetUserByUsername(username string) (*User, error) {
	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		return nil, mapUserLookupErr(err)
	}
	return user, nil
}

func (s *UserService) GetUserByIdentifier(identifier string) (*User, error) {
	user, err := s.store.GetUserByUsername(identifier)
	if err != nil {
		user, err = s.store.GetUserByEmail(identifier)
		if err != nil {
			return nil, mapUserLookupErr(err)
		}
	}
	return user, nil
}

// lookupRemoteUserByIdentifier resolves identifier as a username, falling
// back to an email lookup. Unlike GetUserByIdentifier, it does not collapse
// store errors into ErrUserNotFound: GetOrCreateRemoteUser auto-provisions on
// ErrUserNotFound, so a transient store error (e.g. a DB hiccup) must not be
// mistaken for "no such user" and trigger an unwanted account creation.
func (s *UserService) lookupRemoteUserByIdentifier(identifier string) (*User, error) {
	user, err := s.store.GetUserByUsername(identifier)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}
	return s.store.GetUserByEmail(identifier)
}

// GetOrCreateRemoteUser resolves identifier (username or email, same lookup as
// lookupRemoteUserByIdentifier) to an existing user, auto-provisioning a new
// one with a random password and defaultRole if none exists. Used by the
// reverse-proxy auto-create feature: identifier becomes the new user's
// username verbatim; if email is empty, a non-deliverable placeholder is
// synthesized under the RFC 2606 reserved ".invalid" TLD.
func (s *UserService) GetOrCreateRemoteUser(identifier, email, defaultRole string) (*User, error) {
	user, err := s.lookupRemoteUserByIdentifier(identifier)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	if email == "" {
		email = identifier + "@remote-user.invalid"
	}

	password, err := shared.GenerateRandomPassword(32)
	if err != nil {
		return nil, err
	}

	created, err := s.CreateUser(identifier, email, password, defaultRole)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			// CreateUser's ErrUserAlreadyExists means either identifier collided
			// (we lost a create race against a concurrent request for the same new
			// identifier — the expected, recoverable case) or email collided with a
			// different, unrelated existing user (identifier itself was never
			// created). Disambiguate instead of assuming the race case: only treat
			// it as recovered if identifier now resolves to a user.
			if existing, lookupErr := s.lookupRemoteUserByIdentifier(identifier); lookupErr == nil {
				return existing, nil
			}
			return nil, ErrRemoteUserEmailConflict
		}
		return nil, err
	}

	s.log.Info("remote user auto-provisioned", "userID", created.ID, "username", identifier, "role", defaultRole)
	return created, nil
}

// GetUserBySub resolves an OIDC-linked account by its provider "sub" claim.
func (s *UserService) GetUserBySub(sub string) (*User, error) {
	user, err := s.store.GetUserBySub(sub)
	if err != nil {
		return nil, mapUserLookupErr(err)
	}
	return user, nil
}

// GetOrCreateOIDCUser resolves an OIDC login to a local account: first by the
// provider's stable "sub" claim, then — for a user never seen via OIDC
// before — by preferredUsername, to link a pre-existing local account rather
// than create a duplicate. If neither matches, a brand-new account is
// created. Unlike GetOrCreateRemoteUser, no email-collision disambiguation
// is needed on the linking path: a signed, verified "sub" is a strong enough
// signal on its own that a username match is intentional, so email is never
// compared here (also per the provider's email_verified being unreliable).
func (s *UserService) GetOrCreateOIDCUser(sub, preferredUsername, email, role string) (*User, error) {
	user, err := s.store.GetUserBySub(sub)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	if existing, lookupErr := s.store.GetUserByUsername(preferredUsername); lookupErr == nil {
		if err := s.store.SetOIDCSub(existing.ID, sub); err != nil {
			return nil, err
		}
		existing.OIDCSub = sub
		s.log.Info("local account linked to OIDC identity", "userID", existing.ID, "username", preferredUsername)
		return existing, nil
	} else if !errors.Is(lookupErr, ErrUserNotFound) {
		return nil, lookupErr
	}

	if email == "" {
		email = preferredUsername + "@oidc-user.invalid"
	}
	password, err := shared.GenerateRandomPassword(32)
	if err != nil {
		return nil, err
	}

	created, err := s.CreateUser(preferredUsername, email, password, role)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			// Lost a create race against a concurrent OIDC login for the same new
			// identity — re-resolve by sub first (the other request may have
			// already finished linking/creating), then by username.
			if existing, lookupErr := s.store.GetUserBySub(sub); lookupErr == nil {
				return existing, nil
			}
			if existing, lookupErr := s.store.GetUserByUsername(preferredUsername); lookupErr == nil {
				if err := s.store.SetOIDCSub(existing.ID, sub); err != nil {
					return nil, err
				}
				existing.OIDCSub = sub
				return existing, nil
			}
		}
		return nil, err
	}

	if err := s.store.SetOIDCSub(created.ID, sub); err != nil {
		return nil, err
	}
	created.OIDCSub = sub
	s.log.Info("OIDC user auto-provisioned", "userID", created.ID, "username", preferredUsername, "role", role)
	return created, nil
}

// SyncOIDCRole updates userID's role to match the role currently mapped from
// their OIDC groups claim, re-evaluated on every login per the fork's spec.
// A tripped last-admin guard (ErrLastAdminCannotBeDemoted) is logged and
// swallowed rather than failing the login — the account keeps its current
// role until there's a second admin to take over.
func (s *UserService) SyncOIDCRole(userID, role string) {
	if err := s.store.UpdateUserRole(userID, role); err != nil {
		if errors.Is(err, ErrLastAdminCannotBeDemoted) {
			s.log.Warn("OIDC role sync skipped: cannot demote the last admin", "userID", userID, "mappedRole", role)
			return
		}
		s.log.Error("OIDC role sync failed", "userID", userID, "mappedRole", role, "error", err)
	}
}

func (s *UserService) GetUserByEmailOrUsernameAndPassword(identifier, password string) (*User, error) {
	user, err := s.store.GetUserByUsername(identifier)
	if err != nil {
		user, err = s.store.GetUserByEmail(identifier)
		if err != nil {
			return nil, mapUserLookupErr(err)
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, ErrUserInvalidCredentials
	}

	return user, nil
}

func (s *UserService) ChangeOwnPassword(id, oldPassword, newPassword string) error {
	// Check if user exists
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return mapUserLookupErr(err)
	}

	// Check if old password is correct
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword))
	if err != nil {
		return ErrUserInvalidCredentials
	}

	// hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Save updated user to store
	err = s.store.UpdatePassword(id, string(hashedPassword))
	if err != nil {
		return err
	}

	s.log.Info("password changed", "userID", id)
	return nil
}

func (s *UserService) ResetAdminUserPassword(username, email string) (*User, error) {
	// Generate a new password for the admin user
	password, err := shared.GenerateRandomPassword(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	// if the user is not found create a new one
	adminUser, err := s.store.GetAdminUser()
	if err != nil {
		if err == ErrUserNotFound {
			// Create default admin user
			username = defaultIfEmpty(username, defaultAdminUsername)
			email = defaultIfEmpty(email, defaultAdminEmail)
			adminUser, err = s.CreateUser(username, email, password, RoleAdmin)
			if err != nil {
				return nil, fmt.Errorf("failed to create default admin: %w", err)
			}
		} else {
			return nil, err
		}
	}

	// Update the password for the admin user
	err = s.UpdatePassword(adminUser.ID, password)
	if err != nil {
		return nil, fmt.Errorf("failed to update admin password: %w", err)
	}

	// Return the admin user
	// Note: I need to return the user with the new password, because the user lost his password
	adminUser.Password = password // Set the password to the generated one

	s.log.Info("admin password reset", "userID", adminUser.ID)
	return adminUser, nil
}

// InviteUser creates a new user with a random, bcrypt-hashed password that is
// never returned or logged, and marks MustSetPassword so the admin UI can
// show an "Invitation pending" state. The account exists and is fully usable
// (listable, assignable API keys, etc.) but cannot meaningfully log in until
// CompleteInvite sets a real password.
func (s *UserService) InviteUser(username, email, role string) (*User, error) {
	password, err := shared.GenerateRandomPassword(32)
	if err != nil {
		return nil, err
	}

	user, err := s.CreateUser(username, email, password, role)
	if err != nil {
		return nil, err
	}

	if err := s.store.SetMustSetPassword(user.ID, true); err != nil {
		return nil, err
	}
	user.MustSetPassword = true

	s.log.Info("user invited", "userID", user.ID, "role", user.Role)
	return user, nil
}

// CompleteInvite sets id's real password and clears MustSetPassword. Called
// once an invite token is confirmed (see auth.EmailTokenService.ConfirmInvite).
// It only succeeds while the invite is still pending and returns
// ErrInviteAlreadyAccepted once any password has been set (another invite
// link, a reset link, an admin), or when id has no pending invite at all.
func (s *UserService) CompleteInvite(id, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	completed, err := s.store.CompleteInvite(id, string(hashedPassword))
	if err != nil {
		return err
	}
	if !completed {
		return ErrInviteAlreadyAccepted
	}

	s.log.Info("invite accepted", "userID", id)
	return nil
}

// ConsumeRecoveryCodeHash atomically replaces oldHashes with newHashes for id
// via compare-and-swap: the write only takes effect if the stored hashes
// still match oldHashes exactly. Returns swapped=false (with no error) if
// they no longer match — e.g. a concurrent request already consumed a code —
// so the caller can re-read the current hashes and retry.
func (s *UserService) ConsumeRecoveryCodeHash(id string, oldHashes, newHashes []string) (swapped bool, err error) {
	return s.store.ConsumeRecoveryCodeHash(id, oldHashes, newHashes)
}

// SetPendingTOTPSecret stores a freshly generated, not-yet-confirmed encrypted
// TOTP secret for id. TOTP remains disabled until EnableTOTP confirms it.
func (s *UserService) SetPendingTOTPSecret(id, encryptedSecret string) error {
	return s.store.SetPendingTOTPSecret(id, encryptedSecret)
}

// EnableTOTP marks TOTP enabled for id with the confirmed encrypted secret and
// the hashed recovery codes generated alongside it.
func (s *UserService) EnableTOTP(id, encryptedSecret string, recoveryCodeHashes []string) error {
	return s.store.EnableTOTP(id, encryptedSecret, recoveryCodeHashes)
}

// DisableTOTP clears TOTP secret, enabled flag, and recovery codes for id.
func (s *UserService) DisableTOTP(id string) error {
	return s.store.DisableTOTP(id)
}

func (s *UserService) Close() error {
	return s.store.Close()
}

// suspendStore closes the underlying store's DB connection and prevents it
// from lazily reconnecting — see UserStore.suspend.
func (s *UserService) suspendStore() error {
	return s.store.suspend()
}
