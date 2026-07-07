package auth

import "errors"

var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserInvalidCredentials = errors.New("invalid credentials")
var ErrUserInvalidRole = errors.New("invalid role")
var ErrUserAdminCannotBeDeleted = errors.New("admin user cannot be deleted; change role before deletion")
var ErrLastAdminCannotBeDemoted = errors.New("cannot remove admin role from the last admin user")
var ErrInvalidToken = errors.New("invalid token")
var ErrUserAccountLocked = errors.New("account temporarily locked due to too many failed login attempts")

var ErrAPIKeyNotFound = errors.New("api key not found")
var ErrAPIKeyInvalid = errors.New("invalid api key")
var ErrAPIKeyRevoked = errors.New("api key has been revoked")
var ErrAPIKeyExpired = errors.New("api key has expired")
var ErrAPIKeyPrefixCollision = errors.New("api key prefix collision")
