package pagesave

import (
	"log/slog"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/links"
)

// LinkIndexSideEffect updates the link index after every page mutation.
type LinkIndexSideEffect struct {
	svc     *links.LinkService
	log     *slog.Logger
	metrics *httpmetrics.HTTPMetrics
}

// NewLinkIndexSideEffect creates a LinkIndexSideEffect.
func NewLinkIndexSideEffect(svc *links.LinkService, log *slog.Logger, metrics *httpmetrics.HTTPMetrics) *LinkIndexSideEffect {
	if log == nil {
		log = slog.Default()
	}
	return &LinkIndexSideEffect{svc: svc, log: log, metrics: metrics}
}

func (e *LinkIndexSideEffect) Name() string {
	return "links"
}

func (e *LinkIndexSideEffect) Apply(event PageSaveEvent) {
	if e.svc == nil {
		return
	}
	switch event.Operation {
	case PageOperationCreate:
		e.updateAndHeal(event.After, event.Operation)
		e.reconcilePageTitle(event.After, event.Operation)

	case PageOperationRestore:
		// Content was restored to a previous version; update outgoing links and heal incoming.
		e.updateAndHeal(event.After, event.Operation)
		e.reconcilePageTitle(event.After, event.Operation)

	case PageOperationUpdate:
		if event.SlugChanged {
			e.markBrokenForOldPath(event.OldPath, event.Operation)
			for _, p := range event.AffectedPages {
				e.updateAndHeal(p, event.Operation)
			}
		} else if event.After != nil {
			if err := e.svc.UpdateLinksForPage(event.After, event.After.Content); err != nil {
				e.log.Warn("failed to update links for page", "pageID", event.After.ID, "error", err)
				e.recordFailure(event.Operation)
			}
			e.healExact(event.After, event.Operation)
		}
		// A title change alters which pages carry the old and the new title,
		// whether or not the slug changed. [[OldTitle]] and [[NewTitle]] links
		// are recomputed in one statement per title.
		if event.TitleChanged && event.After != nil {
			e.reconcileTitles(event.Operation, event.OldTitle, event.After.Title)
		}

	case PageOperationMove:
		e.markBrokenForOldPath(event.OldPath, event.Operation)
		for _, p := range event.AffectedPages {
			e.updateAndHeal(p, event.Operation)
		}

	case PageOperationDelete:
		for _, p := range event.AffectedPages {
			if err := e.svc.DeleteOutgoingLinksForPage(p.ID); err != nil {
				e.log.Warn("failed to delete outgoing links", "pageID", p.ID, "error", err)
				e.recordFailure(event.Operation)
			}
		}
		if event.Before == nil {
			e.reconcileDeletedTitles(event)
			return
		}
		if len(event.AffectedPages) > 1 {
			// Recursive delete: mark path-based links broken via prefix …
			e.markBrokenForOldPath(event.OldPath, event.Operation)
			// … and by page ID so that healed wikilink sentinels
			// (to_path="wikilink:X", not the route path) are also marked broken.
			for _, p := range event.AffectedPages {
				if p == nil {
					continue
				}
				if err := e.svc.MarkIncomingLinksBrokenForPage(p.ID); err != nil {
					e.log.Warn("failed to mark incoming links broken", "pageID", p.ID, "error", err)
					e.recordFailure(event.Operation)
				}
			}
		} else {
			// Single-page delete.
			if err := e.svc.MarkIncomingLinksBrokenForPage(event.Before.ID); err != nil {
				e.log.Warn("failed to mark incoming links broken", "pageID", event.Before.ID, "error", err)
				e.recordFailure(event.Operation)
			}
			if event.OldPath != "" {
				if err := e.svc.MarkLinksBrokenForPath(event.OldPath); err != nil {
					e.log.Warn("failed to mark links broken for path", "path", event.OldPath, "error", err)
					e.recordFailure(event.Operation)
				}
			}
		}
		e.reconcileDeletedTitles(event)
	}
}

func (e *LinkIndexSideEffect) healExact(p *tree.Page, operation PageOperationType) {
	if p == nil {
		return
	}
	if err := e.svc.HealLinksForExactPath(p); err != nil {
		e.log.Warn("failed to heal links for page", "pageID", p.ID, "error", err)
		e.recordFailure(operation)
	}
}

// reconcileDeletedTitles: the deleted pages no longer carry their titles, so
// [[Title]] links may now be resolved (one page left), still ambiguous or broken.
func (e *LinkIndexSideEffect) reconcileDeletedTitles(event PageSaveEvent) {
	titles := make([]string, 0, len(event.AffectedPages))
	for _, p := range event.AffectedPages {
		if p != nil {
			titles = append(titles, p.Title)
		}
	}
	e.reconcileTitles(event.Operation, titles...)
}

func (e *LinkIndexSideEffect) reconcilePageTitle(p *tree.Page, operation PageOperationType) {
	if p == nil {
		return
	}
	e.reconcileTitles(operation, p.Title)
}

// reconcileTitles recomputes the state of all [[Title]] links for each given
// title, once per distinct (case-insensitive) title.
func (e *LinkIndexSideEffect) reconcileTitles(operation PageOperationType, titles ...string) {
	seen := make(map[string]struct{}, len(titles))
	for _, title := range titles {
		key := strings.ToLower(strings.TrimSpace(title))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := e.svc.ReconcileTitle(title); err != nil {
			e.log.Warn("failed to reconcile wiki links for title", "title", title, "error", err)
			e.recordFailure(operation)
		}
	}
}

func (e *LinkIndexSideEffect) updateAndHeal(p *tree.Page, operation PageOperationType) {
	if p == nil {
		return
	}
	if err := e.svc.UpdateLinksForPage(p, p.Content); err != nil {
		e.log.Warn("failed to update links for page", "pageID", p.ID, "error", err)
		e.recordFailure(operation)
	}
	e.healExact(p, operation)
}

func (e *LinkIndexSideEffect) markBrokenForOldPath(oldPath string, operation PageOperationType) {
	if oldPath == "" {
		return
	}
	if err := e.svc.MarkLinksBrokenForPrefix(oldPath); err != nil {
		e.log.Warn("failed to mark links broken for prefix", "path", oldPath, "error", err)
		e.recordFailure(operation)
	}
}

func (e *LinkIndexSideEffect) recordFailure(operation PageOperationType) {
	e.metrics.IncPageSaveSideEffectFailure(string(operation), e.Name())
}
