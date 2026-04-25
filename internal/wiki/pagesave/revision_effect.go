package pagesave

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/revision"
)

// RevisionSideEffect records revision history entries after page mutations.
type RevisionSideEffect struct {
	svc *revision.Service
	log *slog.Logger
}

// NewRevisionSideEffect creates a RevisionSideEffect.
func NewRevisionSideEffect(svc *revision.Service, log *slog.Logger) *RevisionSideEffect {
	return &RevisionSideEffect{svc: svc, log: log}
}

func (e *RevisionSideEffect) Apply(event PageSaveEvent) {
	if e.svc == nil {
		return
	}
	switch event.Operation {
	case PageOperationCreate:
		if event.After != nil {
			e.recordContent(event.After.ID, event.UserID, event.Summary)
		}

	case PageOperationUpdate:
		if event.SlugChanged {
			for _, p := range event.AffectedPages {
				if event.ContentChanged && p.ID == event.After.ID {
					// Root page with content change: record content revision instead.
					continue
				}
				e.recordStructure(p.ID, event.UserID)
			}
		} else if event.TitleChanged && !event.ContentChanged {
			if event.After != nil {
				e.recordStructure(event.After.ID, event.UserID)
			}
		}
		if event.ContentChanged && event.After != nil {
			e.recordContent(event.After.ID, event.UserID, event.Summary)
		}

	case PageOperationMove:
		for _, p := range event.AffectedPages {
			e.recordStructure(p.ID, event.UserID)
		}

	case PageOperationDelete:
		// No revision entry on delete; data cleanup is handled by the use case.

	case PageOperationRestore:
		// RestoreRevision already writes a RevisionTypeRestore entry internally.
	}
}

func (e *RevisionSideEffect) recordContent(pageID, userID, summary string) {
	if _, _, err := e.svc.RecordContentUpdate(pageID, userID, summary); err != nil {
		e.log.Warn("failed to record content revision", "pageID", pageID, "error", err)
	}
}

func (e *RevisionSideEffect) recordStructure(pageID, userID string) {
	if _, _, err := e.svc.RecordStructureChange(pageID, userID, ""); err != nil {
		e.log.Warn("failed to record structure revision", "pageID", pageID, "error", err)
	}
}
