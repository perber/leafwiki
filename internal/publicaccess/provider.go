package publicaccess

// Provider is the read-only view of the public-access flag that the HTTP layer
// depends on. *Service implements it; so does the value returned by Fixed.
// Keeping the interface here (rather than in internal/http) lets both the
// router and the per-request auth middleware share one definition.
type Provider interface {
	// Enabled reports whether anonymous read access is currently allowed.
	Enabled() bool
	// EnvManaged reports whether the value is pinned by environment
	// configuration and therefore read-only in the Settings UI.
	EnvManaged() bool
}

// ReadGate is the minimal slice of Provider the per-request read gate needs
// (see auth.RequireAuthOrPublicRead). Provider satisfies it.
type ReadGate interface {
	Enabled() bool
}

// Fixed returns a Provider pinned to a compile-time value, for tests and for
// embedders that never wire a real Service. It presents as env-managed because
// it can never change.
func Fixed(enabled bool) Provider { return fixedProvider(enabled) }

type fixedProvider bool

func (f fixedProvider) Enabled() bool    { return bool(f) }
func (f fixedProvider) EnvManaged() bool { return true }
