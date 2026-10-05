package pagesave

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

func createEffectPage(t *testing.T, ts *tree.TreeService, parentID *string, title, slug string) string {
	t.Helper()
	id, err := ts.CreateNode("system", parentID, title, slug, pageKindPtr())
	if err != nil {
		t.Fatalf("CreateNode %q: %v", title, err)
	}
	return *id
}

func indexLinks(t *testing.T, svc *links.LinkService, ts *tree.TreeService, pageID, content string) {
	t.Helper()
	page, err := ts.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if err := svc.UpdateLinksForPage(page, content); err != nil {
		t.Fatalf("UpdateLinksForPage: %v", err)
	}
}

func brokenCount(t *testing.T, svc *links.LinkService) int {
	t.Helper()
	broken, err := svc.GetBrokenLinks()
	if err != nil {
		t.Fatalf("GetBrokenLinks: %v", err)
	}
	return len(broken)
}

func backlinkCount(t *testing.T, svc *links.LinkService, pageID string) int {
	t.Helper()
	bl, err := svc.GetBacklinksForPage(pageID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage: %v", err)
	}
	return bl.Count
}

// Creating a second page with the same title turns an already resolved
// [[Title]] into an ambiguous one (not broken, backlink on both pages) without
// the source being re-saved.
func TestLinkIndexSideEffect_Create_DuplicateTitle_ResolvedBecomesAmbiguous(t *testing.T) {
	effect, svc, ts := setupLinkEffect(t)

	sourceID := createEffectPage(t, ts, nil, "Source", "source")
	firstID := createEffectPage(t, ts, nil, "Kafka", "kafka")
	indexLinks(t, svc, ts, sourceID, "[[Kafka]]")

	docsID := createEffectPage(t, ts, nil, "Docs", "docs")
	secondID := createEffectPage(t, ts, &docsID, "Kafka", "kafka")
	second, _ := ts.GetPage(secondID)
	effect.Apply(PageSaveEvent{Operation: PageOperationCreate, After: second, AffectedPages: []*tree.Page{second}})

	if n := brokenCount(t, svc); n != 0 {
		t.Fatalf("ambiguous link must not be broken, got %d broken", n)
	}
	for _, id := range []string{firstID, secondID} {
		if n := backlinkCount(t, svc, id); n != 1 {
			t.Fatalf("page %s: expected 1 backlink, got %d", id, n)
		}
	}
}

// Title-only rename (slug unchanged): links under the old title must stop
// pointing at the renamed page, links under the new title must resolve to it.
func TestLinkIndexSideEffect_TitleOnlyRename_ReconcilesOldAndNewTitle(t *testing.T) {
	effect, svc, ts := setupLinkEffect(t)

	sourceID := createEffectPage(t, ts, nil, "Source", "source")
	kafkaID := createEffectPage(t, ts, nil, "Kafka", "kafka")
	indexLinks(t, svc, ts, sourceID, "[[Kafka]] and [[Streams]]")

	if n := brokenCount(t, svc); n != 1 {
		t.Fatalf("precondition: [[Streams]] should be broken, got %d broken", n)
	}

	before, _ := ts.GetPage(kafkaID)
	content := before.Content
	if err := ts.UpdateNode("system", kafkaID, "Streams", before.Slug, &content, tree.VersionUnchecked, nil, nil, false); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	after, _ := ts.GetPage(kafkaID)
	effect.Apply(PageSaveEvent{
		Operation:     PageOperationUpdate,
		Before:        before,
		After:         after,
		TitleChanged:  true,
		OldTitle:      "Kafka",
		OldPath:       before.CalculatePath(),
		AffectedPages: []*tree.Page{after},
	})

	broken, err := svc.GetBrokenLinks()
	if err != nil {
		t.Fatalf("GetBrokenLinks: %v", err)
	}
	if len(broken) != 1 || broken[0].ToPath != "wikilink:Kafka" {
		t.Fatalf("expected only [[Kafka]] broken after the rename, got %#v", broken)
	}
	if n := backlinkCount(t, svc, kafkaID); n != 1 {
		t.Fatalf("renamed page should be linked by [[Streams]] only, got %d backlinks", n)
	}
}

func TestLinkIndexSideEffect_Delete_OneOfTwo_AmbiguousBecomesResolved(t *testing.T) {
	effect, svc, ts := setupLinkEffect(t)

	sourceID := createEffectPage(t, ts, nil, "Source", "source")
	firstID := createEffectPage(t, ts, nil, "Kafka", "kafka")
	docsID := createEffectPage(t, ts, nil, "Docs", "docs")
	secondID := createEffectPage(t, ts, &docsID, "Kafka", "kafka")
	indexLinks(t, svc, ts, sourceID, "[[Kafka]]")

	before, _ := ts.GetPage(secondID)
	oldPath := before.CalculatePath()
	if err := ts.DeleteNode("system", secondID, false, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	effect.Apply(PageSaveEvent{Operation: PageOperationDelete, Before: before, OldPath: oldPath, AffectedPages: []*tree.Page{before}})

	if n := brokenCount(t, svc); n != 0 {
		t.Fatalf("expected no broken links, got %d", n)
	}
	bl, _ := svc.GetBacklinksForPage(firstID)
	if bl.Count != 1 || bl.Backlinks[0].ToPageID != firstID {
		t.Fatalf("expected a resolved backlink on the remaining page, got %#v", bl)
	}
}

func TestLinkIndexSideEffect_Delete_LastMatch_BecomesBroken(t *testing.T) {
	effect, svc, ts := setupLinkEffect(t)

	sourceID := createEffectPage(t, ts, nil, "Source", "source")
	kafkaID := createEffectPage(t, ts, nil, "Kafka", "kafka")
	indexLinks(t, svc, ts, sourceID, "[[Kafka]]")

	before, _ := ts.GetPage(kafkaID)
	oldPath := before.CalculatePath()
	if err := ts.DeleteNode("system", kafkaID, false, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	effect.Apply(PageSaveEvent{Operation: PageOperationDelete, Before: before, OldPath: oldPath, AffectedPages: []*tree.Page{before}})

	if n := brokenCount(t, svc); n != 1 {
		t.Fatalf("expected 1 broken link, got %d", n)
	}
}

// linkSnapshot captures (source, target path, target page, state) for every
// page so the incrementally maintained index can be compared to a rebuild.
func linkSnapshot(t *testing.T, svc *links.LinkService, ts *tree.TreeService) map[string]bool {
	t.Helper()
	snap := map[string]bool{}
	var ids []string
	if err := ts.WalkNodes(func(id string) error { ids = append(ids, id); return nil }); err != nil {
		t.Fatalf("WalkNodes: %v", err)
	}
	for _, id := range ids {
		status, err := svc.GetLinkStatusForPage(id, mustPath(t, ts, id))
		if err != nil {
			t.Fatalf("GetLinkStatusForPage: %v", err)
		}
		for _, o := range status.Outgoings {
			snap["ok|"+id+"|"+o.ToPath+"|"+o.ToPageID] = true
		}
		for _, o := range status.BrokenOutgoings {
			snap["broken|"+id+"|"+o.ToPath] = true
		}
		for _, b := range status.Backlinks {
			snap["back|"+id+"|"+b.FromPageID+"|"+b.ToPageID] = true
		}
	}
	return snap
}

func mustPath(t *testing.T, ts *tree.TreeService, id string) string {
	t.Helper()
	p, err := ts.GetPage(id)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	return p.CalculatePath()
}

// Invariant: after any sequence of page operations the incrementally
// maintained link states equal what a full rebuild (resync) produces.
func TestLinkIndexSideEffect_IncrementalState_EqualsFullRebuild(t *testing.T) {
	effect, svc, ts := setupLinkEffect(t)

	sourceID := createEffectPage(t, ts, nil, "Source", "source")
	kafkaID := createEffectPage(t, ts, nil, "Kafka", "kafka")
	indexLinks(t, svc, ts, sourceID, "[[Kafka]] [[Streams]] [[Ghost]]")

	check := func(step string) {
		t.Helper()
		incremental := linkSnapshot(t, svc, ts)
		if err := svc.IndexAllPages(); err != nil {
			t.Fatalf("%s: IndexAllPages: %v", step, err)
		}
		rebuilt := linkSnapshot(t, svc, ts)
		for k := range incremental {
			if !rebuilt[k] {
				t.Errorf("%s: only in incremental index: %s", step, k)
			}
		}
		for k := range rebuilt {
			if !incremental[k] {
				t.Errorf("%s: only in rebuilt index: %s", step, k)
			}
		}
	}
	// The source page content lives only in the index until a rebuild reads it
	// from the tree, so persist it first.
	src, _ := ts.GetPage(sourceID)
	content := "[[Kafka]] [[Streams]] [[Ghost]]"
	if err := ts.UpdateNode("system", sourceID, src.Title, src.Slug, &content, tree.VersionUnchecked, nil, nil, false); err != nil {
		t.Fatalf("UpdateNode source: %v", err)
	}
	check("initial")

	// duplicate Kafka -> ambiguous
	docsID := createEffectPage(t, ts, nil, "Docs", "docs")
	dupID := createEffectPage(t, ts, &docsID, "Kafka", "kafka")
	dup, _ := ts.GetPage(dupID)
	effect.Apply(PageSaveEvent{Operation: PageOperationCreate, After: dup, AffectedPages: []*tree.Page{dup}})
	check("duplicate created")

	// title-only rename of the first Kafka to Streams
	before, _ := ts.GetPage(kafkaID)
	kc := before.Content
	if err := ts.UpdateNode("system", kafkaID, "Streams", before.Slug, &kc, tree.VersionUnchecked, nil, nil, false); err != nil {
		t.Fatalf("UpdateNode rename: %v", err)
	}
	after, _ := ts.GetPage(kafkaID)
	effect.Apply(PageSaveEvent{Operation: PageOperationUpdate, Before: before, After: after, TitleChanged: true, OldTitle: "Kafka", OldPath: before.CalculatePath(), AffectedPages: []*tree.Page{after}})
	check("title-only rename")

	// delete the remaining Kafka -> [[Kafka]] broken
	dupBefore, _ := ts.GetPage(dupID)
	oldPath := dupBefore.CalculatePath()
	if err := ts.DeleteNode("system", dupID, false, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	effect.Apply(PageSaveEvent{Operation: PageOperationDelete, Before: dupBefore, OldPath: oldPath, AffectedPages: []*tree.Page{dupBefore}})
	check("delete")
}
