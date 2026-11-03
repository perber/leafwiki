package wiki

import (
	"context"
	stderrors "errors"
	"log/slog"
	"strings"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/revision"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/favorites"
	"github.com/perber/wiki/internal/http/dto"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/search"
	"github.com/perber/wiki/internal/tags"
	wikimcp "github.com/perber/wiki/internal/wiki/mcp"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	"github.com/perber/wiki/internal/wiki/pagesave"
	wikisearch "github.com/perber/wiki/internal/wiki/search"
)

// WikiMCPAdapter implements mcp.PageOps by delegating to the existing
// pages/search use cases (ADR-0018). It is the only place the MCP slice meets
// the wiki's services: the slice never talks to internal/core or the
// top-level index packages directly, it calls PageOps, and this adapter fills
// in the slice's DTOs. The write ops use the same use case constructors as
// the REST routes (buildPagesRoutes), so pagesave events, Before/After
// capture, AffectedPages and the asset/favorite/revision cleanup fire
// exactly as through HTTP.
type WikiMCPAdapter struct {
	tree         *tree.TreeService
	searchIndex  *search.SQLiteIndex
	tags         *tags.TagsService
	slug         *tree.SlugService
	orchestrator *pagesave.PageSaveOrchestrator
	revision     *revision.Service
	assets       *assets.AssetService
	favorites    *favorites.FavoritesStore
	log          *slog.Logger
	metrics      *httpmetrics.HTTPMetrics
}

// NewWikiMCPAdapter constructs a WikiMCPAdapter over the wiki's services.
func NewWikiMCPAdapter(w *Wiki) *WikiMCPAdapter {
	return &WikiMCPAdapter{
		tree:         w.tree,
		searchIndex:  w.searchIndex,
		tags:         w.tags,
		slug:         w.slug,
		orchestrator: w.newPageOrchestrator(),
		revision:     w.revision,
		assets:       w.asset,
		favorites:    w.favorites,
		log:          w.log,
		metrics:      w.metrics,
	}
}

// GetPage reads a page and its metadata by stable ID, delegating to the
// existing get-page-by-ID use case.
func (a *WikiMCPAdapter) GetPage(ctx context.Context, id string) (*wikimcp.Page, error) {
	view, err := wikipages.NewGetPageViewByIDUseCase(a.tree).Execute(ctx, wikipages.GetPageViewByIDInput{ID: id})
	if err != nil {
		return nil, mapReadError(err)
	}
	return mcpPageFromView(view), nil
}

// GetPageByPath reads a page and its metadata by route path, delegating to the
// existing get-page-by-path use case.
func (a *WikiMCPAdapter) GetPageByPath(ctx context.Context, routePath string) (*wikimcp.Page, error) {
	view, err := wikipages.NewGetPageViewByPathUseCase(a.tree).Execute(ctx, wikipages.GetPageViewByPathInput{RoutePath: routePath})
	if err != nil {
		return nil, mapReadError(err)
	}
	return mcpPageFromView(view), nil
}

// mapReadError converts the tree sentinels the page-read use cases pass
// through into the LocalizedErrors the REST route would return for them
// (respondWithPageError), so the tools report the same client-facing codes
// and the tool error mapper does not fall back to mcp_internal_error. The
// other tree sentinels are not reachable from these read ops:
// ErrParentNotFound is only produced by mutations (create/move), and the
// search and tree-list ops do not surface a tree sentinel at all.
func mapReadError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case stderrors.Is(err, tree.ErrPageNotFound):
		return sharederrors.NewLocalizedError(wikipages.ErrCodePageNotFound, "Page not found", "page not found", err)
	case stderrors.Is(err, tree.ErrTreeNotLoaded):
		return sharederrors.NewLocalizedError(wikipages.ErrCodePageInternalError, "Tree not loaded", "tree not loaded", err)
	default:
		return err
	}
}

// CreatePage creates a page or section through the create-page use case the
// REST create route uses, and (when content is supplied) sets the body with
// the update-page use case — the same two steps the editor performs through
// the UI, so the pagesave events fire the same way. An omitted slug is
// suggested by the same SuggestSlugUseCase the REST slug-suggestion route
// uses, so an unknown parent surfaces with the code the REST create route
// answers for one (page_not_found, via the use case's raw tree error) and an
// omitted one deduplicates among the siblings exactly like the UI.
func (a *WikiMCPAdapter) CreatePage(ctx context.Context, in wikimcp.CreatePageInput) (*wikimcp.Page, error) {
	kind := tree.NodeKindPage
	if in.Kind == "section" {
		kind = tree.NodeKindSection
	}
	slug := in.Slug
	if slug == "" {
		parentID := ""
		if in.ParentID != nil {
			parentID = *in.ParentID
		}
		out, err := wikipages.NewSuggestSlugUseCase(a.tree, a.slug).Execute(ctx, wikipages.SuggestSlugInput{
			ParentID: parentID,
			Title:    in.Title,
		})
		if err != nil {
			return nil, mapWriteError(err)
		}
		slug = out.Slug
	}

	out, err := wikipages.NewCreatePageUseCase(a.tree, a.slug, a.orchestrator, a.log, a.metrics).Execute(ctx, wikipages.CreatePageInput{
		UserID: in.Actor.ID, ParentID: in.ParentID, Title: in.Title, Slug: slug, Kind: &kind,
	})
	if err != nil {
		return nil, mapWriteError(err)
	}
	page := out.Page
	if in.Content != "" {
		content := in.Content
		updated, err := a.updatePageChange(ctx, in.Actor.ID, page, "", page.Slug, &content)
		if err != nil {
			a.removeHalfCreated(ctx, in.Actor.ID, page, err)
			return nil, err
		}
		page = updated
	}
	return mcpPageFromPage(page), nil
}

// updatePageChange runs the update-page use case the REST update route uses
// for a known page: the kind comes from the page; an empty title or slug
// keeps the page's current value, content is the new body or nil to keep the
// current one, and the change is checked against the page's current version
// (optimistic locking, the way the UI's saves go). The returned page is the
// live node after the change.
func (a *WikiMCPAdapter) updatePageChange(ctx context.Context, actorID string, page *tree.Page, title, slug string, content *string) (*tree.Page, error) {
	if title == "" {
		title = page.Title
	}
	out, err := wikipages.NewUpdatePageUseCase(a.tree, a.slug, a.orchestrator, a.log, a.metrics).Execute(ctx, wikipages.UpdatePageInput{
		UserID: actorID, ID: page.ID, Version: page.Version(),
		Title: title, Slug: slug, Content: content, Kind: &page.Kind,
	})
	if err != nil {
		return nil, mapWriteError(err)
	}
	return out.Page, nil
}

// removeHalfCreated deletes a page that was created but could not be finished
// (a failed content set), so a failed create leaves no orphan page behind.
// A version conflict is the one failure that must not trigger cleanup: it
// means a concurrent writer already saved to the page, and deleting it would
// destroy that save. Otherwise best effort: a failing cleanup is logged, the
// original error is the one reported.
func (a *WikiMCPAdapter) removeHalfCreated(ctx context.Context, actorID string, page *tree.Page, writeErr error) {
	if loc, ok := sharederrors.AsLocalizedError(writeErr); ok && loc.Code == wikipages.ErrCodePageVersionConflict {
		return
	}
	err := wikipages.NewDeletePageUseCase(a.tree, a.revision, a.assets, a.favorites, a.orchestrator, a.log, a.metrics).Execute(ctx, wikipages.DeletePageInput{
		UserID: actorID, ID: page.ID, Version: page.Version(), Recursive: false,
	})
	if err != nil {
		a.log.Warn("failed to remove half-created page", "pageID", page.ID, "error", err)
	}
}

// UpdatePage fully replaces a page's title and/or content through the
// update-page use case the REST update route uses. The REST contract is a
// full replace (title, slug, content all supplied); the tool's omitted fields
// are filled from the live page first, so an omitted title or content keeps
// the current value.
func (a *WikiMCPAdapter) UpdatePage(ctx context.Context, in wikimcp.UpdatePageInput) (*wikimcp.Page, error) {
	view, err := a.pageViewByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	// An empty title keeps the current one; nil content keeps the current
	// body (the update-page use case's full-replace contract, with the
	// tool's omitted fields filled from the live page).
	if _, err := a.updatePageChange(ctx, in.Actor.ID, view.Page, in.Title, view.Page.Slug, in.Content); err != nil {
		return nil, err
	}
	// A content replacement rewrites the page's frontmatter, so the result's
	// tags and properties come from a fresh view (the REST update route
	// re-reads them for exactly this reason).
	fresh, err := a.pageViewByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return mcpPageFromView(fresh), nil
}

// PatchPage applies a line-range patch to a page's body: it reads the live
// page, applies the patch, and writes the result through the update-page
// use case under the read version, so a concurrent edit is a clean
// page_version_conflict instead of a silent overwrite.
func (a *WikiMCPAdapter) PatchPage(ctx context.Context, in wikimcp.PatchPageInput) (*wikimcp.Page, error) {
	view, err := a.pageViewByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	patched, err := wikimcp.ApplyPatch(view.Page.Content, in.Ranges)
	if err != nil {
		return nil, patchFieldError(err)
	}
	content := patched
	if _, err := a.updatePageChange(ctx, in.Actor.ID, view.Page, "", view.Page.Slug, &content); err != nil {
		return nil, err
	}
	// The patch rewrites the page's body, so the result's tags and properties
	// come from a fresh view (as UpdatePage does; the REST update route
	// re-reads them for its response the same way).
	fresh, err := a.pageViewByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return mcpPageFromView(fresh), nil
}

// patchFieldError reports an invalid patch (a range out of line bounds or
// overlapping another) as a validation error on the patch field, which the
// tool error mapper renders as mcp_invalid_input.
func patchFieldError(err error) error {
	ve := sharederrors.NewValidationErrors()
	ve.Add("patch", err.Error())
	return ve
}

// pageViewByID runs the get-page-view-by-ID use case and maps its errors like
// the read ops do.
func (a *WikiMCPAdapter) pageViewByID(ctx context.Context, id string) (*wikipages.PageView, error) {
	view, err := wikipages.NewGetPageViewByIDUseCase(a.tree).Execute(ctx, wikipages.GetPageViewByIDInput{ID: id})
	if err != nil {
		return nil, mapReadError(err)
	}
	return view, nil
}

// mapWriteError converts a page-write use case error onto the LocalizedError
// the REST route would return for it (respondWithPageError), so the tools
// report the same client-facing codes and the tool error mapper does not fall
// back to mcp_internal_error. LocalizedErrors and validation errors pass
// through unchanged; everything else goes through the pages slice's shared
// sentinel table (MapPageError), which also covers the internal-error arm a
// wrapped storage failure lands in.
func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := sharederrors.AsLocalizedError(err); ok {
		return err
	}
	var vErr *sharederrors.ValidationErrors
	if stderrors.As(err, &vErr) {
		return vErr
	}
	return wikipages.MapPageError(err)
}

// DeletePage deletes a page (optionally its whole subtree) through the
// delete-page use case the REST delete route uses: assets, favorites and
// revision data are cleaned up exactly as through HTTP. The delete is checked
// against the page's current version, like a REST client's save.
func (a *WikiMCPAdapter) DeletePage(ctx context.Context, in wikimcp.DeletePageInput) error {
	view, err := a.pageViewByID(ctx, in.ID)
	if err != nil {
		return err
	}
	err = wikipages.NewDeletePageUseCase(a.tree, a.revision, a.assets, a.favorites, a.orchestrator, a.log, a.metrics).Execute(ctx, wikipages.DeletePageInput{
		UserID: in.Actor.ID, ID: in.ID, Version: view.Page.Version(), Recursive: in.Recursive,
	})
	return mapWriteError(err)
}

// MovePage moves a page to a new route path. The last path segment is the
// new slug and the remaining segments name the destination parent, resolved
// by route path like the REST lookup; an empty path moves the page to the
// wiki root without touching its slug. A changed slug goes through the
// update-page use case and the parent change through the move-page use case,
// so the link/search side effects for the affected subtree fire as for
// manual edits. Self and circular destinations are rejected before any
// mutation, so those failures leave the page exactly where it was; a
// destination slug collision is only discovered by the move itself, so that
// failure may leave the new slug in place (stated in the tool description).
func (a *WikiMCPAdapter) MovePage(ctx context.Context, in wikimcp.MovePageInput) error {
	view, err := a.pageViewByID(ctx, in.ID)
	if err != nil {
		return err
	}
	page := view.Page

	newSlug := ""
	parentPath := ""
	if in.Path != "" {
		newSlug = in.Path
		if i := strings.LastIndex(in.Path, "/"); i >= 0 {
			newSlug, parentPath = in.Path[i+1:], in.Path[:i]
		}
	}

	parentID := ""
	if parentPath != "" {
		dest, err := wikipages.NewGetPageViewByPathUseCase(a.tree).Execute(ctx, wikipages.GetPageViewByPathInput{RoutePath: parentPath})
		if err != nil {
			// The REST move route reports an unknown parent with
			// page_parent_not_found; keep that code.
			if stderrors.Is(err, tree.ErrPageNotFound) {
				return sharederrors.NewLocalizedError(wikipages.ErrCodePageParentNotFound, "Parent page not found", "parent page not found", err)
			}
			return mapReadError(err)
		}
		// Pre-validate before any mutation: moving into the page itself
		// or its own subtree would otherwise leave the already-committed
		// slug change behind (the move use case rejects both, but only
		// after the update ran).
		if dest.Page.ID == page.ID {
			return sharederrors.NewLocalizedError(wikipages.ErrCodePageCannotMoveToSelf, "Page cannot be moved to itself", "page cannot be moved to itself", tree.ErrPageCannotBeMovedToItself)
		}
		if page.IsChildOf(dest.Page.ID, true) {
			return sharederrors.NewLocalizedError(wikipages.ErrCodePageCircularMove, "Move would create a circular reference", "circular reference detected", tree.ErrMovePageCircularReference)
		}
		parentID = dest.Page.ID
	}

	if newSlug != "" && newSlug != page.Slug {
		out, err := a.updatePageChange(ctx, in.Actor.ID, page, "", newSlug, nil)
		if err != nil {
			return err
		}
		page = out
	}

	err = wikipages.NewMovePageUseCase(a.tree, a.orchestrator, a.log, a.metrics).Execute(ctx, wikipages.MovePageInput{
		UserID: in.Actor.ID, ID: in.ID, Version: page.Version(), ParentID: parentID,
	})
	return mapWriteError(err)
}

// EnsurePath ensures a route path exists — the missing segments become
// sections and the last segment takes the given kind — through the
// ensure-path use case the REST ensure route uses (the same constructor the
// import adapter already delegates through). A path that already exists is
// returned unchanged, so a repeated call creates nothing and never clobbers
// an existing node's title.
func (a *WikiMCPAdapter) EnsurePath(ctx context.Context, in wikimcp.EnsurePathInput) (*wikimcp.Page, error) {
	kind := tree.NodeKindPage
	if in.Kind == "section" {
		kind = tree.NodeKindSection
	}
	out, err := wikipages.NewEnsurePathUseCase(a.tree, a.slug, a.orchestrator, a.log).Execute(ctx, wikipages.EnsurePathInput{
		UserID: in.Actor.ID, TargetPath: in.Path, TargetTitle: in.Title, Kind: &kind,
	})
	if err != nil {
		return nil, mapWriteError(err)
	}
	return mcpPageFromPage(out.Page), nil
}

// ConvertNode converts a page to a section, or a section to a page, through
// the convert-page use case the REST convert route uses. The version is taken
// from the live page (a tool caller does not track versions), and a
// conversion the tree refuses — a section with children to a page — surfaces
// with the REST-parity code page_has_children (the mapper's
// page_convert_not_allowed arm covers the node store's not-empty-folder
// refusal, reachable only from an on-disk drift state).
func (a *WikiMCPAdapter) ConvertNode(ctx context.Context, in wikimcp.ConvertNodeInput) error {
	view, err := a.pageViewByID(ctx, in.ID)
	if err != nil {
		return err
	}
	err = wikipages.NewConvertPageUseCase(a.tree, a.revision, a.log).Execute(ctx, wikipages.ConvertPageInput{
		UserID: in.Actor.ID, ID: in.ID, Version: view.Page.Version(), TargetKind: tree.NodeKind(in.Kind),
	})
	return mapWriteError(err)
}

// CopyPage copies a page and its assets under a new title and slug through
// the copy-page use case the REST copy route uses — the single-page semantics
// (the children stay with the source). A destination slug already taken under
// the same parent surfaces with the REST-parity code page_slug_conflict.
func (a *WikiMCPAdapter) CopyPage(ctx context.Context, in wikimcp.CopyPageInput) (*wikimcp.Page, error) {
	out, err := wikipages.NewCopyPageUseCase(a.tree, a.slug, a.orchestrator, a.assets, a.log).Execute(ctx, wikipages.CopyPageInput{
		UserID: in.Actor.ID, SourcePageID: in.SourceID, TargetParentID: in.ParentID,
		Title: in.Title, Slug: in.Slug,
	})
	if err != nil {
		return nil, mapWriteError(err)
	}
	return mcpPageFromPage(out.Page), nil
}

// Search runs a full-text search, delegating to the existing search use case.
func (a *WikiMCPAdapter) Search(ctx context.Context, query string, limit int) (*wikimcp.SearchResult, error) {
	out, err := wikisearch.NewSearchUseCase(a.searchIndex, a.tags, a.tree).Execute(
		ctx, wikisearch.SearchInput{Query: query, Limit: limit},
	)
	if err != nil {
		return nil, err
	}
	return mcpSearchFromResult(out.Result), nil
}

// ListTree returns the wiki's page tree (top-level nodes, synthetic root
// stripped), reading it straight from the tree like the pages route does.
func (a *WikiMCPAdapter) ListTree(_ context.Context) ([]wikimcp.TreeNode, error) {
	root := a.tree.GetTree()
	if root == nil {
		return nil, nil
	}
	return mcpTreeFromNodes(root.Children), nil
}

// mcpPageFromView maps an existing page view (node + content + metadata) onto
// the slice's Page DTO.
func mcpPageFromView(view *wikipages.PageView) *wikimcp.Page {
	page := mcpPageFromPage(view.Page)
	if page == nil {
		return nil
	}
	page.Tags = view.Tags
	page.Properties = view.Properties
	return page
}

// mcpPageFromPage maps a tree page onto the slice's Page DTO without the
// frontmatter metadata, which the write ops' use case outputs do not carry.
func mcpPageFromPage(page *tree.Page) *wikimcp.Page {
	if page == nil || page.PageNode == nil {
		return nil
	}
	return &wikimcp.Page{
		ID:        page.ID,
		Title:     page.Title,
		Path:      dto.BuildPathFromNode(page.PageNode),
		Kind:      string(page.Kind),
		Content:   page.Content,
		CreatedAt: page.Metadata.CreatedAt,
		UpdatedAt: page.Metadata.UpdatedAt,
	}
}

// mcpSearchFromResult maps a search result onto the slice's SearchResult DTO,
// keeping only the fields a client needs to act on a hit.
func mcpSearchFromResult(result *search.SearchResult) *wikimcp.SearchResult {
	if result == nil {
		return &wikimcp.SearchResult{}
	}
	items := make([]wikimcp.SearchItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, wikimcp.SearchItem{
			ID:      item.PageID,
			Title:   item.Title,
			Path:    item.Path,
			Kind:    item.Kind,
			Excerpt: item.Excerpt,
		})
	}
	return &wikimcp.SearchResult{Count: result.Count, Items: items}
}

// mcpTreeFromNodes maps a level of tree nodes (and their descendants) onto the
// slice's TreeNode DTOs, building each node's route path as it descends.
func mcpTreeFromNodes(nodes []*tree.PageNode) []wikimcp.TreeNode {
	result := make([]wikimcp.TreeNode, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		result = append(result, mcpTreeNodeFromNode(node, ""))
	}
	return result
}

func mcpTreeNodeFromNode(node *tree.PageNode, parentPath string) wikimcp.TreeNode {
	path := node.Slug
	if parentPath != "" {
		path = parentPath + "/" + node.Slug
	}
	children := make([]wikimcp.TreeNode, 0, len(node.Children))
	for _, child := range node.Children {
		if child == nil {
			continue
		}
		children = append(children, mcpTreeNodeFromNode(child, path))
	}
	return wikimcp.TreeNode{
		ID:       node.ID,
		Title:    node.Title,
		Slug:     node.Slug,
		Kind:     string(node.Kind),
		Path:     path,
		Children: children,
	}
}
