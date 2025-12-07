package backlinks

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

func TestNormalizeLink_Absolute(t *testing.T) {
	current := "docs/guide/page1"
	link := "/docs/other/page2"

	got := normalizeLink(current, link)
	want := "docs/other/page2"

	if got != want {
		t.Errorf("normalizeLink(%q, %q) = %q, want %q", current, link, got, want)
	}
}

func TestNormalizeLink_RelativeSameDir(t *testing.T) {
	current := "docs/guide/page1"
	link := "page2"

	got := normalizeLink(current, link)
	want := "docs/guide/page2"

	if got != want {
		t.Errorf("normalizeLink(%q, %q) = %q, want %q", current, link, got, want)
	}
}

func TestNormalizeLink_RelativeParentDir(t *testing.T) {
	current := "docs/guide/page1"
	link := "../overview"

	got := normalizeLink(current, link)
	want := "docs/overview"

	if got != want {
		t.Errorf("normalizeLink(%q, %q) = %q, want %q", current, link, got, want)
	}
}

func TestNormalizeLink_Empty(t *testing.T) {
	got := normalizeLink("docs/guide/page1", "")
	if got != "" {
		t.Errorf("normalizeLink with empty link = %q, want empty string", got)
	}
}

// helper to create a small tree structure:
// root
//
//	└─ docs
//	     ├─ page1
//	     └─ page2
func setupTreeForBacklinksTest(t *testing.T) (*tree.TreeService, string, string) {
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
	ts, page1ID, page2ID := setupTreeForBacklinksTest(t)

	// current page: docs/page1
	page1, err := ts.GetPage(page1ID)
	if err != nil {
		t.Fatalf("GetPage(page1) failed: %v", err)
	}
	currentPath := page1.CalculatePath() // should be "docs/page1"

	// we want to link from page1 to page2 using a relative link
	links := []string{"./page2"}

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

func TestResolveTargetLinks_IgnoresNonExistingTargets(t *testing.T) {
	ts, page1ID, _ := setupTreeForBacklinksTest(t)

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

	if len(targets) != 0 {
		t.Fatalf("expected 0 target links, got %d: %#v", len(targets), targets)
	}
}

func setupBacklinkService(t *testing.T) (*BacklinkService, *tree.TreeService, *BacklinksStore) {
	t.Helper()

	dataDir := t.TempDir()

	ts := tree.NewTreeService(dataDir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	store, err := NewBacklinksStore(dataDir)
	if err != nil {
		t.Fatalf("NewBacklinksStore failed: %v", err)
	}

	svc := NewBacklinkService(dataDir, ts, store)
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
	contentA := "Link to B: [Go to B](b)"
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

func TestBacklinkService_IndexAllPages_BuildsBacklinks(t *testing.T) {
	svc, ts, _ := setupBacklinkService(t)
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

// Testet, dass IndexAllPages alte Backlinks überschreibt (Clear + Reindex)
func TestBacklinkService_IndexAllPages_ReplacesExistingBacklinks(t *testing.T) {
	svc, ts, _ := setupBacklinkService(t)
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
func TestBacklinkService_UpdateBacklinksForPage_OnlyAffectsOnePage(t *testing.T) {
	svc, ts, _ := setupBacklinkService(t)
	pageAID, pageBID := createSimpleLinkedPages(t, ts)

	pageA, err := ts.GetPage(pageAID)
	if err != nil {
		t.Fatalf("GetPage a failed: %v", err)
	}
	if err := svc.UpdateBacklinksForPage(pageA, pageA.Content); err != nil {
		t.Fatalf("UpdateBacklinksForPage failed: %v", err)
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

func TestBacklinkService_ClearBacklinks_RemovesAllBacklinks(t *testing.T) {
	svc, ts, _ := setupBacklinkService(t)
	_, pageBID := createSimpleLinkedPages(t, ts)

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	if err := svc.ClearBacklinks(); err != nil {
		t.Fatalf("ClearBacklinks failed: %v", err)
	}

	data, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}
	if len(data.Backlinks) != 0 {
		t.Fatalf("expected 0 backlinks after ClearBacklinks, got %d: %#v", len(data.Backlinks), data.Backlinks)
	}
}

func TestBacklinkService_RemoveBacklinksForPage_RemovesIncomingAndOutgoing(t *testing.T) {
	svc, ts, _ := setupBacklinkService(t)
	pageAID, pageBID := createSimpleLinkedPages(t, ts)

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	if err := svc.RemoveBacklinksForPage(pageAID); err != nil {
		t.Fatalf("RemoveBacklinksForPage failed: %v", err)
	}

	dataB, err := svc.GetBacklinksForPage(pageBID)
	if err != nil {
		t.Fatalf("GetBacklinksForPage failed: %v", err)
	}
	if len(dataB.Backlinks) != 0 {
		t.Fatalf("expected 0 backlinks for B after removal, got %d: %#v", len(dataB.Backlinks), dataB.Backlinks)
	}
}
