package wiki

import (
	"errors"
)

// ErrAuthDisabled is returned when an auth-related operation is called
// while authentication is disabled.
var ErrAuthDisabled = errors.New("authentication is disabled")
