package pagesave

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/tags"
)

// TagsSideEffect updates the tag index after every page mutation.
type TagsSideEffect struct {
	svc *tags.TagsService
	log *slog.Logger
}

func NewTagsSideEffect(svc *tags.TagsService, log *slog.Logger) *TagsSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &TagsSideEffect{svc: svc, log: log}
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
	t := tags.ExtractTagsFromContent(p.Content)
	if err := e.svc.SetTagsForPage(p.ID, t); err != nil {
		e.log.Warn("failed to set tags for page", "pageID", p.ID, "error", err)
	}
}

func (e *TagsSideEffect) deleteTags(p *tree.Page) {
	if err := e.svc.DeleteTagsForPage(p.ID); err != nil {
		e.log.Warn("failed to delete tags for page", "pageID", p.ID, "error", err)
	}
}
