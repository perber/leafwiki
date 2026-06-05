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
			// When the title also changed, healed wikilink sentinels
			// (wikilink:OldTitle, broken=0) are not reached by the path-prefix
			// query above. Break them by page ID, then re-heal if another page
			// now exclusively holds the old title.
			if event.TitleChanged && event.After != nil {
				if err := e.svc.MarkIncomingLinksBrokenForPage(event.After.ID); err != nil {
					e.log.Warn("failed to mark incoming links broken for renamed page", "pageID", event.After.ID, "error", err)
				}
				if err := e.svc.HealWikiLinksForTitleIfUnambiguous(event.OldTitle); err != nil {
					e.log.Warn("failed to heal wiki links for old title", "title", event.OldTitle, "error", err)
				}
			}
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
			// Recursive delete: mark path-based links broken via prefix …
			e.markBrokenForOldPath(event.OldPath)
			// … and by page ID so that healed wikilink sentinels
			// (to_path="wikilink:X", not the route path) are also marked broken.
			for _, p := range event.AffectedPages {
				if p == nil {
					continue
				}
				if err := e.svc.MarkIncomingLinksBrokenForPage(p.ID); err != nil {
					e.log.Warn("failed to mark incoming links broken", "pageID", p.ID, "error", err)
				}
			}
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
		// After deletion the title may now be unambiguous: heal any [[Title]]
		// sentinels that were waiting for a unique match. Deduplicate by title
		// to avoid redundant DB round-trips for same-titled pages in a subtree.
		seenTitles := make(map[string]struct{}, len(event.AffectedPages))
		for _, p := range event.AffectedPages {
			if p == nil || p.Title == "" {
				continue
			}
			if _, seen := seenTitles[p.Title]; seen {
				continue
			}
			seenTitles[p.Title] = struct{}{}
			if err := e.svc.HealWikiLinksForTitleIfUnambiguous(p.Title); err != nil {
				e.log.Warn("failed to heal wiki links after delete", "title", p.Title, "error", err)
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
	if err := e.svc.HealWikiLinksForPage(p); err != nil {
		e.log.Warn("failed to heal wiki links for page", "pageID", p.ID, "error", err)
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
