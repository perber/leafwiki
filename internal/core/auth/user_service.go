package auth

import (
	"fmt"
	"strings"

	"github.com/perber/wiki/internal/core/shared"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAdminUsername = "admin"
	defaultAdminEmail    = "admin@localhost"
)

type UserService struct {
	store *UserStore
}

func NewUserService(store *UserStore) *UserService {
	return &UserService{
		store: store,
	}
}

func (s *UserService) InitDefaultAdmin(username, email, newPassword string) error {
	// Check if admin user already exists

	if _, err := s.store.GetAdminUser(); err == nil {
		// Admin user already exists, no need to create a new one
		return nil
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

	return user, nil
}

func (s *UserService) GetUserByID(id string) (*User, error) {
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return user, nil
}

func (s *UserService) UpdateUser(id, username, email, password, role string) (*User, error) {
	// Check if user exists
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return nil, ErrUserNotFound
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

	// Update user fields
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

func (s *UserService) DoesIDAndPasswordMatch(id, password string) (bool, error) {
	// Check if user exists
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return false, ErrUserNotFound
	}

	// Check if password is correct
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return false, ErrUserInvalidCredentials
	}

	return true, nil
}

func (s *UserService) DeleteUser(id string) error {
	// Check if user exists
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return ErrUserNotFound
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
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *UserService) GetUserByIdentifier(identifier string) (*User, error) {
	user, err := s.store.GetUserByUsername(identifier)
	if err != nil {
		user, err = s.store.GetUserByEmail(identifier)
		if err != nil {
			return nil, ErrUserNotFound
		}
	}
	return user, nil
}

func (s *UserService) GetUserByEmailOrUsernameAndPassword(identifier, password string) (*User, error) {
	user, err := s.store.GetUserByUsername(identifier)
	if err != nil {
		user, err = s.store.GetUserByEmail(identifier)
		if err != nil {
			return nil, ErrUserNotFound
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
		return ErrUserNotFound
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

	return adminUser, nil
}

// UpdateRecoveryCodeHashes replaces the stored TOTP recovery-code hashes for
// id, e.g. to atomically remove a hash after it has been consumed at login.
func (s *UserService) UpdateRecoveryCodeHashes(id string, hashes []string) error {
	return s.store.UpdateRecoveryCodeHashes(id, hashes)
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
