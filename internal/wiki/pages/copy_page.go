package pages

import (
	"context"
	"log"
	"log/slog"
	"strings"

	"github.com/perber/wiki/internal/core/assets"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

// CopyPageInput is the input for CopyPageUseCase.
type CopyPageInput struct {
	UserID          string
	SourcePageID    string
	TargetParentID  *string
	Title           string
	Slug            string
}

// CopyPageOutput is the output of CopyPageUseCase.
type CopyPageOutput struct {
	Page *tree.Page
}

// CopyPageUseCase duplicates a page and its assets under a new slug/title.
type CopyPageUseCase struct {
	tree     *tree.TreeService
	slug     *tree.SlugService
	revision *revision.Service
	links    *links.LinkService
	assets   *assets.AssetService
	log      *slog.Logger
}

// NewCopyPageUseCase constructs a CopyPageUseCase.
func NewCopyPageUseCase(
	t *tree.TreeService,
	s *tree.SlugService,
	r *revision.Service,
	l *links.LinkService,
	a *assets.AssetService,
	log *slog.Logger,
) *CopyPageUseCase {
	return &CopyPageUseCase{tree: t, slug: s, revision: r, links: l, assets: a, log: log}
}

// Execute copies the source page to a new node with duplicated assets.
func (uc *CopyPageUseCase) Execute(_ context.Context, in CopyPageInput) (*CopyPageOutput, error) {
	ve := sharederrors.NewValidationErrors()
	if in.Title == "" {
		ve.Add("title", "Title must not be empty")
	}
	if err := uc.slug.IsValidSlug(in.Slug); err != nil {
		ve.Add("slug", err.Error())
	}
	if ve.HasErrors() {
		return nil, ve
	}

	page, err := uc.tree.GetPage(in.SourcePageID)
	if err != nil {
		return nil, err
	}

	kind := tree.NodeKindPage
	copyID, err := uc.tree.CreateNode(in.UserID, in.TargetParentID, in.Title, in.Slug, &kind)
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = uc.tree.DeleteNode(in.UserID, *copyID, false) }

	copyPage, err := uc.tree.GetPage(*copyID)
	if err != nil {
		cleanup()
		return nil, err
	}

	if err := uc.assets.CopyAllAssets(page.PageNode, copyPage.PageNode); err != nil {
		cleanup()
		return nil, err
	}

	updatedContent := strings.ReplaceAll(page.Content, "/assets/"+page.ID+"/", "/assets/"+copyPage.ID+"/")
	if err := uc.tree.UpdateNode(in.UserID, copyPage.ID, copyPage.Title, copyPage.Slug, &updatedContent); err != nil {
		cleanup()
		_ = uc.assets.DeleteAllAssetsForPage(copyPage.PageNode)
		return nil, err
	}

	if uc.links != nil {
		if err := uc.links.HealLinksForExactPath(copyPage); err != nil {
			log.Printf("warning: failed to heal links for copied page %s: %v", copyPage.ID, err)
		}
	}

	if uc.revision != nil {
		recordContentRevision(uc.revision, uc.log, copyPage.ID, in.UserID, "page copied")
	}

	return &CopyPageOutput{Page: copyPage}, nil
}
