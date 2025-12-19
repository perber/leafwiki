package links

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

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
	docsIDPtr, err := ts.CreatePage(nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}
	docsID := *docsIDPtr

	// create "page1" and "page2" under docs
	page1IDPtr, err := ts.CreatePage(&docsID, "Page 1", "page1")
	if err != nil {
		t.Fatalf("CreatePage page1 failed: %v", err)
	}
	page2IDPtr, err := ts.CreatePage(&docsID, "Page 2", "page2")
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

	aIDPtr, err := ts.CreatePage(nil, "Page A", "a")
	if err != nil {
		t.Fatalf("CreatePage a failed: %v", err)
	}
	pageAID = *aIDPtr

	bIDPtr, err := ts.CreatePage(nil, "Page B", "b")
	if err != nil {
		t.Fatalf("CreatePage b failed: %v", err)
	}
	pageBID = *bIDPtr

	aPage, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage a failed: %v", err)
	}
	contentA := "Link to B: [Go to B](/b)"
	if err := ts.UpdatePage(aPage.ID, aPage.Title, aPage.Slug, contentA); err != nil {
		t.Fatalf("UpdatePage a failed: %v", err)
	}

	bPage, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage b failed: %v", err)
	}
	contentB := "# Page B\nNo outgoing links."
	if err := ts.UpdatePage(bPage.ID, bPage.Title, bPage.Slug, contentB); err != nil {
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
	if err := ts.UpdatePage(aPage.ID, aPage.Title, aPage.Slug, "No more links."); err != nil {
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

func TestLinkService_RemoveLinksForPage_RemovesIncomingAndOutgoing(t *testing.T) {
	svc, ts, _ := setupLinkService(t)
	pageAID, pageBID := createSimpleLinkedPages(t, ts)

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	if err := svc.RemoveLinksForPage(pageAID); err != nil {
		t.Fatalf("RemoveLinksForPage failed: %v", err)
	}

	dataB, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}
	if len(dataB.Backlinks) != 0 {
		t.Fatalf("expected 0 backlinks for B after removal, got %d: %#v", len(dataB.Backlinks), dataB.Backlinks)
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
	wantPath := pageB.CalculatePath()
	if item.ToPath != wantPath {
		t.Errorf("ToPath = %q, want %q", item.ToPath, wantPath)
	}
	if item.ToPageTitle != pageB.Title {
		t.Errorf("ToPageTitle = %q, want %q", item.ToPageTitle, pageB.Title)
	}
}

func TestLinkService_GetOutgoingLinksForPage_NoOutgoings(t *testing.T) {
	svc, ts, _ := setupLinkService(t)

	aIDPtr, err := ts.CreatePage(nil, "Lonely Page", "lonely")
	if err != nil {
		t.Fatalf("CreatePage lonely failed: %v", err)
	}
	lonelyID := *aIDPtr

	page, err := ts.GetPage(lonelyID)
	if err != nil {
		t.Fatalf("GetPage lonely failed: %v", err)
	}

	if err := ts.UpdatePage(page.ID, page.Title, page.Slug, "Just some text, no links."); err != nil {
		t.Fatalf("UpdatePage lonely failed: %v", err)
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

	outgoings := []Outgoing{{FromPageID: page1ID, ToPageID: page2ID, FromTitle: "Page 1"}}

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
	if item.ToPath != page2.CalculatePath() {
		t.Errorf("ToPath = %q, want %q", item.ToPath, page2.CalculatePath())
	}
}

func TestLinkService_LateCreatedTarget_BecomesResolvedAfterReindex(t *testing.T) {
	svc, ts, _ := setupLinkService(t)

	// A existiert, B noch nicht
	aIDPtr, err := ts.CreatePage(nil, "Page A", "a")
	if err != nil {
		t.Fatalf("CreatePage a failed: %v", err)
	}
	pageAID := *aIDPtr

	aPage, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage a failed: %v", err)
	}
	if err := ts.UpdatePage(aPage.ID, aPage.Title, aPage.Slug, "Link to B: [Go](/b)"); err != nil {
		t.Fatalf("UpdatePage a failed: %v", err)
	}

	// Index: B fehlt -> outgoing broken
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

	// Jetzt B anlegen
	bIDPtr, err := ts.CreatePage(nil, "Page B", "b")
	if err != nil {
		t.Fatalf("CreatePage b failed: %v", err)
	}
	pageBID := *bIDPtr

	bPage, err := ts.GetPage(pageBID)
	if err != nil {
		t.Fatalf("GetPage b failed: %v", err)
	}
	if err := ts.UpdatePage(bPage.ID, bPage.Title, bPage.Slug, "# Page B"); err != nil {
		t.Fatalf("UpdatePage b failed: %v", err)
	}

	// Reindex: jetzt sollte es resolved sein
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

	// Und Backlink bei B muss existieren
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
