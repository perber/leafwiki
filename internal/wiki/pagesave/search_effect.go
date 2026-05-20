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
	tree  *tree.TreeService
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

func (e *SearchIndexSideEffect) indexPage(page *tree.Page) {
	if page == nil {
		return
	}

	raw := page.Content
	if e.tree != nil {
		loadedRaw, err := e.tree.ReadPageRaw(page.ID)
		if err != nil {
			e.log.Warn("failed to read raw content for search indexing", "pageID", page.ID, "error", err)
		} else {
			raw = loadedRaw
		}
	}

	path := strings.TrimPrefix(page.CalculatePath(), "/")
	filePath := path
	if filePath != "" {
		filePath += ".md"
	}

	if err := e.index.IndexPage(path, filePath, page.ID, page.Title, page.Kind, raw); err != nil {
		e.log.Warn("failed to update search index for page", "pageID", page.ID, "error", err)
	}
}
