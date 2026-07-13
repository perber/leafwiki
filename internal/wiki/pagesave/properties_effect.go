package pagesave

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/properties"
)

// PropertiesSideEffect updates the properties index after every page mutation.
type PropertiesSideEffect struct {
	svc     *properties.PropertiesService
	log     *slog.Logger
	metrics *httpmetrics.HTTPMetrics
}

func NewPropertiesSideEffect(svc *properties.PropertiesService, log *slog.Logger, metrics *httpmetrics.HTTPMetrics) *PropertiesSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &PropertiesSideEffect{svc: svc, log: log, metrics: metrics}
}

func (e *PropertiesSideEffect) Name() string {
	return "properties"
}

func (e *PropertiesSideEffect) Apply(event PageSaveEvent) {
	if e.svc == nil {
		return
	}
	switch event.Operation {
	case PageOperationCreate, PageOperationUpdate, PageOperationRestore:
		if event.After != nil {
			e.setProperties(event.After, event.Operation)
		}

	case PageOperationMove:
		// page_id is stable across moves; properties in frontmatter are unchanged — no-op.

	case PageOperationDelete:
		for _, p := range event.AffectedPages {
			e.deleteProperties(p, event.Operation)
		}
	}
}

func (e *PropertiesSideEffect) setProperties(p *tree.Page, operation PageOperationType) {
	props := properties.ExtractPropertiesFromContent(p.RawContent)
	if err := e.svc.SetPropertiesForPage(p.ID, props); err != nil {
		e.log.Warn("failed to set properties for page", "pageID", p.ID, "error", err)
		e.metrics.IncPageSaveSideEffectFailure(string(operation), e.Name())
	}
}

func (e *PropertiesSideEffect) deleteProperties(p *tree.Page, operation PageOperationType) {
	if err := e.svc.DeletePropertiesForPage(p.ID); err != nil {
		e.log.Warn("failed to delete properties for page", "pageID", p.ID, "error", err)
		e.metrics.IncPageSaveSideEffectFailure(string(operation), e.Name())
	}
}
