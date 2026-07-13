package pagesave

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/revision"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
)

// RevisionSideEffect records revision history entries after page mutations.
type RevisionSideEffect struct {
	svc     *revision.Service
	log     *slog.Logger
	metrics *httpmetrics.HTTPMetrics
}

// NewRevisionSideEffect creates a RevisionSideEffect.
func NewRevisionSideEffect(svc *revision.Service, log *slog.Logger, metrics *httpmetrics.HTTPMetrics) *RevisionSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &RevisionSideEffect{svc: svc, log: log, metrics: metrics}
}

func (e *RevisionSideEffect) Name() string {
	return "revision"
}

func (e *RevisionSideEffect) Apply(event PageSaveEvent) {
	if e.svc == nil {
		return
	}
	switch event.Operation {
	case PageOperationCreate:
		if event.After != nil {
			e.recordContent(event.After.ID, event.UserID, event.Summary, event.Operation)
		}

	case PageOperationUpdate:
		if event.SlugChanged {
			for _, p := range event.AffectedPages {
				if event.ContentChanged && p.ID == event.After.ID {
					// Root page with content change: record content revision instead.
					continue
				}
				e.recordStructure(p.ID, event.UserID, event.Operation)
			}
		} else if event.TitleChanged && !event.ContentChanged {
			if event.After != nil {
				e.recordStructure(event.After.ID, event.UserID, event.Operation)
			}
		}
		if event.ContentChanged && event.After != nil {
			e.recordContent(event.After.ID, event.UserID, event.Summary, event.Operation)
		}

	case PageOperationMove:
		for _, p := range event.AffectedPages {
			e.recordStructure(p.ID, event.UserID, event.Operation)
		}

	case PageOperationDelete:
		// No revision entry on delete; data cleanup is handled by the use case.

	case PageOperationRestore:
		// RestoreRevision already writes a RevisionTypeRestore entry internally.
	}
}

func (e *RevisionSideEffect) recordContent(pageID, userID, summary string, operation PageOperationType) {
	if _, _, err := e.svc.RecordContentUpdate(pageID, userID, summary); err != nil {
		e.log.Warn("failed to record content revision", "pageID", pageID, "error", err)
		e.metrics.IncPageSaveSideEffectFailure(string(operation), e.Name())
	}
}

func (e *RevisionSideEffect) recordStructure(pageID, userID string, operation PageOperationType) {
	if _, _, err := e.svc.RecordStructureChange(pageID, userID, ""); err != nil {
		e.log.Warn("failed to record structure revision", "pageID", pageID, "error", err)
		e.metrics.IncPageSaveSideEffectFailure(string(operation), e.Name())
	}
}
