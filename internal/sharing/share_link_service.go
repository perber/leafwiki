// Package sharing owns share links (#1252): a random token that lets someone
// without an account read one page, or a page and its subtree. This package
// creates, revokes and resolves links. What a resolved link may read is
// decided by internal/scopedread (ADR-0015); the token never leaves this
// package and the HTTP layer above it.
package sharing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/shared"
	"github.com/perber/wiki/internal/core/tree"
)

// treeRootID is the tree's root; sharing it would be public mode, not a link.
const treeRootID = "root"

// ShareLink is a stored share link. Its token is returned once by Create and
// never stored; tokenHash is only set inside this package.
type ShareLink struct {
	ID                 string
	PageID             string
	IncludeDescendants bool
	ExpiresAt          *time.Time // nil = never expires
	CreatedBy          string     // user ID of the editor or admin who created it
	CreatedAt          time.Time

	tokenHash string
}

// expired reports whether the link has an expiry at or before now.
func (l *ShareLink) expired(now time.Time) bool {
	return l.ExpiresAt != nil && !now.Before(*l.ExpiresAt)
}

// Grant is what a valid token resolves to; callers turn it into a ReadScope.
type Grant struct {
	ShareID            string
	PageID             string
	IncludeDescendants bool
}

// PageLookup checks that a shared page exists. *tree.TreeService implements it.
type PageLookup interface {
	FindPageByID(id string) (*tree.PageNode, error)
}

// CreateShareLinkInput is the input for ShareLinkService.Create.
type CreateShareLinkInput struct {
	PageID             string
	IncludeDescendants bool
	ExpiresAt          *time.Time // nil = never expires; must be in the future
	CreatedBy          string
}

// ShareLinkService manages share links on top of a ShareLinkStore.
type ShareLinkService struct {
	store *ShareLinkStore
	pages PageLookup
	log   *slog.Logger
	now   func() time.Time
}

// NewShareLinkService returns a service over store.
func NewShareLinkService(store *ShareLinkStore, pages PageLookup, log *slog.Logger) *ShareLinkService {
	if log == nil {
		log = slog.Default()
	}
	return &ShareLinkService{
		store: store,
		pages: pages,
		log:   log.With("component", "ShareLinkService"),
		now:   time.Now,
	}
}

// Create stores a new link and returns it with its token, the only time it is returned.
func (s *ShareLinkService) Create(ctx context.Context, in CreateShareLinkInput) (*ShareLink, string, error) {
	if strings.TrimSpace(in.CreatedBy) == "" {
		return nil, "", errors.New("share link creator is required")
	}
	if in.PageID == "" || in.PageID == treeRootID {
		return nil, "", errShareLinkInvalidPage()
	}
	if _, err := s.pages.FindPageByID(in.PageID); err != nil {
		if errors.Is(err, tree.ErrPageNotFound) {
			return nil, "", errShareLinkInvalidPage()
		}
		return nil, "", err
	}
	now := s.now().UTC()
	if in.ExpiresAt != nil && !in.ExpiresAt.After(now) {
		return nil, "", errShareLinkInvalidExpiry()
	}

	id, err := shared.GenerateUniqueID()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate share link id: %w", err)
	}
	token, err := newToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate share link token: %w", err)
	}
	link := &ShareLink{
		ID:                 id,
		PageID:             in.PageID,
		IncludeDescendants: in.IncludeDescendants,
		ExpiresAt:          in.ExpiresAt,
		CreatedBy:          in.CreatedBy,
		CreatedAt:          now,
		tokenHash:          hashToken(token),
	}
	if err := s.store.Insert(ctx, link); err != nil {
		return nil, "", err
	}
	link.tokenHash = ""

	s.log.Info("share link created", "shareID", link.ID, "pageID", link.PageID,
		"includeDescendants", link.IncludeDescendants, "createdBy", link.CreatedBy)
	return link, token, nil
}

// Revoke deletes the link id. The next Resolve of its token fails.
func (s *ShareLinkService) Revoke(ctx context.Context, id string) error {
	removed, err := s.store.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !removed {
		return errShareLinkNotFound()
	}
	s.log.Info("share link revoked", "shareID", id)
	return nil
}

// Resolve returns the grant of a valid, unexpired token; anything else is the
// same share_link_not_found. A restore in progress is store_unavailable instead.
func (s *ShareLinkService) Resolve(ctx context.Context, token string) (*Grant, error) {
	if !looksLikeToken(token) {
		return nil, errShareLinkNotFound()
	}
	link, err := s.store.FindByTokenHash(ctx, hashToken(token))
	if errors.Is(err, errLinkNotStored) {
		return nil, errShareLinkNotFound()
	}
	if err != nil {
		return nil, err
	}
	if link.expired(s.now()) || link.PageID == "" || link.PageID == treeRootID { // e.g. from a hostile restored snapshot
		return nil, errShareLinkNotFound()
	}
	return &Grant{ShareID: link.ID, PageID: link.PageID, IncludeDescendants: link.IncludeDescendants}, nil
}

// PauseForSwap, Replace and Close delegate to the store (restore.SwappableStore).
func (s *ShareLinkService) PauseForSwap() error          { return s.store.PauseForSwap() }
func (s *ShareLinkService) Replace(dataDir string) error { return s.store.Replace(dataDir) }
func (s *ShareLinkService) Close() error                 { return s.store.Close() }
