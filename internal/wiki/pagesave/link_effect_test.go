package pagesave

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

func pageKindPtr() *tree.NodeKind {
	k := tree.NodeKindPage
	return &k
}

func setupLinkEffect(t *testing.T) (*LinkIndexSideEffect, *links.LinkService, *tree.TreeService) {
	t.Helper()
	dataDir := t.TempDir()
	ts := tree.NewTreeService(dataDir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	store, err := links.NewLinksStore(dataDir)
	if err != nil {
		t.Fatalf("NewLinksStore: %v", err)
	}
	svc := links.NewLinkService(dataDir, ts, store)
	return NewLinkIndexSideEffect(svc, nil, nil), svc, ts
}

// TestLinkIndexSideEffect_Rename_HealsPreexistingBrokenWikilinks verifies that
// renaming page "Alpha" → "Beta" heals broken [[Alpha]] sentinels that already
// existed in the store (e.g. from when the title was ambiguous) by re-routing
// them to the one remaining page still titled "Alpha".
func TestLinkIndexSideEffect_Rename_HealsPreexistingBrokenWikilinks(t *testing.T) {
	effect, svc, ts := setupLinkEffect(t)

	// "Alpha" will be renamed; "Keeper" retains the title so [[Alpha]] becomes unambiguous.
	alphaIDPtr, err := ts.CreateNode("system", nil, "Alpha", "alpha", pageKindPtr())
	if err != nil {
		t.Fatalf("CreateNode alpha: %v", err)
	}
	alphaID := *alphaIDPtr

	keeperIDPtr, err := ts.CreateNode("system", nil, "Alpha", "keeper", pageKindPtr())
	if err != nil {
		t.Fatalf("CreateNode keeper: %v", err)
	}
	keeperID := *keeperIDPtr

	linkerIDPtr, err := ts.CreateNode("system", nil, "Linker", "linker", pageKindPtr())
	if err != nil {
		t.Fatalf("CreateNode linker: %v", err)
	}
	linkerID := *linkerIDPtr

	// Write [[Alpha]] wikilink into "Linker" and index it.
	// With two pages titled "Alpha" the sentinel is ambiguous → stored broken.
	linkerPage, err := ts.GetPage(linkerID)
	if err != nil {
		t.Fatalf("GetPage linker: %v", err)
	}
	content := "See [[Alpha]] for details."
	if err := ts.UpdateNode("system", linkerPage.ID, linkerPage.Title, linkerPage.Slug, &content, tree.VersionUnchecked, nil, nil, false); err != nil {
		t.Fatalf("UpdateNode linker: %v", err)
	}
	linkerPage, err = ts.GetPage(linkerID)
	if err != nil {
		t.Fatalf("GetPage linker (after update): %v", err)
	}
	if err := svc.UpdateLinksForPage(linkerPage, linkerPage.Content); err != nil {
		t.Fatalf("UpdateLinksForPage: %v", err)
	}

	// Precondition: sentinel must be broken (ambiguous).
	out, err := svc.GetOutgoingLinksForPage(linkerID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage: %v", err)
	}
	if out.Count != 1 || !out.Outgoings[0].Broken {
		t.Fatalf("precondition failed: expected 1 broken [[Alpha]] sentinel, got %+v", out.Outgoings)
	}

	// Rename "Alpha" → "Beta" in the tree (UpdateNode mutates the live node).
	if err := ts.UpdateNode("system", alphaID, "Beta", "beta", nil, tree.VersionUnchecked, nil, nil, false); err != nil {
		t.Fatalf("UpdateNode rename alpha→beta: %v", err)
	}
	afterPage, err := ts.GetPage(alphaID)
	if err != nil {
		t.Fatalf("GetPage after rename: %v", err)
	}

	// Apply the link effect with the event that the use case would emit.
	effect.Apply(PageSaveEvent{
		Operation:     PageOperationUpdate,
		UserID:        "system",
		After:         afterPage,
		OldPath:       "/alpha",
		OldTitle:      "Alpha",
		TitleChanged:  true,
		SlugChanged:   true,
		AffectedPages: []*tree.Page{afterPage},
	})

	// The broken [[Alpha]] sentinel must now be healed to point to "Keeper" —
	// the only page still titled "Alpha".
	out2, err := svc.GetOutgoingLinksForPage(linkerID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage (after heal): %v", err)
	}
	if out2.Count != 1 {
		t.Fatalf("expected 1 outgoing after heal, got %d: %+v", out2.Count, out2.Outgoings)
	}
	if out2.Outgoings[0].Broken {
		t.Fatalf("expected [[Alpha]] sentinel to be healed, still broken: %+v", out2.Outgoings[0])
	}
	if out2.Outgoings[0].ToPageID != keeperID {
		t.Fatalf("expected sentinel to point to keeper (%q), got %q", keeperID, out2.Outgoings[0].ToPageID)
	}
}
