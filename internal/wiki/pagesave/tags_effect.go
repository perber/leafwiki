package pagesave

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/tags"
)

// TagsSideEffect updates the tag index after every page mutation.
type TagsSideEffect struct {
	svc  *tags.TagsService
	tree *tree.TreeService
	log  *slog.Logger
}

func NewTagsSideEffect(svc *tags.TagsService, treeService *tree.TreeService, log *slog.Logger) *TagsSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &TagsSideEffect{svc: svc, tree: treeService, log: log}
}

func (e *TagsSideEffect) Apply(event PageSaveEvent) {
	if e.svc == nil {
		return
	}
	switch event.Operation {
	case PageOperationCreate, PageOperationUpdate, PageOperationRestore:
		if event.After != nil {
			e.setTags(event.After)
		}

	case PageOperationMove:
		// page_id is stable across moves; tags in frontmatter are unchanged — no-op.

	case PageOperationDelete:
		for _, p := range event.AffectedPages {
			e.deleteTags(p)
		}
	}
}

func (e *TagsSideEffect) setTags(p *tree.Page) {
	content := p.Content
	if e.tree != nil {
		raw, err := e.tree.ReadPageRaw(p.ID)
		if err != nil {
			e.log.Warn("failed to read raw content for tag indexing", "pageID", p.ID, "error", err)
		} else {
			content = raw
		}
	}

	if err := e.svc.IndexPageContent(p.ID, content); err != nil {
		e.log.Warn("failed to index page content", "pageID", p.ID, "error", err)
	}
}

func (e *TagsSideEffect) deleteTags(p *tree.Page) {
	if err := e.svc.DeletePageIndex(p.ID); err != nil {
		e.log.Warn("failed to delete page index", "pageID", p.ID, "error", err)
	}
}
