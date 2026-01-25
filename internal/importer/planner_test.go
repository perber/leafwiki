package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

type fakeWiki struct {
	treeHash string

	// planner part
	lookups   map[string]*tree.PathLookup
	lookupErr error

	// executor part
	ensureCalls        int
	updateCalls        int
	lastUpdatedContent *string

	ensureErr     error
	ensureNilPage bool
	updateErr     error
}

func (f *fakeWiki) TreeHash() string { return f.treeHash }

func (f *fakeWiki) LookupPagePath(p string) (*tree.PathLookup, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	if v, ok := f.lookups[p]; ok {
		return v, nil
	}
	return &tree.PathLookup{Path: p, Exists: false, Segments: []tree.PathSegment{}}, nil
}

func (f *fakeWiki) EnsurePath(userID string, targetPath string, title string, kind *tree.NodeKind) (*tree.Page, error) {
	f.ensureCalls++
	if f.ensureErr != nil {
		return nil, f.ensureErr
	}
	if f.ensureNilPage {
		return nil, nil
	}
	k := tree.NodeKindPage
	if kind != nil {
		k = *kind
	}
	// create minimal page object
	return &tree.Page{PageNode: &tree.PageNode{
		ID:    "p1",
		Title: title,
		Slug:  "slug",
		Kind:  k,
	}}, nil
}

func (f *fakeWiki) UpdatePage(userID string, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error) {
	f.updateCalls++
	f.lastUpdatedContent = content
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	// simulate tree change after update
	f.treeHash = f.treeHash + "-changed"
	k := tree.NodeKindPage
	if kind != nil {
		k = *kind
	}
	return &tree.Page{PageNode: &tree.PageNode{
		ID:    id,
		Title: title,
		Slug:  slug,
		Kind:  k,
	}}, nil
}

func writeFile(t *testing.T, base, rel, content string) string {
	t.Helper()
	abs := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return abs
}

func newPlannerWithFake(w *fakeWiki) *Planner {
	return NewPlanner(w, tree.NewSlugService())
}

func TestPlanner_CreatePlan_CreateNewPage_NonIndex(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "My Page.md", "# Hello\n\nbody")

	wiki := &fakeWiki{
		treeHash: "h1",
		lookups:  map[string]*tree.PathLookup{},
	}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "My Page.md"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "/docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	if res.TreeHash != "h1" {
		t.Fatalf("TreeHash = %q", res.TreeHash)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("Errors = %#v", res.Errors)
	}
	if len(res.Items) != 1 {
		t.Fatalf("Items len = %d", len(res.Items))
	}

	it := res.Items[0]
	if it.Action != PlanActionCreate {
		t.Fatalf("Action = %q", it.Action)
	}
	if it.Kind != tree.NodeKindPage {
		t.Fatalf("Kind = %v", it.Kind)
	}
	if it.Title != "Hello" {
		t.Fatalf("Title = %q", it.Title)
	}
	if it.TargetPath != "docs/my-page" {
		t.Fatalf("TargetPath = %q (want docs/my-page)", it.TargetPath)
	}
	if it.DesiredSlug != "my-page" {
		t.Fatalf("DesiredSlug = %q (want my-page)", it.DesiredSlug)
	}
}

func TestPlanner_CreatePlan_CreateNewSection_IndexMd(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "Guides/index.md", "---\ntitle: Guides\n---\n\n# Ignored")

	wiki := &fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "Guides/index.md"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	it := res.Items[0]

	if it.Kind != tree.NodeKindSection {
		t.Fatalf("Kind = %v", it.Kind)
	}
	if it.Action != PlanActionCreate {
		t.Fatalf("Action = %q", it.Action)
	}
	if it.TargetPath != "docs/guides" {
		t.Fatalf("TargetPath = %q (want docs/guides)", it.TargetPath)
	}
	if it.DesiredSlug != "guides" {
		t.Fatalf("DesiredSlug = %q (want guides)", it.DesiredSlug)
	}
	if it.Title != "Guides" {
		t.Fatalf("Title = %q", it.Title)
	}
}

func TestPlanner_CreatePlan_SkipExisting_UsesLookupLastSegment(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "a.md", "# A")

	existingID := "id123"
	existingKind := tree.NodeKindPage
	existingTitle := "Existing A"

	wiki := &fakeWiki{
		treeHash: "h",
		lookups: map[string]*tree.PathLookup{
			"docs/a": {
				Path:   "docs/a",
				Exists: true,
				Segments: []tree.PathSegment{
					{Slug: "docs", Exists: true},
					{Slug: "a", Exists: true, ID: &existingID, Kind: &existingKind, Title: &existingTitle},
				},
			},
		},
	}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "a.md"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("Errors = %#v", res.Errors)
	}

	it := res.Items[0]
	if it.Action != PlanActionSkip {
		t.Fatalf("Action = %q", it.Action)
	}
	if !it.Exists {
		t.Fatalf("Exists = false")
	}
	if it.ExistingID == nil || *it.ExistingID != existingID {
		t.Fatalf("ExistingID = %#v (want %q)", it.ExistingID, existingID)
	}
	if it.DesiredSlug != "a" {
		t.Fatalf("DesiredSlug = %q (want a)", it.DesiredSlug)
	}
}

func TestPlanner_CreatePlan_Error_SourceMissing_IsCollected(t *testing.T) {
	tmp := t.TempDir()
	wiki := &fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "missing.md"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	if len(res.Items) != 0 {
		t.Fatalf("Items len = %d (want 0)", len(res.Items))
	}
	if len(res.Errors) != 1 {
		t.Fatalf("Errors len = %d (want 1)", len(res.Errors))
	}
}

func TestPlanner_CreatePlan_Error_SourceIsDirectory_IsCollected(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	wiki := &fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "dir"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	if len(res.Items) != 0 {
		t.Fatalf("Items len = %d (want 0)", len(res.Items))
	}
	if len(res.Errors) != 1 {
		t.Fatalf("Errors len = %d (want 1)", len(res.Errors))
	}
}

func TestPlanner_CreatePlan_Error_ExistingZeroSegments_IsCollected(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "x.md", "# X")

	wiki := &fakeWiki{
		treeHash: "h",
		lookups: map[string]*tree.PathLookup{
			"docs/x": {Path: "docs/x", Exists: true, Segments: []tree.PathSegment{}},
		},
	}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "x.md"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	if len(res.Items) != 0 {
		t.Fatalf("Items len = %d (want 0)", len(res.Items))
	}
	if len(res.Errors) != 1 {
		t.Fatalf("Errors len = %d (want 1)", len(res.Errors))
	}
}

// ---- Title extraction -------------------------------------------------------

func TestPlanner_extractTitleFromMDFile_FrontmatterTitleWins(t *testing.T) {
	tmp := t.TempDir()
	abs := writeFile(t, tmp, "t.md", "---\ntitle: FM Title\n---\n\n# Heading")

	p := newPlannerWithFake(&fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}})

	title, err := p.extractTitleFromMDFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "FM Title" {
		t.Fatalf("title = %q", title)
	}
}

func TestPlanner_extractTitleFromMDFile_LeafwikiTitle(t *testing.T) {
	tmp := t.TempDir()
	abs := writeFile(t, tmp, "t.md", "---\nleafwiki_title: Leaf\n---\n\n# Heading")

	p := newPlannerWithFake(&fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}})

	title, err := p.extractTitleFromMDFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "Leaf" {
		t.Fatalf("title = %q", title)
	}
}

func TestPlanner_extractTitleFromMDFile_FirstHeadingFallback(t *testing.T) {
	tmp := t.TempDir()
	abs := writeFile(t, tmp, "t.md", "no fm\n\n# Heading Only\nx")

	p := newPlannerWithFake(&fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}})

	title, err := p.extractTitleFromMDFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "Heading Only" {
		t.Fatalf("title = %q", title)
	}
}

func TestPlanner_extractTitleFromMDFile_FilenameFallback(t *testing.T) {
	tmp := t.TempDir()
	abs := writeFile(t, tmp, "some-file.md", "no title")

	p := newPlannerWithFake(&fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}})

	title, err := p.extractTitleFromMDFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "some-file" {
		t.Fatalf("title = %q", title)
	}
}

func TestPlanner_CreatePlan_TitleExtractionError_AddsNote(t *testing.T) {
	tmp := t.TempDir()
	abs := writeFile(t, tmp, "unreadable.md", "# Title")

	// Make file unreadable to trigger extraction error
	if err := os.Chmod(abs, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() {
		if err := os.Chmod(abs, 0o644); err != nil { // restore for cleanup
			t.Fatalf("chmod restore: %v", err)
		}
	}()

	wiki := &fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "unreadable.md"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("Errors = %#v", res.Errors)
	}
	if len(res.Items) != 1 {
		t.Fatalf("Items len = %d (want 1)", len(res.Items))
	}

	it := res.Items[0]
	if len(it.Notes) != 1 {
		t.Fatalf("Notes len = %d (want 1)", len(it.Notes))
	}
	if !strings.Contains(it.Notes[0], "Failed to extract title") {
		t.Fatalf("Note = %q (should contain 'Failed to extract title')", it.Notes[0])
	}
	// Title should still be set (fallback to filename)
	if it.Title != "unreadable" {
		t.Fatalf("Title = %q (want unreadable)", it.Title)
	}
}

func TestPlanner_analyzeEntry_NormalizesSourceDirSegments(t *testing.T) {
	// "My Guides/Intro.md" -> "my-guides/intro" (SlugService.NormalizePath + NormalizeFilename)
	tmp := t.TempDir()
	writeFile(t, tmp, "My Guides/Intro.md", "# Intro")

	wiki := &fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "My Guides/Intro.md"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("Errors = %#v", res.Errors)
	}
	if res.Items[0].TargetPath != "docs/my-guides/intro" {
		t.Fatalf("TargetPath = %q (want docs/my-guides/intro)", res.Items[0].TargetPath)
	}
}

func TestPlanner_analyzeEntry_InvalidSourceDirSegment_ReturnsError(t *testing.T) {
	// NormalizePath(validate=true) nutzt IsValidSlug() nach slug.Make().
	// Ein Segment wie "!!!" sluggt zu "" => invalid.
	tmp := t.TempDir()
	writeFile(t, tmp, "!!!/a.md", "# A")

	wiki := &fakeWiki{treeHash: "h", lookups: map[string]*tree.PathLookup{}}
	p := newPlannerWithFake(wiki)

	res, err := p.CreatePlan([]ImportMDFile{{SourcePath: "!!!/a.md"}}, PlanOptions{
		SourceBasePath: tmp,
		TargetBasePath: "docs",
	})
	if err != nil {
		t.Fatalf("CreatePlan err: %v", err)
	}
	if len(res.Items) != 0 {
		t.Fatalf("Items len = %d (want 0)", len(res.Items))
	}
	if len(res.Errors) != 1 {
		t.Fatalf("Errors len = %d (want 1)", len(res.Errors))
	}
	// optional: grobe Assertion, dass es ein Validate-Fehler ist
	if res.Errors[0] == "" {
		t.Fatalf("unexpected error: %v", res.Errors[0])
	}
}
