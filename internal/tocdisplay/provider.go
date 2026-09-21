package tocdisplay

// Provider is the read-only view of the always-show-TOC flag that the HTTP
// layer depends on. *Service implements it; so does the value returned by
// Fixed.
type Provider interface {
	// AlwaysShow reports whether the TOC should always be shown, regardless
	// of heading count.
	AlwaysShow() bool
}

// Fixed returns a Provider pinned to a compile-time value, for the router's
// default (no Service configured) and for tests.
func Fixed(alwaysShow bool) Provider { return fixedProvider(alwaysShow) }

type fixedProvider bool

func (f fixedProvider) AlwaysShow() bool { return bool(f) }
