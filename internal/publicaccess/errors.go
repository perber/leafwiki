package publicaccess

import sharederrors "github.com/perber/wiki/internal/core/shared/errors"

// ErrCodeEnvManaged is the LocalizedError code returned when a caller tries to
// change public access mode on an instance where it is pinned by
// --public-access / LEAFWIKI_PUBLIC_ACCESS (or forced by --disable-auth). The
// HTTP layer maps this code to 409 Conflict.
const ErrCodeEnvManaged = "public_access_env_managed"

func errEnvManaged() error {
	return sharederrors.NewLocalizedError(
		ErrCodeEnvManaged,
		"Public access mode is set by environment configuration and cannot be changed here",
		"public access mode is env-managed",
		nil,
	)
}
