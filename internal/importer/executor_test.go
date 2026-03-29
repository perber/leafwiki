package importer

import (
	"errors"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
)

type fakeExecWiki struct {
	hash string

	ensureCalls int
	updateCalls int

	ensureFn func(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error)
	updateFn func(userID, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error)

	lastUpdatedContent *string
	ensureTargets      []string
	ensureKinds        []tree.NodeKind
	updateTitles       []string
	uploadCalls        int
	uploadedAssets     []string
	lastUploadMaxBytes int64
}

func (f *fakeExecWiki) TreeHash() string { return f.hash }

func (f *fakeExecWiki) LookupPagePath(path string) (*tree.PathLookup, error) {
	panic("not used by Executor")
}

func (f *fakeExecWiki) EnsurePath(userID string, targetPath string, title string, kind *tree.NodeKind) (*tree.Page, error) {
	f.ensureCalls++
	f.ensureTargets = append(f.ensureTargets, targetPath)
	if kind != nil {
		f.ensureKinds = append(f.ensureKinds, *kind)
	}
	if f.ensureFn != nil {
		return f.ensureFn(userID, targetPath, title, kind)
	}
	return &tree.Page{PageNode: &tree.PageNode{ID: "p1", Title: title, Slug: "slug", Kind: *kind}}, nil
}

func (f *fakeExecWiki) UpdatePage(userID string, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error) {
	f.updateCalls++
	f.lastUpdatedContent = content
	f.updateTitles = append(f.updateTitles, title)
	if f.updateFn != nil {
		return f.updateFn(userID, id, title, slug, content, kind)
	}
	// simulate tree change
	f.hash = f.hash + "-changed"
	return &tree.Page{PageNode: &tree.PageNode{ID: id, Title: title, Slug: slug, Kind: *kind}}, nil
}

func (f *fakeExecWiki) UploadAsset(userID, pageID string, file multipart.File, filename string, maxBytes int64) (string, error) {
	f.uploadCalls++
	f.uploadedAssets = append(f.uploadedAssets, filename)
	f.lastUploadMaxBytes = maxBytes
	return "/assets/" + pageID + "/" + filename, nil
}

func writeTmp(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestExecutor_StalePlan(t *testing.T) {
	w := &fakeExecWiki{hash: "new"}
	plan := &PlanResult{TreeHash: "old"}
	opts := &PlanOptions{SourceBasePath: t.TempDir()}
	ex := NewExecutor(plan, opts, 0, w, slog.Default())

	got, err := ex.Execute("user1")
	if err == nil {
		t.Fatalf("expected stale plan error")
	}
	if got != nil {
		t.Fatalf("expected nil result on stale plan, got %#v", got)
	}
}

func TestExecutor_Create_HappyPath_PreservesNonInternalFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "a.md", "---\naliases:\n  - x\ncustom_key: keep-me\nleafwiki_id: source-id\nleafwiki_title: Source Title\ntitle: X\n---\n\n# Heading\nBody")

	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Title: "A", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())

	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	if res.ImportedCount != 1 || res.SkippedCount != 0 {
		t.Fatalf("counts imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}
	if len(res.Items) != 1 || res.Items[0].Action != ExecutionActionCreated {
		t.Fatalf("item result: %#v", res.Items)
	}
	if w.ensureCalls != 1 || w.updateCalls != 1 {
		t.Fatalf("calls ensure=%d update=%d", w.ensureCalls, w.updateCalls)
	}

	if w.lastUpdatedContent == nil {
		t.Fatalf("expected content to be passed to UpdatePage")
	}
	fm, body, has, err := markdown.ParseFrontmatter(*w.lastUpdatedContent)
	if err != nil {
		t.Fatalf("ParseFrontmatter err: %v", err)
	}
	if !has {
		t.Fatalf("expected preserved frontmatter, got: %q", *w.lastUpdatedContent)
	}
	if body != "\n# Heading\nBody" {
		t.Fatalf("unexpected body: %q", body)
	}
	if got := fm.ExtraFields["custom_key"]; got != "keep-me" {
		t.Fatalf("expected custom_key to be preserved, got %#v", got)
	}
	if got := fm.ExtraFields["title"]; got != "X" {
		t.Fatalf("expected title extra field to be preserved, got %#v", got)
	}
	aliases, ok := fm.ExtraFields["aliases"].([]interface{})
	if !ok || len(aliases) != 1 || aliases[0] != "x" {
		t.Fatalf("expected aliases to be preserved, got %#v", fm.ExtraFields["aliases"])
	}
	if strings.Contains(*w.lastUpdatedContent, "leafwiki_id: source-id") {
		t.Fatalf("expected source leafwiki_id to be dropped, got: %q", *w.lastUpdatedContent)
	}
	if strings.Contains(*w.lastUpdatedContent, "leafwiki_title: Source Title") {
		t.Fatalf("expected source leafwiki_title to be dropped, got: %q", *w.lastUpdatedContent)
	}

	if res.TreeHashBefore != "h1" {
		t.Fatalf("TreeHashBefore = %q", res.TreeHashBefore)
	}
	if res.TreeHash == "h1" {
		t.Fatalf("expected TreeHash to change (fake changes it), got %q", res.TreeHash)
	}
}

func TestExecutor_Create_HappyPath_PreservesDistinctExtraFieldValues(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "a.md", "---\nalpha: first\nbeta: second\nnested:\n  key: value\n---\n\nBody")

	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Title: "A", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	if _, err := ex.Execute("user1"); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	if w.lastUpdatedContent == nil {
		t.Fatalf("expected content to be passed to UpdatePage")
	}

	fm, _, has, err := markdown.ParseFrontmatter(*w.lastUpdatedContent)
	if err != nil {
		t.Fatalf("ParseFrontmatter err: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter, got %q", *w.lastUpdatedContent)
	}
	if got := fm.ExtraFields["alpha"]; got != "first" {
		t.Fatalf("expected alpha=first, got %#v", got)
	}
	if got := fm.ExtraFields["beta"]; got != "second" {
		t.Fatalf("expected beta=second, got %#v", got)
	}
	nested, ok := fm.ExtraFields["nested"].(map[string]interface{})
	if !ok || nested["key"] != "value" {
		t.Fatalf("expected nested map to be preserved, got %#v", fm.ExtraFields["nested"])
	}
}

func TestExecutor_Skip_DoesNotCallWiki(t *testing.T) {
	tmp := t.TempDir()
	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Action: PlanActionSkip},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	if res.SkippedCount != 1 || res.ImportedCount != 0 {
		t.Fatalf("counts imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}
	if w.ensureCalls != 0 || w.updateCalls != 0 {
		t.Fatalf("expected no wiki calls, got ensure=%d update=%d", w.ensureCalls, w.updateCalls)
	}
}

func TestExecutor_Create_EnsurePathError_SkipsItem(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "a.md", "Body")

	w := &fakeExecWiki{
		hash: "h1",
		ensureFn: func(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error) {
			return nil, errors.New("boom")
		},
	}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Title: "A", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if res.SkippedCount != 1 || res.ImportedCount != 0 {
		t.Fatalf("counts imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}
	if res.Items[0].Error == nil || *res.Items[0].Error == "" {
		t.Fatalf("expected error message")
	}
	if w.updateCalls != 0 {
		t.Fatalf("UpdatePage should not be called")
	}
}

func TestExecutor_UnknownAction_SkipsItem(t *testing.T) {
	tmp := t.TempDir()
	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Action: PlanActionUpdate}, // not handled in switch
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if res.SkippedCount != 1 {
		t.Fatalf("SkippedCount=%d", res.SkippedCount)
	}
	if res.Items[0].Error == nil || *res.Items[0].Error != "unknown action" {
		t.Fatalf("Error=%#v", res.Items[0].Error)
	}
}

func TestExecutor_Create_FolderIndexAndSiblingPage_ImportsSectionThenNestedPage(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Ordner/index.md", `---
title: Ordner
---

# Ordner`)
	writeTmp(t, tmp, "Ordner/Ordner.md", "# Unterseite")

	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "Ordner/index.md", TargetPath: "ordner", Title: "Ordner", Kind: tree.NodeKindSection, Action: PlanActionCreate},
			{SourcePath: "Ordner/Ordner.md", TargetPath: "ordner/ordner", Title: "Unterseite", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if res.ImportedCount != 2 || res.SkippedCount != 0 {
		t.Fatalf("counts imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}
	if w.ensureCalls != 2 || w.updateCalls != 2 {
		t.Fatalf("calls ensure=%d update=%d", w.ensureCalls, w.updateCalls)
	}
	if len(w.ensureTargets) != 2 || w.ensureTargets[0] != "ordner" || w.ensureTargets[1] != "ordner/ordner" {
		t.Fatalf("unexpected ensure targets: %#v", w.ensureTargets)
	}
	if len(w.ensureKinds) != 2 || w.ensureKinds[0] != tree.NodeKindSection || w.ensureKinds[1] != tree.NodeKindPage {
		t.Fatalf("unexpected ensure kinds: %#v", w.ensureKinds)
	}
	if len(w.updateTitles) != 2 || w.updateTitles[0] != "Ordner" || w.updateTitles[1] != "Unterseite" {
		t.Fatalf("unexpected update titles: %#v", w.updateTitles)
	}
}

func TestExecutor_Create_RewritesMarkdownAndWikiLinksToImportedPages(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Guides/index.md", "# Guides")
	writeTmp(t, tmp, "Guides/Setup.md", strings.Join([]string{
		"# Setup",
		"",
		"[Relative](../Reference/Endpoints.md)",
		"[Absolute](/Guides/index.md)",
		"[RouteStyle](/Reference/Endpoints)",
		"[Container](/Guides/)",
		"[[Reference/Endpoints|API Alias]]",
	}, "\n"))
	writeTmp(t, tmp, "Reference/Endpoints.md", "# Endpoints")

	w := &fakeExecWiki{hash: "h1"}
	updatedContentByTitle := map[string]string{}
	w.updateFn = func(userID, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error) {
		w.lastUpdatedContent = content
		w.updateTitles = append(w.updateTitles, title)
		if content != nil {
			updatedContentByTitle[title] = *content
		}
		w.hash = w.hash + "-changed"
		return &tree.Page{PageNode: &tree.PageNode{ID: id, Title: title, Slug: slug, Kind: *kind}}, nil
	}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "Guides/index.md", TargetPath: "guides", Title: "Guides", Kind: tree.NodeKindSection, Action: PlanActionCreate},
			{SourcePath: "Guides/Setup.md", TargetPath: "guides/setup", Title: "Setup", Kind: tree.NodeKindPage, Action: PlanActionCreate},
			{SourcePath: "Reference/Endpoints.md", TargetPath: "reference/endpoints", Title: "Endpoints", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	if _, err := ex.Execute("user1"); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	setupContent, ok := updatedContentByTitle["Setup"]
	if !ok {
		t.Fatalf("expected setup content to be updated")
	}

	for _, expected := range []string{
		"[Relative](/reference/endpoints)",
		"[Absolute](/guides)",
		"[RouteStyle](/reference/endpoints)",
		"[Container](/guides)",
		"[API Alias](/reference/endpoints)",
	} {
		if !strings.Contains(setupContent, expected) {
			t.Fatalf("expected rewritten content to contain %q, got:\n%s", expected, setupContent)
		}
	}
}

func TestExecutor_Create_UploadsRelativeAndRootAssets(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Guides/Setup.md", strings.Join([]string{
		"# Setup",
		"",
		"![Relative](./images/logo.png)",
		"[Asset](/shared/manual.pdf)",
		"![[./images/logo.png]]",
	}, "\n"))
	writeTmp(t, tmp, "Guides/images/logo.png", "png-bytes")
	writeTmp(t, tmp, "shared/manual.pdf", "pdf-bytes")

	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "Guides/Setup.md", TargetPath: "guides/setup", Title: "Setup", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 1234, w, slog.Default())
	if _, err := ex.Execute("user1"); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	if w.uploadCalls != 2 {
		t.Fatalf("expected 2 asset uploads, got %d", w.uploadCalls)
	}
	if w.lastUpdatedContent == nil {
		t.Fatalf("expected content to be updated")
	}
	if !strings.Contains(*w.lastUpdatedContent, "![Relative](/assets/p1/logo.png)") {
		t.Fatalf("expected relative asset link rewrite, got:\n%s", *w.lastUpdatedContent)
	}
	if !strings.Contains(*w.lastUpdatedContent, "[Asset](/assets/p1/manual.pdf)") {
		t.Fatalf("expected root asset link rewrite, got:\n%s", *w.lastUpdatedContent)
	}
	if !strings.Contains(*w.lastUpdatedContent, "![logo.png](/assets/p1/logo.png)") {
		t.Fatalf("expected wiki asset link rewrite, got:\n%s", *w.lastUpdatedContent)
	}
	if w.lastUploadMaxBytes != 1234 {
		t.Fatalf("expected asset uploads to use configured max bytes, got %d", w.lastUploadMaxBytes)
	}
}

func TestExecutor_Create_WikiLinkToNonImageAssetStaysNormalLink(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Guides/Setup.md", strings.Join([]string{
		"# Setup",
		"",
		"[[../shared/manual.pdf]]",
		"![[../shared/manual.pdf]]",
	}, "\n"))
	writeTmp(t, tmp, "shared/manual.pdf", "pdf-bytes")

	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "Guides/Setup.md", TargetPath: "guides/setup", Title: "Setup", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	if _, err := ex.Execute("user1"); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	if w.lastUpdatedContent == nil {
		t.Fatalf("expected content to be updated")
	}
	if !strings.Contains(*w.lastUpdatedContent, "[manual.pdf](/assets/p1/manual.pdf)") {
		t.Fatalf("expected non-embed wiki asset to stay a normal link, got:\n%s", *w.lastUpdatedContent)
	}
	if !strings.Contains(*w.lastUpdatedContent, "![manual.pdf](/assets/p1/manual.pdf)") {
		t.Fatalf("expected embed wiki asset to use embed syntax, got:\n%s", *w.lastUpdatedContent)
	}
}

func TestExecutor_Create_WikiLinkFallsBackToUniqueNestedBasenameOnly(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Home.md", strings.Join([]string{
		"# Home",
		"",
		"[[Brainstorm]]",
		"[[Meeting Notes]]",
	}, "\n"))
	writeTmp(t, tmp, "Daily/Brainstorm.md", "# Brainstorm")
	writeTmp(t, tmp, "Daily/Meeting Notes.md", "# Daily Meeting Notes")
	writeTmp(t, tmp, "Archive/Meeting Notes.md", "# Archived Meeting Notes")

	w := &fakeExecWiki{hash: "h1"}
	updatedContentByTitle := map[string]string{}
	w.updateFn = func(userID, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error) {
		w.lastUpdatedContent = content
		if content != nil {
			updatedContentByTitle[title] = *content
		}
		w.hash = w.hash + "-changed"
		return &tree.Page{PageNode: &tree.PageNode{ID: id, Title: title, Slug: slug, Kind: *kind}}, nil
	}

	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "Home.md", TargetPath: "home", Title: "Home", Kind: tree.NodeKindPage, Action: PlanActionCreate},
			{SourcePath: "Daily/Brainstorm.md", TargetPath: "daily/brainstorm", Title: "Brainstorm", Kind: tree.NodeKindPage, Action: PlanActionCreate},
			{SourcePath: "Daily/Meeting Notes.md", TargetPath: "daily/meeting-notes", Title: "Meeting Notes", Kind: tree.NodeKindPage, Action: PlanActionCreate},
			{SourcePath: "Archive/Meeting Notes.md", TargetPath: "archive/meeting-notes", Title: "Meeting Notes", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	if _, err := ex.Execute("user1"); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	homeContent := updatedContentByTitle["Home"]
	if !strings.Contains(homeContent, "[Brainstorm](/daily/brainstorm)") {
		t.Fatalf("expected unique basename wiki link rewrite, got:\n%s", homeContent)
	}
	if !strings.Contains(homeContent, "[[Meeting Notes]]") {
		t.Fatalf("expected ambiguous basename wiki link to stay unchanged, got:\n%s", homeContent)
	}
}

func TestExecutor_Create_DoesNotRewriteLinksInsideCode(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Guides/Setup.md", strings.Join([]string{
		"# Setup",
		"",
		"`[Inline](../Reference/Endpoints.md)`",
		"",
		"```md",
		"[Fence](../Reference/Endpoints.md)",
		"[[Reference/Endpoints|Fence Alias]]",
		"```",
		"",
		"[Real](../Reference/Endpoints.md)",
		"[[Reference/Endpoints|Real Alias]]",
	}, "\n"))
	writeTmp(t, tmp, "Reference/Endpoints.md", "# Endpoints")

	w := &fakeExecWiki{hash: "h1"}
	updatedContentByTitle := map[string]string{}
	w.updateFn = func(userID, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error) {
		w.lastUpdatedContent = content
		if content != nil {
			updatedContentByTitle[title] = *content
		}
		w.hash = w.hash + "-changed"
		return &tree.Page{PageNode: &tree.PageNode{ID: id, Title: title, Slug: slug, Kind: *kind}}, nil
	}

	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "Guides/Setup.md", TargetPath: "guides/setup", Title: "Setup", Kind: tree.NodeKindPage, Action: PlanActionCreate},
			{SourcePath: "Reference/Endpoints.md", TargetPath: "reference/endpoints", Title: "Endpoints", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	if _, err := ex.Execute("user1"); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	setupContent := updatedContentByTitle["Setup"]
	for _, expected := range []string{
		"`[Inline](../Reference/Endpoints.md)`",
		"[Fence](../Reference/Endpoints.md)",
		"[[Reference/Endpoints|Fence Alias]]",
		"[Real](/reference/endpoints)",
		"[Real Alias](/reference/endpoints)",
	} {
		if !strings.Contains(setupContent, expected) {
			t.Fatalf("expected content to contain %q, got:\n%s", expected, setupContent)
		}
	}
}

func TestExecutor_Create_RewritesWindowsStyleMarkdownAndAssetPaths(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Guides/Setup.md", strings.Join([]string{
		"# Setup",
		"",
		"[Doc](..\\Reference\\Endpoints.md)",
		"![Diagram](images\\diagram.png)",
	}, "\n"))
	writeTmp(t, tmp, "Reference/Endpoints.md", "# Endpoints")
	writeTmp(t, tmp, "Guides/images/diagram.png", "png-bytes")

	w := &fakeExecWiki{hash: "h1"}
	updatedContentByTitle := map[string]string{}
	w.updateFn = func(userID, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error) {
		w.lastUpdatedContent = content
		if content != nil {
			updatedContentByTitle[title] = *content
		}
		w.hash = w.hash + "-changed"
		return &tree.Page{PageNode: &tree.PageNode{ID: id, Title: title, Slug: slug, Kind: *kind}}, nil
	}

	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "Guides/Setup.md", TargetPath: "guides/setup", Title: "Setup", Kind: tree.NodeKindPage, Action: PlanActionCreate},
			{SourcePath: "Reference/Endpoints.md", TargetPath: "reference/endpoints", Title: "Endpoints", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	if _, err := ex.Execute("user1"); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	setupContent := updatedContentByTitle["Setup"]
	for _, expected := range []string{
		"[Doc](/reference/endpoints)",
		"![Diagram](/assets/p1/diagram.png)",
	} {
		if !strings.Contains(setupContent, expected) {
			t.Fatalf("expected content to contain %q, got:\n%s", expected, setupContent)
		}
	}
}

func TestExecutor_Create_LeavesWindowsDriveLetterPathsUntouched(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "Guides/Setup.md", strings.Join([]string{
		"# Setup",
		"",
		"[Windows File](C:\\Users\\John\\Notes\\Endpoints.md)",
		"![Windows Image](C:\\Users\\John\\Images\\diagram.png)",
	}, "\n"))

	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "Guides/Setup.md", TargetPath: "guides/setup", Title: "Setup", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, 0, w, slog.Default())
	if _, err := ex.Execute("user1"); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	if w.lastUpdatedContent == nil {
		t.Fatalf("expected updated content")
	}
	for _, expected := range []string{
		"[Windows File](C:\\Users\\John\\Notes\\Endpoints.md)",
		"![Windows Image](C:\\Users\\John\\Images\\diagram.png)",
	} {
		if !strings.Contains(*w.lastUpdatedContent, expected) {
			t.Fatalf("expected content to keep %q untouched, got:\n%s", expected, *w.lastUpdatedContent)
		}
	}
}
