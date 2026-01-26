package tree

import (
	"errors"
	"fmt"
)

var ErrPageNotFound = errors.New("page not found")
var ErrParentNotFound = errors.New("parent not found")
var ErrTreeNotLoaded = errors.New("tree not loaded")
var ErrPageHasChildren = errors.New("page has children")
var ErrPageAlreadyExists = errors.New("page already exists")
var ErrMovePageCircularReference = errors.New("circular reference detected")
var ErrPageCannotBeMovedToItself = errors.New("page cannot be moved to itself")
var ErrInvalidSortOrder = errors.New("invalid sort order")
var ErrFileNotFound = errors.New("file not found")
var ErrDrift = errors.New("drift detected")
var ErrInvalidOperation = errors.New("invalid operation")
var ErrConvertNotAllowed = errors.New("convert not allowed")

// DriftError represents a drift error with detailed information.
type DriftError struct {
	NodeID string
	Kind   NodeKind
	Path   string
	Reason string
}

func (e *DriftError) Error() string {
	return "drift detected: nodeID=" + e.NodeID + ", kind=" + string(e.Kind) + ", path=" + e.Path + ", reason=" + e.Reason
}

func (e *DriftError) Unwrap() error {
	return ErrDrift
}

// InvalidOpError represents an invalid operation error with details.
type InvalidOpError struct {
	Op     string
	Reason string
}

func (e *InvalidOpError) Error() string { return fmt.Sprintf("%s: %s", e.Op, e.Reason) }
func (e *InvalidOpError) Unwrap() error { return ErrInvalidOperation }

// PageAlreadyExistsError: Konflikt bei Create/Move/Rename
type PageAlreadyExistsError struct {
	Path string
}

func (e *PageAlreadyExistsError) Error() string { return fmt.Sprintf("already exists: %s", e.Path) }
func (e *PageAlreadyExistsError) Unwrap() error { return ErrPageAlreadyExists }

// NotFoundError represents a not found error with details.
type NotFoundError struct {
	Resource string
	ID       string
	Path     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

func (e *NotFoundError) Unwrap() error {
	return ErrPageNotFound
}

// ConvertNotAllowedError represents a convert not allowed error with details.
type ConvertNotAllowedError struct {
	From   NodeKind
	To     NodeKind
	Reason string
}

func (e *ConvertNotAllowedError) Error() string {
	return fmt.Sprintf("cannot convert from %s to %s: %s", e.From, e.To, e.Reason)
}

func (e *ConvertNotAllowedError) Unwrap() error {
	return ErrConvertNotAllowed
}
