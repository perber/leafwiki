package settings

import "errors"

// ErrEnvManaged is returned by Value.Put when the value is pinned by an
// environment variable or CLI flag and therefore cannot be changed at
// runtime. A domain layer that needs a localized / HTTP-mapped error tests
// for this with errors.Is and substitutes its own (internal/publicaccess
// maps it to a *LocalizedError with code public_access_env_managed).
var ErrEnvManaged = errors.New("settings: value is managed by environment configuration")

// Value is one settings-managed value of type T. It is either persisted to a
// JSON file in the data directory (Managed, wrapping a *Store[T]) or pinned
// by an env var / flag with no backing file at all (Fixed). Carrying a
// single Value lets a feature drop the (value, envManaged, store) triple and
// the mode branch every accessor otherwise repeats — see the env- vs
// settings-managed split in internal/publicaccess and internal/backup.
type Value[T any] interface {
	Reloadable

	// Get returns the current value.
	Get() T

	// Put persists v and updates the cache. A Fixed value rejects every
	// write with ErrEnvManaged and touches nothing on disk.
	Put(v T) error

	// EnvManaged reports whether this value is env/flag-pinned (and thus
	// read-only in the settings UI).
	EnvManaged() bool
}

// Managed is a Value backed by a JSON file: Get, Put and Reload pass straight
// through to the embedded *Store[T]. Construct it as
// settings.Managed[T]{Store: st}.
type Managed[T any] struct {
	*Store[T]
}

// EnvManaged is always false for a file-backed value.
func (Managed[T]) EnvManaged() bool { return false }

// Fixed is a Value pinned to a boot-time constant. It has no file: Put always
// fails with ErrEnvManaged and Reload is a no-op, so a Fixed value can sit in
// internal/restore's []Reloadable harmlessly (a restore cannot and must not
// change an env-pinned value).
type Fixed[T any] struct {
	v T
}

// NewFixed returns a Fixed value holding v.
func NewFixed[T any](v T) Fixed[T] { return Fixed[T]{v: v} }

// Get returns the pinned value.
func (f Fixed[T]) Get() T { return f.v }

// Put always fails: an env-pinned value cannot be changed at runtime.
func (Fixed[T]) Put(T) error { return ErrEnvManaged }

// Reload is a no-op: there is no file to re-read.
func (Fixed[T]) Reload() error { return nil }

// EnvManaged is always true for a Fixed value.
func (Fixed[T]) EnvManaged() bool { return true }

var (
	_ Value[int] = Managed[int]{}
	_ Value[int] = Fixed[int]{}
)
