// Package scopedread is the read boundary for callers that may see only part
// of the wiki, such as share links (#1252). Core services describe what
// exists; this package decides what a scoped caller may observe, so the
// scope never has to be known by TreeService, search, links or tags.
// See ADR-0015.
//
// Rules for code in and around this package:
//
//   - A ReadScope is resolved at the feature boundary (e.g. from a share
//     token) and passed in here. Tokens and users never reach this package or
//     the core services below it.
//   - Readers check the scope before they read content or files.
//   - Readers return projections with an explicit field allow-list, never
//     the normal API DTOs or core types.
//   - Anything out of scope, unknown or deleted is the same ErrNotFound, so a
//     scoped caller cannot probe which pages exist.
package scopedread

import (
	"errors"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
)

// ErrNotFound is returned for a page that is out of scope, unknown or deleted.
// The three cases are deliberately indistinguishable.
var ErrNotFound = errors.New("not found in read scope")

// ErrInvalidScope is returned for a scope that names no page, the whole wiki,
// or an ID with surrounding whitespace. Exposing the whole wiki is public
// mode, not a scope.
var ErrInvalidScope = errors.New("invalid read scope")

// treeRootID is the ID of the tree's synthetic root node (see
// core/tree.getNodeByIDLocked); a scope on it would expose everything.
const treeRootID = "root"

// ReadScope is the part of the tree a scoped caller may read: the page RootID
// alone or, with IncludeDescendants, RootID and every page below it as the
// tree is at the time of the read. Pages created or moved below RootID later
// become visible; pages moved out stop being visible.
type ReadScope struct {
	RootID             string
	IncludeDescendants bool
}

// subtreeChecker is the slice of the tree a scope check needs.
type subtreeChecker interface {
	IsInSubtree(id, rootID string) (bool, error)
}

func (s ReadScope) validate() error {
	root := strings.TrimSpace(s.RootID)
	if root == "" || root == treeRootID || root != s.RootID {
		return ErrInvalidScope
	}
	return nil
}

// check returns nil if id is inside the scope and ErrNotFound if it is not.
// It never reads page content.
func (s ReadScope) check(src subtreeChecker, id string) error {
	if id == "" {
		return ErrNotFound
	}
	if !s.IncludeDescendants {
		if id != s.RootID {
			return ErrNotFound
		}
		return nil
	}
	in, err := src.IsInSubtree(id, s.RootID)
	if errors.Is(err, tree.ErrPageNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !in {
		return ErrNotFound
	}
	return nil
}
