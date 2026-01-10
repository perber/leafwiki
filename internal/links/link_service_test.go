package links

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

func pageNodeKind() *tree.NodeKind {
	kind := tree.NodeKindPage
	return &kind
}

func TestExtractLinksFromMarkdown_FiltersExternalAndNormalizes(t *testing.T) {
	md := `
# Example

Internal: [Page 1](/docs/page1)
Relative: [Rel](../docs/page2)
Anchor only: [Section](#heading)
External: [Google](https://google.com)
Mail: [Mail](mailto:test@example.com)
With fragment: [WithFragment](/docs/page3#intro)
With query: [WithQuery](/docs/page4?foo=bar)
With both: [Both](/docs/page5?foo=bar#section)
`

	links := extractLinksFromMarkdown(md)

	want := []string{
		"/docs/page1",
		"../docs/page2",
		"/docs/page3",
		"/docs/page4",
		"/docs/page5",
	}

	if len(links) != len(want) {
		t.Fatalf("expected %d links, got %d: %#v", len(want), len(links), links)
	}

	for i, w := range want {
		if links[i] != w {
			t.Errorf("link[%d] = %q, want %q", i, links[i], w)
		}
	}
}

// helper to create a small tree structure:
// root
//
//	└─ docs
//	     ├─ page1
//	     └─ page2
func setupTreeForLinksTest(t *testing.T) (*tree.TreeService, string, string) {
	t.Helper()

	storageDir := t.TempDir()
	ts := tree.NewTreeService(storageDir)

	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	// create "docs" under root
	docsIDPtr, err := ts.CreateNode("system", nil, "Docs", "docs", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}
	docsID := *docsIDPtr

	// create "page1" and "page2" under docs
	page1IDPtr, err := ts.CreateNode("system", &docsID, "Page 1", "page1", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage page1 failed: %v", err)
	}
	page2IDPtr, err := ts.CreateNode("system", &docsID, "Page 2", "page2", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage page2 failed: %v", err)
	}

	return ts, *page1IDPtr, *page2IDPtr
}

func TestResolveTargetLinks_FindsExistingTargets(t *testing.T) {
	ts, page1ID, page2ID := setupTreeForLinksTest(t)

	// current page: docs/page1
	page1, err := ts.GetPage(page1ID)
	if err != nil {
		t.Fatalf("GetPage(page1) failed: %v", err)
	}
	currentPath := page1.CalculatePath() // should be "docs/page1"

	// we want to link from page1 to page2 using a relative link
	links := []string{"../page2"}

	targets := resolveTargetLinks(ts, currentPath, links)

	if len(targets) != 1 {
		t.Fatalf("expected 1 target link, got %d: %#v", len(targets), targets)
	}

	got := targets[0]
	if got.TargetPageID != page2ID {
		t.Errorf("TargetPageID = %q, want %q", got.TargetPageID, page2ID)
	}
	if got.TargetPagePath == "" {
		t.Errorf("TargetPagePath should not be empty")
	}
}

func TestResolveTargetLinks_ReturnsBrokenTargetsForNonExisting(t *testing.T) {
	ts, page1ID, _ := setupTreeForLinksTest(t)

	page1, err := ts.GetPage(page1ID)
	if err != nil {
		t.Fatalf("GetPage(page1) failed: %v", err)
	}
	currentPath := page1.CalculatePath()

	links := []string{
		"./does-not-exist",
		"/docs/unknown",
	}

	targets := resolveTargetLinks(ts, currentPath, links)

	if len(targets) != 2 {
		t.Fatalf("expected 2 target links, got %d: %#v", len(targets), targets)
	}

	if targets[0].Broken != true {
		t.Errorf("targets[0].Broken = %v, want true", targets[0].Broken)
	}
	if targets[0].TargetPageID != "" {
		t.Errorf("targets[0].TargetPageID = %q, want empty", targets[0].TargetPageID)
	}
	if targets[0].TargetPagePath != "/docs/page1/does-not-exist" {
		t.Errorf("targets[0].TargetPagePath = %q, want %q", targets[0].TargetPagePath, "/docs/page1/does-not-exist")
	}

	if targets[1].Broken != true {
		t.Errorf("targets[1].Broken = %v, want true", targets[1].Broken)
	}
	if targets[1].TargetPageID != "" {
		t.Errorf("targets[1].TargetPageID = %q, want empty", targets[1].TargetPageID)
	}
	if targets[1].TargetPagePath != "/docs/unknown" {
		t.Errorf("targets[1].TargetPagePath = %q, want %q", targets[1].TargetPagePath, "/docs/unknown")
	}
}

func setupLinkService(t *testing.T) (*LinkService, *tree.TreeService, *LinksStore) {
	t.Helper()

	dataDir := t.TempDir()

	ts := tree.NewTreeService(dataDir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	store, err := NewLinksStore(dataDir)
	if err != nil {
		t.Fatalf("NewLinksStore failed: %v", err)
	}

	svc := NewLinkService(dataDir, ts, store)
	return svc, ts, store
}

func createSimpleLinkedPages(t *testing.T, ts *tree.TreeService) (pageAID, pageBID string) {
	t.Helper()

	aIDPtr, err := ts.CreateNode("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage a failed: %v", err)
	}
	pageAID = *aIDPtr

	bIDPtr, err := ts.CreateNode("system", nil, "Page B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage b failed: %v", err)
	}
	pageBID = *bIDPtr

	aPage, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage a failed: %v", err)
	}
	contentA := "Link to B: [Go to B](/b)"
	if err := ts.UpdateNode("system", aPage.ID, aPage.Title, aPage.Slug, &contentA); err != nil {
		t.Fatalf("UpdatePage a failed: %v", err)
	}

	bPage, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage b failed: %v", err)
	}
	contentB := "# Page B\nNo outgoing links."
	if err := ts.UpdateNode("system", bPage.ID, bPage.Title, bPage.Slug, &contentB); err != nil {
		t.Fatalf("UpdatePage b failed: %v", err)
	}

	return pageAID, pageBID
}

func TestLinkService_IndexAllPages_BuildsLinks(t *testing.T) {
	svc, ts, _ := setupLinkService(t)
	pageAID, pageBID := createSimpleLinkedPages(t, ts)

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	data, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}

	if len(data.Backlinks) != 1 {
		t.Fatalf("expected 1 backlink for pageB, got %d: %#v", len(data.Backlinks), data.Backlinks)
	}

	bl := data.Backlinks[0]
	if bl.FromPageID != pageAID {
		t.Errorf("FromPageID = %q, want %q", bl.FromPageID, pageAID)
	}
	if bl.ToPageID != pageBID {
		t.Errorf("ToPageID = %q, want %q", bl.ToPageID, pageBID)
	}
	if bl.FromTitle == "" {
		t.Errorf("FromTitle should not be empty")
	}
}

func TestLinkService_IndexAllPages_ReplacesExistingLinks(t *testing.T) {
	svc, ts, _ := setupLinkService(t)
	pageAID, pageBID := createSimpleLinkedPages(t, ts)

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages (first) failed: %v", err)
	}

	aPage, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage a failed: %v", err)
	}
	var noLinks string = "No more links."
	if err := ts.UpdateNode("system", aPage.ID, aPage.Title, aPage.Slug, &noLinks); err != nil {
		t.Fatalf("UpdatePage a failed: %v", err)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages (second) failed: %v", err)
	}

	data, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}

	if len(data.Backlinks) != 0 {
		t.Fatalf("expected 0 backlinks after reindex, got %d: %#v", len(data.Backlinks), data.Backlinks)
	}
}
func TestLinkService_UpdateLinksForPage_OnlyAffectsOnePage(t *testing.T) {
	svc, ts, _ := setupLinkService(t)
	pageAID, pageBID := createSimpleLinkedPages(t, ts)

	pageA, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage a failed: %v", err)
	}
	if err := svc.UpdateLinksForPage(pageA, pageA.Content); err != nil {
		t.Fatalf("UpdateLinksForPage failed: %v", err)
	}

	dataB, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage for B failed: %v", err)
	}
	if len(dataB.Backlinks) != 1 {
		t.Fatalf("expected 1 backlink for B, got %d: %#v", len(dataB.Backlinks), dataB.Backlinks)
	}

	dataA, err := svc.GetBacklinksForPage(pageAID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage for A failed: %v", err)
	}
	if len(dataA.Backlinks) != 0 {
		t.Fatalf("expected 0 backlinks for A, got %d: %#v", len(dataA.Backlinks), dataA.Backlinks)
	}
}

func TestLinkService_ClearLinks_RemovesAllLinks(t *testing.T) {
	svc, ts, _ := setupLinkService(t)
	_, pageBID := createSimpleLinkedPages(t, ts)

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	if err := svc.ClearLinks(); err != nil {
		t.Fatalf("ClearLinks failed: %v", err)
	}

	data, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}
	if len(data.Backlinks) != 0 {
		t.Fatalf("expected 0 backlinks after ClearBacklinks, got %d: %#v", len(data.Backlinks), data.Backlinks)
	}
}

func TestLinkService_GetOutgoingLinksForPage_ReturnsOutgoingLinks(t *testing.T) {
	svc, ts, _ := setupLinkService(t)
	pageAID, pageBID := createSimpleLinkedPages(t, ts)

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	result, err := svc.GetOutgoingLinksForPage(pageAID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage failed: %v", err)
	}

	if result == nil {
		t.Fatalf("expected non-nil result")
	}

	if result.Count != 1 {
		t.Fatalf("expected 1 outgoing link for pageA, got %d: %#v", result.Count, result.Outgoings)
	}

	item := result.Outgoings[0]

	if item.FromPageID != pageAID {
		t.Errorf("FromPageID = %q, want %q", item.FromPageID, pageAID)
	}

	if item.ToPageID != pageBID {
		t.Errorf("ToPageID = %q, want %q", item.ToPageID, pageBID)
	}

	pageB, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage(pageB) failed: %v", err)
	}
	if item.ToPath != "/b" {
		t.Errorf("ToPath = %q, want %q", item.ToPath, "/b")
	}
	if item.ToPageTitle != pageB.Title {
		t.Errorf("ToPageTitle = %q, want %q", item.ToPageTitle, pageB.Title)
	}
}

func TestLinkService_GetOutgoingLinksForPage_NoOutgoings(t *testing.T) {
	svc, ts, _ := setupLinkService(t)

	aIDPtr, err := ts.CreateNode("system", nil, "Lonely Page", "lonely", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode lonely failed: %v", err)
	}
	lonelyID := *aIDPtr

	page, err := ts.GetPage(lonelyID)
	if err != nil {
		t.Fatalf("GetPage lonely failed: %v", err)
	}

	var noLinks string = "Just some text, no links."
	if err := ts.UpdateNode("system", page.ID, page.Title, page.Slug, &noLinks); err != nil {
		t.Fatalf("UpdateNode lonely failed: %v", err)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	result, err := svc.GetOutgoingLinksForPage(lonelyID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage failed: %v", err)
	}

	if result == nil {
		t.Fatalf("expected non-nil result")
	}

	if result.Count != 0 {
		t.Fatalf("expected 0 outgoing links, got %d: %#v", result.Count, result.Outgoings)
	}
}

func TestToOutgoingResult_MapsOutgoingToResultItems(t *testing.T) {
	ts, page1ID, page2ID := setupTreeForLinksTest(t)

	root := ts.GetTree()
	if root == nil {
		t.Fatalf("tree root is nil")
	}

	outgoings := []Outgoing{{FromPageID: page1ID, ToPageID: page2ID, ToPath: "/docs/page2", Broken: false, FromTitle: "Page 1"}}

	result := toOutgoingLinkResult(ts, outgoings)
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.Count != 1 {
		t.Fatalf("expected 1 outgoing, got %d", result.Count)
	}

	item := result.Outgoings[0]

	if item.FromPageID != page1ID {
		t.Errorf("FromPageID = %q, want %q", item.FromPageID, page1ID)
	}
	if item.ToPageID != page2ID {
		t.Errorf("ToPageID = %q, want %q", item.ToPageID, page2ID)
	}

	page2, err := ts.GetPage(page2ID)
	if err != nil {
		t.Fatalf("GetPage page2 failed: %v", err)
	}
	if item.ToPageTitle != page2.Title {
		t.Errorf("ToPageTitle = %q, want %q", item.ToPageTitle, page2.Title)
	}
	if item.ToPath != "/docs/page2" {
		t.Errorf("ToPath = %q, want %q", item.ToPath, "/docs/page2")
	}
	if item.Broken {
		t.Errorf("Broken = %v, want %v", item.Broken, false)
	}

}

func TestLinkService_LateCreatedTarget_BecomesResolvedAfterReindex(t *testing.T) {
	svc, ts, _ := setupLinkService(t)

	aIDPtr, err := ts.CreateNode("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode a failed: %v", err)
	}
	pageAID := *aIDPtr

	aPage, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage a failed: %v", err)
	}
	var linkToB string = "Link to B: [Go](/b)"
	if err := ts.UpdateNode("system", aPage.ID, aPage.Title, aPage.Slug, &linkToB); err != nil {
		t.Fatalf("UpdateNode a failed: %v", err)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	out1, err := svc.GetOutgoingLinksForPage(pageAID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage failed: %v", err)
	}
	if out1.Count != 1 {
		t.Fatalf("expected 1 outgoing, got %d: %#v", out1.Count, out1.Outgoings)
	}
	if out1.Outgoings[0].Broken != true {
		t.Fatalf("expected outgoing to be broken, got %#v", out1.Outgoings[0])
	}
	if out1.Outgoings[0].ToPath != "/b" {
		t.Fatalf("expected ToPath '/b', got %q", out1.Outgoings[0].ToPath)
	}
	if out1.Outgoings[0].ToPageID != "" {
		t.Fatalf("expected empty ToPageID for broken link, got %q", out1.Outgoings[0].ToPageID)
	}

	bIDPtr, err := ts.CreateNode("system", nil, "Page B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode b failed: %v", err)
	}
	pageBID := *bIDPtr

	bPage, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage b failed: %v", err)
	}
	var pageBContent string = "# Page B"
	if err := ts.UpdateNode("system", bPage.ID, bPage.Title, bPage.Slug, &pageBContent); err != nil {
		t.Fatalf("UpdateNode b failed: %v", err)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages (second) failed: %v", err)
	}

	out2, err := svc.GetOutgoingLinksForPage(pageAID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage (second) failed: %v", err)
	}
	if out2.Count != 1 {
		t.Fatalf("expected 1 outgoing, got %d: %#v", out2.Count, out2.Outgoings)
	}
	if out2.Outgoings[0].Broken != false {
		t.Fatalf("expected outgoing to be resolved, got %#v", out2.Outgoings[0])
	}
	if out2.Outgoings[0].ToPageID != pageBID {
		t.Fatalf("expected ToPageID %q, got %q", pageBID, out2.Outgoings[0].ToPageID)
	}

	bl, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}
	if bl.Count != 1 {
		t.Fatalf("expected 1 backlink, got %d: %#v", bl.Count, bl.Backlinks)
	}
	if bl.Backlinks[0].FromPageID != pageAID {
		t.Fatalf("expected FromPageID %q, got %q", pageAID, bl.Backlinks[0].FromPageID)
	}
	if bl.Backlinks[0].ToPageID != pageBID {
		t.Fatalf("expected ToPageID %q, got %q", pageBID, bl.Backlinks[0].ToPageID)
	}
}

func TestLinkService_HealOnPageCreate_ResolvesBrokenLinksWithoutReindex(t *testing.T) {
	svc, ts, _ := setupLinkService(t)

	aIDPtr, err := ts.CreateNode("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode A failed: %v", err)
	}
	pageAID := *aIDPtr

	pageA, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage A failed: %v", err)
	}
	var linkToB string = "Link to B: [Go](/b)"
	if err := ts.UpdateNode("system", pageA.ID, pageA.Title, pageA.Slug, &linkToB); err != nil {
		t.Fatalf("UpdateNode A failed: %v", err)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	out1, err := svc.GetOutgoingLinksForPage(pageAID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage failed: %v", err)
	}
	if out1.Count != 1 {
		t.Fatalf("expected 1 outgoing for A, got %d: %#v", out1.Count, out1.Outgoings)
	}

	if out1.Outgoings[0].Broken != true {
		t.Fatalf("expected outgoing to be broken before heal, got %#v", out1.Outgoings[0])
	}
	if out1.Outgoings[0].ToPath != "/b" {
		t.Fatalf("expected ToPath '/b' before heal, got %q", out1.Outgoings[0].ToPath)
	}
	if out1.Outgoings[0].ToPageID != "" {
		t.Fatalf("expected empty ToPageID before heal, got %q", out1.Outgoings[0].ToPageID)
	}

	bIDPtr, err := ts.CreateNode("system", nil, "Page B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode B failed: %v", err)
	}
	pageBID := *bIDPtr

	pageB, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage B failed: %v", err)
	}

	if err := svc.HealLinksForExactPath(pageB); err != nil {
		t.Fatalf("HealLinksForExactPath failed: %v", err)
	}

	out2, err := svc.GetOutgoingLinksForPage(pageAID)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage (after heal) failed: %v", err)
	}
	if out2.Count != 1 {
		t.Fatalf("expected 1 outgoing for A after heal, got %d: %#v", out2.Count, out2.Outgoings)
	}

	if out2.Outgoings[0].Broken != false {
		t.Fatalf("expected outgoing to be resolved after heal, got %#v", out2.Outgoings[0])
	}
	if out2.Outgoings[0].ToPageID != pageBID {
		t.Fatalf("expected ToPageID %q after heal, got %q", pageBID, out2.Outgoings[0].ToPageID)
	}

	bl, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}
	if bl.Count != 1 {
		t.Fatalf("expected 1 backlink for B after heal, got %d: %#v", bl.Count, bl.Backlinks)
	}
	if bl.Backlinks[0].FromPageID != pageAID {
		t.Fatalf("expected backlink FromPageID %q, got %q", pageAID, bl.Backlinks[0].FromPageID)
	}
	if bl.Backlinks[0].ToPageID != pageBID {
		t.Fatalf("expected backlink ToPageID %q, got %q", pageBID, bl.Backlinks[0].ToPageID)
	}
}

func TestLinksStore_GetBrokenIncomingForPath_ReturnsBrokenLinks(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	// Create three pages: A, B, C
	aIDPtr, err := ts.CreateNode("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode A failed: %v", err)
	}
	pageAID := *aIDPtr

	bIDPtr, err := ts.CreateNode("system", nil, "Page B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode B failed: %v", err)
	}
	pageBID := *bIDPtr

	cIDPtr, err := ts.CreateNode("system", nil, "Page C", "c", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode C failed: %v", err)
	}
	pageCID := *cIDPtr

	// Update A and B to link to a non-existent page "/nonexistent"
	pageA, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage A failed: %v", err)
	}
	var linkToMissing string = "Link: [Missing](/nonexistent)"
	if err := ts.UpdateNode("system", pageA.ID, pageA.Title, pageA.Slug, &linkToMissing); err != nil {
		t.Fatalf("UpdateNode A failed: %v", err)
	}

	pageB, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage B failed: %v", err)
	}
	if err := ts.UpdateNode("system", pageB.ID, pageB.Title, pageB.Slug, &linkToMissing); err != nil {
		t.Fatalf("UpdateNode B failed: %v", err)
	}

	// Page C links to a different broken page
	pageC, err := ts.GetPage(pageCID)
	if err != nil {
		t.Fatalf("GetPage C failed: %v", err)
	}
	var linkToOther string = "Link: [Other](/other-missing)"
	if err := ts.UpdateNode("system", pageC.ID, pageC.Title, pageC.Slug, &linkToOther); err != nil {
		t.Fatalf("UpdateNode C failed: %v", err)
	}

	// Index all pages to create broken links
	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	// Test: GetBrokenIncomingForPath should return broken links for "/nonexistent"
	brokenLinks, err := store.GetBrokenIncomingForPath("/nonexistent")
	if err != nil {
		t.Fatalf("GetBrokenIncomingForPath failed: %v", err)
	}

	if len(brokenLinks) != 2 {
		t.Fatalf("expected 2 broken links for /nonexistent, got %d: %#v", len(brokenLinks), brokenLinks)
	}

	// Verify all returned links are marked as broken
	for i, link := range brokenLinks {
		if !link.Broken {
			t.Errorf("brokenLinks[%d].Broken = %v, want true", i, link.Broken)
		}
		if link.ToPageID != "" {
			t.Errorf("brokenLinks[%d].ToPageID = %q, want empty string for broken link", i, link.ToPageID)
		}
		if link.FromTitle == "" {
			t.Errorf("brokenLinks[%d].FromTitle should not be empty", i)
		}
	}

	// Verify the links come from pages A and B
	fromPageIDs := map[string]struct{}{}
	for _, link := range brokenLinks {
		fromPageIDs[link.FromPageID] = struct{}{}
	}
	if _, found := fromPageIDs[pageAID]; !found {
		t.Errorf("expected broken link from page A (%s)", pageAID)
	}
	if _, found := fromPageIDs[pageBID]; !found {
		t.Errorf("expected broken link from page B (%s)", pageBID)
	}
}

func TestLinksStore_GetBrokenIncomingForPath_FiltersByPath(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	aIDPtr, err := ts.CreateNode("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode A failed: %v", err)
	}
	pageAID := *aIDPtr

	bIDPtr, err := ts.CreateNode("system", nil, "Page B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode B failed: %v", err)
	}
	pageBID := *bIDPtr

	// Page A links to "/missing1"
	pageA, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage A failed: %v", err)
	}
	var linkToMissing1 string = "Link: [Missing1](/missing1)"
	if err := ts.UpdateNode("system", pageA.ID, pageA.Title, pageA.Slug, &linkToMissing1); err != nil {
		t.Fatalf("UpdateNode A failed: %v", err)
	}

	// Page B links to "/missing2"
	pageB, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage B failed: %v", err)
	}
	var linkToMissing2 string = "Link: [Missing2](/missing2)"
	if err := ts.UpdateNode("system", pageB.ID, pageB.Title, pageB.Slug, &linkToMissing2); err != nil {
		t.Fatalf("UpdateNode B failed: %v", err)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	// Test: Should only return broken links for "/missing1"
	broken1, err := store.GetBrokenIncomingForPath("/missing1")
	if err != nil {
		t.Fatalf("GetBrokenIncomingForPath(/missing1) failed: %v", err)
	}

	if len(broken1) != 1 {
		t.Fatalf("expected 1 broken link for /missing1, got %d: %#v", len(broken1), broken1)
	}
	if broken1[0].FromPageID != pageAID {
		t.Errorf("broken link FromPageID = %q, want %q", broken1[0].FromPageID, pageAID)
	}

	// Test: Should only return broken links for "/missing2"
	broken2, err := store.GetBrokenIncomingForPath("/missing2")
	if err != nil {
		t.Fatalf("GetBrokenIncomingForPath(/missing2) failed: %v", err)
	}

	if len(broken2) != 1 {
		t.Fatalf("expected 1 broken link for /missing2, got %d: %#v", len(broken2), broken2)
	}
	if broken2[0].FromPageID != pageBID {
		t.Errorf("broken link FromPageID = %q, want %q", broken2[0].FromPageID, pageBID)
	}
}

func TestLinksStore_GetBrokenIncomingForPath_EmptyWhenNoBrokenLinks(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	aIDPtr, err := ts.CreateNode("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode A failed: %v", err)
	}
	pageAID := *aIDPtr

	_, err = ts.CreateNode("system", nil, "Page B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode B failed: %v", err)
	}

	// Page A links to existing Page B (not broken)
	pageA, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage A failed: %v", err)
	}
	var linkToB string = "Link: [To B](/b)"
	if err := ts.UpdateNode("system", pageA.ID, pageA.Title, pageA.Slug, &linkToB); err != nil {
		t.Fatalf("UpdateNode A failed: %v", err)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	// Test: Should return empty for "/b" since the link is not broken
	brokenLinks, err := store.GetBrokenIncomingForPath("/b")
	if err != nil {
		t.Fatalf("GetBrokenIncomingForPath failed: %v", err)
	}

	if len(brokenLinks) != 0 {
		t.Fatalf("expected 0 broken links for /b (link exists), got %d: %#v", len(brokenLinks), brokenLinks)
	}

	// Test: Should return empty for a path that has no links at all
	noLinks, err := store.GetBrokenIncomingForPath("/never-linked")
	if err != nil {
		t.Fatalf("GetBrokenIncomingForPath(/never-linked) failed: %v", err)
	}

	if len(noLinks) != 0 {
		t.Fatalf("expected 0 broken links for /never-linked, got %d: %#v", len(noLinks), noLinks)
	}
}

func TestLinksStore_GetBrokenIncomingForPath_OrdersByFromTitle(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	// Create three pages with titles that should be ordered alphabetically
	zIDPtr, err := ts.CreateNode("system", nil, "Zebra Page", "z", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode Z failed: %v", err)
	}

	aIDPtr, err := ts.CreateNode("system", nil, "Alpha Page", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode A failed: %v", err)
	}

	mIDPtr, err := ts.CreateNode("system", nil, "Middle Page", "m", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode M failed: %v", err)
	}

	// All three pages link to the same non-existent page
	pageIDs := []string{*zIDPtr, *aIDPtr, *mIDPtr}
	for _, id := range pageIDs {
		page, err := ts.GetPage(id)
		if err != nil {
			t.Fatalf("GetPage(%s) failed: %v", id, err)
		}
		var linkToMissing string = "Link: [Missing](/missing)"
		if err := ts.UpdateNode("system", page.ID, page.Title, page.Slug, &linkToMissing); err != nil {
			t.Fatalf("UpdateNode(%s) failed: %v", id, err)
		}
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	// Test: Results should be ordered by from_title ASC
	brokenLinks, err := store.GetBrokenIncomingForPath("/missing")
	if err != nil {
		t.Fatalf("GetBrokenIncomingForPath failed: %v", err)
	}

	if len(brokenLinks) != 3 {
		t.Fatalf("expected 3 broken links, got %d: %#v", len(brokenLinks), brokenLinks)
	}

	// Verify ordering: Alpha Page, Middle Page, Zebra Page
	expectedTitles := []string{"Alpha Page", "Middle Page", "Zebra Page"}
	for i, expected := range expectedTitles {
		if brokenLinks[i].FromTitle != expected {
			t.Errorf("brokenLinks[%d].FromTitle = %q, want %q", i, brokenLinks[i].FromTitle, expected)
		}
	}
}

func TestLinksStore_GetBrokenIncomingForPath_OnlyReturnsBrokenNotResolved(t *testing.T) {
	svc, ts, store := setupLinkService(t)

	// Create Page A that links to a non-existent page
	aIDPtr, err := ts.CreateNode("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode A failed: %v", err)
	}
	pageAID := *aIDPtr

	pageA, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage A failed: %v", err)
	}
	var linkToB string = "Link: [To B](/b)"
	if err := ts.UpdateNode("system", pageA.ID, pageA.Title, pageA.Slug, &linkToB); err != nil {
		t.Fatalf("UpdateNode A failed: %v", err)
	}

	// Index - this creates a broken link since B doesn't exist
	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages (first) failed: %v", err)
	}

	// Verify the broken link exists
	brokenBefore, err := store.GetBrokenIncomingForPath("/b")
	if err != nil {
		t.Fatalf("GetBrokenIncomingForPath (before) failed: %v", err)
	}
	if len(brokenBefore) != 1 {
		t.Fatalf("expected 1 broken link before creating B, got %d", len(brokenBefore))
	}

	// Now create Page B - this should heal the link
	bIDPtr, err := ts.CreateNode("system", nil, "Page B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreateNode B failed: %v", err)
	}
	pageBID := *bIDPtr

	pageB, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage B failed: %v", err)
	}
	var contentB string = "# Page B"
	if err := ts.UpdateNode("system", pageB.ID, pageB.Title, pageB.Slug, &contentB); err != nil {
		t.Fatalf("UpdateNode B failed: %v", err)
	}

	// Use HealLinksForExactPath to heal the broken link
	if err := svc.HealLinksForExactPath(pageB); err != nil {
		t.Fatalf("HealLinksForExactPath failed: %v", err)
	}

	// Verify the link is no longer broken
	brokenAfter, err := store.GetBrokenIncomingForPath("/b")
	if err != nil {
		t.Fatalf("GetBrokenIncomingForPath (after) failed: %v", err)
	}
	if len(brokenAfter) != 0 {
		t.Fatalf("expected 0 broken links after healing, got %d: %#v", len(brokenAfter), brokenAfter)
	}

	// Verify the link still exists but is not broken
	backlinks, err := store.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}
	if len(backlinks) != 1 {
		t.Fatalf("expected 1 resolved backlink, got %d: %#v", len(backlinks), backlinks)
	}
	if backlinks[0].FromPageID != pageAID {
		t.Errorf("backlink FromPageID = %q, want %q", backlinks[0].FromPageID, pageAID)
	}
}
