package pagesave

import (
	"context"
	"log/slog"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/search"
)

// SearchIndexSideEffect updates the search index after every page mutation.
type SearchIndexSideEffect struct {
	index   *search.SQLiteIndex
	tree    *tree.TreeService // only used by IndexAllPages for the initial walk
	log     *slog.Logger
	metrics *httpmetrics.HTTPMetrics
}

func NewSearchIndexSideEffect(index *search.SQLiteIndex, treeService *tree.TreeService, log *slog.Logger, metrics *httpmetrics.HTTPMetrics) *SearchIndexSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &SearchIndexSideEffect{index: index, tree: treeService, log: log, metrics: metrics}
}

func (e *SearchIndexSideEffect) Name() string {
	return "search"
}

func (e *SearchIndexSideEffect) Apply(event PageSaveEvent) {
	if e.index == nil {
		return
	}

	switch event.Operation {
	case PageOperationCreate, PageOperationUpdate, PageOperationRestore:
		if event.After != nil {
			e.indexPage(event.After, event.Operation)
		}

	case PageOperationMove:
		inputs := make([]search.IndexPageInput, 0, len(event.AffectedPages))
		for _, page := range event.AffectedPages {
			inputs = append(inputs, buildIndexInput(page))
		}
		failures, err := e.index.IndexPages(inputs)
		if err != nil {
			e.log.Warn("failed to batch-update search index for moved subtree", "pageCount", len(inputs), "error", err)
			e.metrics.IncPageSaveSideEffectFailure(string(event.Operation), e.Name())
		}
		for _, f := range failures {
			e.log.Warn("failed to update search index for page", "pageID", f.PageID, "error", f.Err)
			e.metrics.IncPageSaveSideEffectFailure(string(event.Operation), e.Name())
		}

	case PageOperationDelete:
		ids := make([]string, 0, len(event.AffectedPages))
		for _, page := range event.AffectedPages {
			ids = append(ids, page.ID)
		}
		if err := e.index.RemovePages(ids); err != nil {
			e.log.Warn("failed to batch-remove deleted subtree from search index", "pageCount", len(ids), "error", err)
			e.metrics.IncPageSaveSideEffectFailure(string(event.Operation), e.Name())
		}
	}
}

// buildIndexInput computes the (path, filePath) pair IndexPage/IndexPages
// need for page, mirroring writeToIndex's single-page logic.
func buildIndexInput(page *tree.Page) search.IndexPageInput {
	path := strings.TrimPrefix(page.CalculatePath(), "/")
	filePath := path
	if filePath != "" {
		filePath += ".md"
	}
	return search.IndexPageInput{
		Path: path, FilePath: filePath, PageID: page.ID, Title: page.Title, Kind: page.Kind, Raw: page.RawContent,
	}
}

// IndexAllPages clears the search index and rebuilds it from the current tree state.
// Call this once at startup; runtime updates are handled via Apply.
func (e *SearchIndexSideEffect) IndexAllPages() error {
	return e.IndexAllPagesContext(context.Background())
}

func (e *SearchIndexSideEffect) IndexAllPagesContext(ctx context.Context) error {
	if e.index == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.index.Clear(); err != nil {
		return err
	}

	var ids []string
	if err := e.tree.WalkNodes(func(id string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		ids = append(ids, id)
		return nil
	}); err != nil {
		return err
	}

	pages, errs := e.tree.GetPages(ids)
	for i, page := range pages {
		if err := ctx.Err(); err != nil {
			return err
		}
		if errs[i] != nil {
			e.log.Warn("skipping page during search bootstrap", "pageID", ids[i], "error", errs[i])
			continue
		}
		e.writeToIndex(page, page.RawContent, "")
	}
	return nil
}

func (e *SearchIndexSideEffect) indexPage(page *tree.Page, operation PageOperationType) {
	if page == nil {
		return
	}
	e.writeToIndex(page, page.RawContent, operation)
}

func (e *SearchIndexSideEffect) writeToIndex(page *tree.Page, content string, operation PageOperationType) {
	path := strings.TrimPrefix(page.CalculatePath(), "/")
	filePath := path
	if filePath != "" {
		filePath += ".md"
	}
	if err := e.index.IndexPage(path, filePath, page.ID, page.Title, page.Kind, content); err != nil {
		e.log.Warn("failed to update search index for page", "pageID", page.ID, "error", err)
		if operation != "" {
			e.metrics.IncPageSaveSideEffectFailure(string(operation), e.Name())
		}
	}
}
