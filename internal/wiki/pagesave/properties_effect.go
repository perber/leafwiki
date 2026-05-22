package pagesave

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/properties"
)

// PropertiesSideEffect updates the properties index after every page mutation.
type PropertiesSideEffect struct {
	svc *properties.PropertiesService
	log *slog.Logger
}

func NewPropertiesSideEffect(svc *properties.PropertiesService, log *slog.Logger) *PropertiesSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &PropertiesSideEffect{svc: svc, log: log}
}

func (e *PropertiesSideEffect) Apply(event PageSaveEvent) {
	if e.svc == nil {
		return
	}
	switch event.Operation {
	case PageOperationCreate, PageOperationUpdate, PageOperationRestore:
		if event.After != nil {
			e.setProperties(event.After)
		}

	case PageOperationMove:
		// page_id is stable across moves; properties in frontmatter are unchanged — no-op.

	case PageOperationDelete:
		for _, p := range event.AffectedPages {
			e.deleteProperties(p)
		}
	}
}

func (e *PropertiesSideEffect) setProperties(p *tree.Page) {
	props := properties.ExtractPropertiesFromContent(p.RawContent)
	if err := e.svc.SetPropertiesForPage(p.ID, props); err != nil {
		e.log.Warn("failed to set properties for page", "pageID", p.ID, "error", err)
	}
}

func (e *PropertiesSideEffect) deleteProperties(p *tree.Page) {
	if err := e.svc.DeletePropertiesForPage(p.ID); err != nil {
		e.log.Warn("failed to delete properties for page", "pageID", p.ID, "error", err)
	}
}
