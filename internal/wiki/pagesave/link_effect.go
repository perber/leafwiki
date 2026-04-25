package pagesave

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

// LinkIndexSideEffect updates the link index after every page mutation.
type LinkIndexSideEffect struct {
	svc *links.LinkService
	log *slog.Logger
}

// NewLinkIndexSideEffect creates a LinkIndexSideEffect.
func NewLinkIndexSideEffect(svc *links.LinkService, log *slog.Logger) *LinkIndexSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &LinkIndexSideEffect{svc: svc, log: log}
}

func (e *LinkIndexSideEffect) Apply(event PageSaveEvent) {
	if e.svc == nil {
		return
	}
	switch event.Operation {
	case PageOperationCreate:
		e.updateAndHeal(event.After)

	case PageOperationRestore:
		// Content was restored to a previous version; update outgoing links and heal incoming.
		e.updateAndHeal(event.After)

	case PageOperationUpdate:
		if event.SlugChanged {
			e.markBrokenForOldPath(event.OldPath)
			for _, p := range event.AffectedPages {
				e.updateAndHeal(p)
			}
		} else {
			if event.After != nil {
				if err := e.svc.UpdateLinksForPage(event.After, event.After.Content); err != nil {
					e.log.Warn("failed to update links for page", "pageID", event.After.ID, "error", err)
				}
				e.healExact(event.After)
			}
		}

	case PageOperationMove:
		e.markBrokenForOldPath(event.OldPath)
		for _, p := range event.AffectedPages {
			e.updateAndHeal(p)
		}

	case PageOperationDelete:
		for _, p := range event.AffectedPages {
			if err := e.svc.DeleteOutgoingLinksForPage(p.ID); err != nil {
				e.log.Warn("failed to delete outgoing links", "pageID", p.ID, "error", err)
			}
		}
		if event.Before == nil {
			return
		}
		if len(event.AffectedPages) > 1 {
			// Recursive delete: mark entire subtree prefix as broken.
			e.markBrokenForOldPath(event.OldPath)
		} else {
			// Single-page delete.
			if err := e.svc.MarkIncomingLinksBrokenForPage(event.Before.ID); err != nil {
				e.log.Warn("failed to mark incoming links broken", "pageID", event.Before.ID, "error", err)
			}
			if event.OldPath != "" {
				if err := e.svc.MarkLinksBrokenForPath(event.OldPath); err != nil {
					e.log.Warn("failed to mark links broken for path", "path", event.OldPath, "error", err)
				}
			}
		}
	}
}

func (e *LinkIndexSideEffect) healExact(p *tree.Page) {
	if p == nil {
		return
	}
	if err := e.svc.HealLinksForExactPath(p); err != nil {
		e.log.Warn("failed to heal links for page", "pageID", p.ID, "error", err)
	}
}

func (e *LinkIndexSideEffect) updateAndHeal(p *tree.Page) {
	if p == nil {
		return
	}
	if err := e.svc.UpdateLinksForPage(p, p.Content); err != nil {
		e.log.Warn("failed to update links for page", "pageID", p.ID, "error", err)
	}
	e.healExact(p)
}

func (e *LinkIndexSideEffect) markBrokenForOldPath(oldPath string) {
	if oldPath == "" {
		return
	}
	if err := e.svc.MarkLinksBrokenForPrefix(oldPath); err != nil {
		e.log.Warn("failed to mark links broken for prefix", "path", oldPath, "error", err)
	}
}
