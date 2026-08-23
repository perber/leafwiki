package auth

import "errors"

var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserInvalidCredentials = errors.New("invalid credentials")
var ErrUserInvalidRole = errors.New("invalid role")
var ErrUserAdminCannotBeDeleted = errors.New("admin user cannot be deleted; change role before deletion")
var ErrLastAdminCannotBeDemoted = errors.New("cannot remove admin role from the last admin user")
var ErrInvalidToken = errors.New("invalid token")
var ErrSessionManagerNotWired = errors.New("session manager: resolveUser not configured (NewAuthService wires this; a SessionManager built directly must set it before use)")
var ErrUserAccountLocked = errors.New("account temporarily locked due to too many failed login attempts")
var ErrRemoteUserEmailConflict = errors.New("remote user auto-create: asserted email belongs to a different existing user")
var ErrPasswordTooShort = errors.New("password is too short")
var ErrEditorLimitReached = errors.New("editor limit reached for this plan")

var ErrAPIKeyNotFound = errors.New("api key not found")
var ErrAPIKeyInvalid = errors.New("invalid api key")
var ErrAPIKeyRevoked = errors.New("api key has been revoked")
var ErrAPIKeyExpired = errors.New("api key has expired")
var ErrAPIKeyPrefixCollision = errors.New("api key prefix collision")

var ErrEmailTokenInvalid = errors.New("invalid or expired token")
var ErrEmailDisabled = errors.New("email is not configured")
var ErrInviteAlreadyAccepted = errors.New("invite has already been accepted")
