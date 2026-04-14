package wiki

import (
	"context"
	"log/slog"
	"mime/multipart"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
	wikiassets "github.com/perber/wiki/internal/wiki/assets"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
)

// WikiImportAdapter implements the importer.ImporterWiki interface using
// the wiki's internal services directly via use cases.
type WikiImportAdapter struct {
	tree     *tree.TreeService
	slug     *tree.SlugService
	revision *revision.Service
	links    *links.LinkService
	asset    *assets.AssetService
	log      *slog.Logger
}

// NewWikiImportAdapter constructs an importer adapter backed by the wiki's
// internal services.
func NewWikiImportAdapter(w *Wiki) *WikiImportAdapter {
	return &WikiImportAdapter{
		tree:     w.tree,
		slug:     w.slug,
		revision: w.revision,
		links:    w.links,
		asset:    w.asset,
		log:      w.log,
	}
}

func (a *WikiImportAdapter) TreeHash() string {
	return a.tree.TreeHash()
}

func (a *WikiImportAdapter) LookupPagePath(path string) (*tree.PathLookup, error) {
	return a.tree.LookupPagePath(path)
}

func (a *WikiImportAdapter) FindByPath(route string) (*tree.Page, error) {
	return a.tree.FindPageByRoutePath(route)
}

func (a *WikiImportAdapter) ListAssets(pageID string) ([]string, error) {
	page, err := a.tree.FindPageByID(pageID)
	if err != nil {
		return nil, err
	}
	return a.asset.ListAssetsForPage(page)
}

func (a *WikiImportAdapter) EnsurePath(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error) {
	out, err := wikipages.NewEnsurePathUseCase(a.tree, a.slug, a.revision, a.links, a.log).Execute(
		context.Background(),
		wikipages.EnsurePathInput{UserID: userID, TargetPath: targetPath, TargetTitle: title, Kind: kind},
	)
	if err != nil {
		return nil, err
	}
	return out.Page, nil
}

func (a *WikiImportAdapter) UpdatePage(userID, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error) {
	out, err := wikipages.NewUpdatePageUseCase(a.tree, a.slug, a.revision, a.links, a.log).Execute(
		context.Background(),
		wikipages.UpdatePageInput{UserID: userID, ID: id, Title: title, Slug: slug, Content: content, Kind: kind},
	)
	if err != nil {
		return nil, err
	}
	return out.Page, nil
}

func (a *WikiImportAdapter) UploadAsset(userID, pageID string, file multipart.File, filename string, maxBytes int64) (string, error) {
	out, err := wikiassets.NewUploadAssetUseCase(a.tree, a.asset, a.revision, a.log).Execute(
		context.Background(),
		wikiassets.UploadAssetInput{UserID: userID, PageID: pageID, File: file, Filename: filename, MaxBytes: maxBytes},
	)
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
