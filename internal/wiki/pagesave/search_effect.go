package pagesave

import (
	"log/slog"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/search"
)

// SearchIndexSideEffect updates the search index after every page mutation.
type SearchIndexSideEffect struct {
	index *search.SQLiteIndex
	tree  *tree.TreeService // only used by IndexAllPages for the initial walk
	log   *slog.Logger
}

func NewSearchIndexSideEffect(index *search.SQLiteIndex, treeService *tree.TreeService, log *slog.Logger) *SearchIndexSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &SearchIndexSideEffect{index: index, tree: treeService, log: log}
}

func (e *SearchIndexSideEffect) Apply(event PageSaveEvent) {
	if e.index == nil {
		return
	}

	switch event.Operation {
	case PageOperationCreate, PageOperationUpdate, PageOperationRestore:
		if event.After != nil {
			e.indexPage(event.After)
		}

	case PageOperationMove:
		for _, page := range event.AffectedPages {
			e.indexPage(page)
		}

	case PageOperationDelete:
		for _, page := range event.AffectedPages {
			if err := e.index.RemovePage(page.ID); err != nil {
				e.log.Warn("failed to remove page from search index", "pageID", page.ID, "error", err)
			}
		}
	}
}

// IndexAllPages clears the search index and rebuilds it from the current tree state.
// Call this once at startup; runtime updates are handled via Apply.
func (e *SearchIndexSideEffect) IndexAllPages() error {
	if e.index == nil {
		return nil
	}
	if err := e.index.Clear(); err != nil {
		return err
	}

	var ids []string
	if err := e.tree.WalkNodes(func(id string) error {
		ids = append(ids, id)
		return nil
	}); err != nil {
		return err
	}

	pages, errs := e.tree.GetPages(ids)
	for i, page := range pages {
		if errs[i] != nil {
			e.log.Warn("skipping page during search bootstrap", "pageID", ids[i], "error", errs[i])
			continue
		}
		e.writeToIndex(page, page.RawContent)
	}
	return nil
}

func (e *SearchIndexSideEffect) indexPage(page *tree.Page) {
	if page == nil {
		return
	}
	e.writeToIndex(page, page.RawContent)
}

func (e *SearchIndexSideEffect) writeToIndex(page *tree.Page, content string) {
	path := strings.TrimPrefix(page.CalculatePath(), "/")
	filePath := path
	if filePath != "" {
		filePath += ".md"
	}
	if err := e.index.IndexPage(path, filePath, page.ID, page.Title, page.Kind, content); err != nil {
		e.log.Warn("failed to update search index for page", "pageID", page.ID, "error", err)
	}
}
