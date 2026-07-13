package pagesave

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/tags"
)

// TagsSideEffect updates the tag index after every page mutation.
type TagsSideEffect struct {
	svc     *tags.TagsService
	log     *slog.Logger
	metrics *httpmetrics.HTTPMetrics
}

func NewTagsSideEffect(svc *tags.TagsService, log *slog.Logger, metrics *httpmetrics.HTTPMetrics) *TagsSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &TagsSideEffect{svc: svc, log: log, metrics: metrics}
}

func (e *TagsSideEffect) Name() string {
	return "tags"
}

func (e *TagsSideEffect) Apply(event PageSaveEvent) {
	if e.svc == nil {
		return
	}
	switch event.Operation {
	case PageOperationCreate, PageOperationUpdate, PageOperationRestore:
		if event.After != nil {
			e.setTags(event.After, event.Operation)
		}

	case PageOperationMove:
		// page_id is stable across moves; tags in frontmatter are unchanged — no-op.

	case PageOperationDelete:
		for _, p := range event.AffectedPages {
			e.deleteTags(p, event.Operation)
		}
	}
}

func (e *TagsSideEffect) setTags(p *tree.Page, operation PageOperationType) {
	if err := e.svc.IndexPageContent(p.ID, p.RawContent); err != nil {
		e.log.Warn("failed to index page content", "pageID", p.ID, "error", err)
		e.metrics.IncPageSaveSideEffectFailure(string(operation), e.Name())
	}
}

func (e *TagsSideEffect) deleteTags(p *tree.Page, operation PageOperationType) {
	if err := e.svc.DeletePageIndex(p.ID); err != nil {
		e.log.Warn("failed to delete page index", "pageID", p.ID, "error", err)
		e.metrics.IncPageSaveSideEffectFailure(string(operation), e.Name())
	}
}
