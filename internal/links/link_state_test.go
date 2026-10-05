package links

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

// Regression for perber/leafwiki#1615: an ambiguous [[Title]] (several pages
// share the title) resolves to all of them and is shown as a normal backlink,
// so it must not show up in the wiki-wide broken-links list.
func TestLinkService_GetBrokenLinks_AmbiguousWikilink_NotListed(t *testing.T) {
	svc, ts, _ := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	mustCreatePage(t, ts, nil, "Kafka", "kafka")
	sectionID := mustCreatePage(t, ts, nil, "Docs", "docs")
	mustCreatePage(t, ts, &sectionID, "Kafka", "kafka")

	updateLinks(t, svc, ts, sourceID, "See [[Kafka]].")

	broken, err := svc.GetBrokenLinks()
	if err != nil {
		t.Fatalf("GetBrokenLinks failed: %v", err)
	}
	if len(broken) != 0 {
		t.Fatalf("ambiguous wikilink must not be listed as broken, got %#v", broken)
	}
}

func TestLinkService_GetBrokenLinks_UnresolvedWikilink_Listed(t *testing.T) {
	svc, ts, _ := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	updateLinks(t, svc, ts, sourceID, "See [[Nowhere]].")

	broken, err := svc.GetBrokenLinks()
	if err != nil {
		t.Fatalf("GetBrokenLinks failed: %v", err)
	}
	if len(broken) != 1 || broken[0].FromPageID != sourceID {
		t.Fatalf("expected the unresolved wikilink to be listed, got %#v", broken)
	}
}

func mustCreatePage(t *testing.T, ts *tree.TreeService, parentID *string, title, slug string) string {
	t.Helper()
	id, err := ts.CreateNode("system", parentID, title, slug, pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode %q failed: %v", title, err)
	}
	return *id
}

func updateLinks(t *testing.T, svc *LinkService, ts *tree.TreeService, pageID, content string) {
	t.Helper()
	page, err := ts.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage %s failed: %v", pageID, err)
	}
	if err := svc.UpdateLinksForPage(page, content); err != nil {
		t.Fatalf("UpdateLinksForPage failed: %v", err)
	}
}

func outgoingState(t *testing.T, store *LinksStore, fromID string) LinkState {
	t.Helper()
	outs, err := store.GetOutgoingLinksForPage(fromID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage failed: %v", err)
	}
	if len(outs) != 1 {
		t.Fatalf("expected exactly 1 outgoing link, got %#v", outs)
	}
	return outs[0].State
}

func TestClassifyTitleMatches(t *testing.T) {
	cases := []struct {
		matches int
		want    LinkState
	}{
		{0, LinkBroken},
		{1, LinkResolved},
		{2, LinkAmbiguous},
		{5, LinkAmbiguous},
	}
	for _, tc := range cases {
		if got := classifyTitleMatches(tc.matches); got != tc.want {
			t.Errorf("classifyTitleMatches(%d) = %v, want %v", tc.matches, got, tc.want)
		}
	}
}

func TestLinkService_UpdateLinks_StoresStatePerMatchCount(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")

	updateLinks(t, svc, ts, sourceID, "[[Kafka]]")
	if got := outgoingState(t, store, sourceID); got != LinkBroken {
		t.Fatalf("0 matches: state = %v, want broken", got)
	}

	mustCreatePage(t, ts, nil, "Kafka", "kafka")
	updateLinks(t, svc, ts, sourceID, "[[Kafka]]")
	if got := outgoingState(t, store, sourceID); got != LinkResolved {
		t.Fatalf("1 match: state = %v, want resolved", got)
	}

	docsID := mustCreatePage(t, ts, nil, "Docs", "docs")
	mustCreatePage(t, ts, &docsID, "Kafka", "kafka")
	updateLinks(t, svc, ts, sourceID, "[[Kafka]]")
	if got := outgoingState(t, store, sourceID); got != LinkAmbiguous {
		t.Fatalf("2 matches: state = %v, want ambiguous", got)
	}
}

// A resolved link must flip to ambiguous when a second page with the same
// title appears, without the source page being re-saved.
func TestLinkService_ReconcileTitle_DuplicateCreated_ResolvedBecomesAmbiguous(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	firstID := mustCreatePage(t, ts, nil, "Kafka", "kafka")
	updateLinks(t, svc, ts, sourceID, "[[Kafka]]")

	docsID := mustCreatePage(t, ts, nil, "Docs", "docs")
	secondID := mustCreatePage(t, ts, &docsID, "Kafka", "kafka")
	if err := svc.ReconcileTitle("Kafka"); err != nil {
		t.Fatalf("ReconcileTitle failed: %v", err)
	}

	if got := outgoingState(t, store, sourceID); got != LinkAmbiguous {
		t.Fatalf("state = %v, want ambiguous", got)
	}
	broken, _ := svc.GetBrokenLinks()
	if len(broken) != 0 {
		t.Fatalf("ambiguous link must not be listed as broken: %#v", broken)
	}
	for _, id := range []string{firstID, secondID} {
		bl, err := svc.GetBacklinksForPage(id)
		if err != nil {
			t.Fatalf("GetBacklinksForPage failed: %v", err)
		}
		if bl.Count != 1 || bl.Backlinks[0].FromPageID != sourceID || bl.Backlinks[0].Broken {
			t.Fatalf("page %s: expected the source as a non-broken backlink, got %#v", id, bl)
		}
	}
}

func TestLinkService_ReconcileTitle_OneOfTwoDeleted_AmbiguousBecomesResolved(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	firstID := mustCreatePage(t, ts, nil, "Kafka", "kafka")
	docsID := mustCreatePage(t, ts, nil, "Docs", "docs")
	secondID := mustCreatePage(t, ts, &docsID, "Kafka", "kafka")
	updateLinks(t, svc, ts, sourceID, "[[Kafka]]")

	if err := ts.DeleteNode("system", secondID, false, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := svc.ReconcileTitle("Kafka"); err != nil {
		t.Fatalf("ReconcileTitle failed: %v", err)
	}

	if got := outgoingState(t, store, sourceID); got != LinkResolved {
		t.Fatalf("state = %v, want resolved", got)
	}
	bl, err := svc.GetBacklinksForPage(firstID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}
	if bl.Count != 1 || bl.Backlinks[0].ToPageID != firstID {
		t.Fatalf("expected resolved backlink to the remaining page, got %#v", bl)
	}
}

func TestLinkService_ReconcileTitle_AllDeleted_BecomesBroken(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	kafkaID := mustCreatePage(t, ts, nil, "Kafka", "kafka")
	updateLinks(t, svc, ts, sourceID, "[[Kafka]]")

	if err := ts.DeleteNode("system", kafkaID, false, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := svc.ReconcileTitle("Kafka"); err != nil {
		t.Fatalf("ReconcileTitle failed: %v", err)
	}

	if got := outgoingState(t, store, sourceID); got != LinkBroken {
		t.Fatalf("state = %v, want broken", got)
	}
	broken, _ := svc.GetBrokenLinks()
	if len(broken) != 1 {
		t.Fatalf("expected 1 broken link, got %#v", broken)
	}
}

func TestLinkService_ReconcileTitle_IsCaseInsensitive(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	updateLinks(t, svc, ts, sourceID, "[[kafka]]")
	mustCreatePage(t, ts, nil, "Kafka", "kafka")

	if err := svc.ReconcileTitle("Kafka"); err != nil {
		t.Fatalf("ReconcileTitle failed: %v", err)
	}
	if got := outgoingState(t, store, sourceID); got != LinkResolved {
		t.Fatalf("state = %v, want resolved", got)
	}
}

// Path-based links are never ambiguous; reconciling a title must not touch them.
func TestLinkService_ReconcileTitle_LeavesPathLinksAlone(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	mustCreatePage(t, ts, nil, "Kafka", "kafka")
	updateLinks(t, svc, ts, sourceID, "[k](/kafka)")

	if err := svc.ReconcileTitle("Kafka"); err != nil {
		t.Fatalf("ReconcileTitle failed: %v", err)
	}
	if got := outgoingState(t, store, sourceID); got != LinkResolved {
		t.Fatalf("path link state = %v, want resolved", got)
	}
}

// SQLite's LOWER() only folds ASCII, so the reconcile key must be folded the
// same way or titles with non-ASCII capitals (Ärzte, Über, Éclair) never match.
func TestLinkService_ReconcileTitle_NonASCIITitle(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	mustCreatePage(t, ts, nil, "Ärzte", "aerzte")
	updateLinks(t, svc, ts, sourceID, "[[Ärzte]]")

	docsID := mustCreatePage(t, ts, nil, "Docs", "docs")
	mustCreatePage(t, ts, &docsID, "Ärzte", "aerzte")
	if err := svc.ReconcileTitle("Ärzte"); err != nil {
		t.Fatalf("ReconcileTitle failed: %v", err)
	}
	if got := outgoingState(t, store, sourceID); got != LinkAmbiguous {
		t.Fatalf("state = %v, want ambiguous", got)
	}
}

func TestLinkService_ReconcileTitle_TrimsWhitespace(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	sourceID := mustCreatePage(t, ts, nil, "Source", "source")
	updateLinks(t, svc, ts, sourceID, "[[Kafka]]")
	mustCreatePage(t, ts, nil, "Kafka", "kafka")

	if err := svc.ReconcileTitle("  Kafka "); err != nil {
		t.Fatalf("ReconcileTitle failed: %v", err)
	}
	if got := outgoingState(t, store, sourceID); got != LinkResolved {
		t.Fatalf("state = %v, want resolved", got)
	}
}

// Regression: a path rewrite (rename/move) re-writes the whole row set of a
// source page. Its wiki-link rows must survive it.
func TestLinkService_UpdateRewrittenLinks_KeepsWikilinkRows(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	srcID := mustCreatePage(t, ts, nil, "Source", "source")
	fooID := mustCreatePage(t, ts, nil, "Foo", "foo")
	mustCreatePage(t, ts, nil, "Bar", "bar")
	updateLinks(t, svc, ts, srcID, "[[Foo]] [[Ghost]] [b](/bar)")

	page, _ := ts.GetPage(srcID)
	if err := svc.UpdateRewrittenLinksAndHealForPages([]*tree.Page{page}, []RewriteRule{{OldPath: "/old", NewPath: "/new"}}); err != nil {
		t.Fatalf("UpdateRewrittenLinksAndHealForPages failed: %v", err)
	}

	outs, err := store.GetOutgoingLinksForPage(srcID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage failed: %v", err)
	}
	got := map[string]Outgoing{}
	for _, o := range outs {
		got[o.ToPath] = o
	}
	if len(outs) != 3 {
		t.Fatalf("expected 3 rows (Foo, Ghost, /bar), got %#v", outs)
	}
	if o := got["wikilink:Foo"]; o.State != LinkResolved || o.ToPageID != fooID {
		t.Errorf("[[Foo]] row = %#v, want resolved to %s", o, fooID)
	}
	if o := got["wikilink:Ghost"]; o.State != LinkBroken {
		t.Errorf("[[Ghost]] row = %#v, want broken", o)
	}
}

// When the rename rule rewrites [[Old]] to [[New]] in the content, the index row
// has to follow: it becomes wikilink:New, resolved to the renamed page.
func TestLinkService_UpdateRewrittenLinks_TitleRuleMovesWikilinkRow(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	srcID := mustCreatePage(t, ts, nil, "Source", "source")
	kafkaID := mustCreatePage(t, ts, nil, "Kafka", "kafka")
	updateLinks(t, svc, ts, srcID, "[[Kafka]]")

	kafka, _ := ts.GetPage(kafkaID)
	content := kafka.Content
	if err := ts.UpdateNode("system", kafkaID, "Streams", kafka.Slug, &content, tree.VersionUnchecked, nil, nil, false); err != nil {
		t.Fatalf("UpdateNode rename: %v", err)
	}

	page, _ := ts.GetPage(srcID)
	rules := []RewriteRule{{OldTitle: "Kafka", NewTitle: "Streams"}}
	if err := svc.UpdateRewrittenLinksAndHealForPages([]*tree.Page{page}, rules); err != nil {
		t.Fatalf("UpdateRewrittenLinksAndHealForPages failed: %v", err)
	}

	outs, _ := store.GetOutgoingLinksForPage(srcID)
	if len(outs) != 1 || outs[0].ToPath != "wikilink:Streams" || outs[0].State != LinkResolved || outs[0].ToPageID != kafkaID {
		t.Fatalf("expected a single resolved wikilink:Streams row, got %#v", outs)
	}
}
