package pages_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki/pages"
)

// readZipEntries unzips data into a name→content map for assertions.
func readZipEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	entries := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open zip entry %q: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("failed to read zip entry %q: %v", f.Name, err)
		}
		entries[f.Name] = string(content)
	}
	return entries
}

func TestDownloadPageUseCase_Page_ReturnsCleanMarkdown(t *testing.T) {
	deps := newTestDeps(t)
	metrics := httpmetrics.NewHTTPMetrics()
	createUC := newDownloadTestCreatePageUseCase(deps, metrics)
	updateUC := newDownloadTestUpdatePageUseCase(deps, metrics)
	downloadUC := pages.NewDownloadPageUseCase(deps.tree, deps.assets)

	created, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Getting Started", Slug: "getting-started", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating page: %v", err)
	}

	content := "# Getting Started\n\nHello world\n"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "user1", ID: created.Page.ID, Version: created.Page.Version(),
		Title: "Getting Started", Slug: "getting-started", Content: &content, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error updating page: %v", err)
	}

	out, err := downloadUC.Execute(context.Background(), pages.DownloadPageInput{ID: created.Page.ID})
	if err != nil {
		t.Fatalf("unexpected error downloading page: %v", err)
	}

	if out.Kind != tree.NodeKindPage {
		t.Errorf("expected kind %q, got %q", tree.NodeKindPage, out.Kind)
	}
	if out.Filename != "getting-started.md" {
		t.Errorf("expected filename %q, got %q", "getting-started.md", out.Filename)
	}
	if out.ContentType != "text/markdown; charset=utf-8" {
		t.Errorf("unexpected content type %q", out.ContentType)
	}
	if string(out.Data) != content {
		t.Errorf("expected data %q, got %q", content, string(out.Data))
	}
}

func TestDownloadPageUseCase_PageWithAssets_ReturnsPortableZip(t *testing.T) {
	deps := newTestDeps(t)
	metrics := httpmetrics.NewHTTPMetrics()
	createUC := newDownloadTestCreatePageUseCase(deps, metrics)
	updateUC := newDownloadTestUpdatePageUseCase(deps, metrics)
	downloadUC := pages.NewDownloadPageUseCase(deps.tree, deps.assets)

	created, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Getting Started", Slug: "getting-started", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating page: %v", err)
	}
	assetURL := saveTestAsset(t, deps, created.Page.PageNode, "logo.png", []byte("image bytes"))

	content := "# Getting Started\n\n![Logo](" + assetURL + ")\n"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "user1", ID: created.Page.ID, Version: created.Page.Version(),
		Title: "Getting Started", Slug: "getting-started", Content: &content, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error updating page: %v", err)
	}

	out, err := downloadUC.Execute(context.Background(), pages.DownloadPageInput{ID: created.Page.ID})
	if err != nil {
		t.Fatalf("unexpected error downloading page: %v", err)
	}

	if out.Filename != "getting-started.zip" {
		t.Errorf("expected filename %q, got %q", "getting-started.zip", out.Filename)
	}
	if out.ContentType != "application/zip" {
		t.Errorf("unexpected content type %q", out.ContentType)
	}

	entries := readZipEntries(t, out.Data)
	expectedMarkdown := "# Getting Started\n\n![Logo](getting-started_assets/logo.png)\n"
	if got := entries["getting-started.md"]; got != expectedMarkdown {
		t.Errorf("expected getting-started.md = %q, got %q", expectedMarkdown, got)
	}
	if got := entries["getting-started_assets/logo.png"]; got != "image bytes" {
		t.Errorf("expected image bytes in zip, got %q", got)
	}
}

func TestDownloadPageUseCase_Section_ZipsWholeSubtree(t *testing.T) {
	deps := newTestDeps(t)
	metrics := httpmetrics.NewHTTPMetrics()
	createUC := newDownloadTestCreatePageUseCase(deps, metrics)
	updateUC := newDownloadTestUpdatePageUseCase(deps, metrics)
	downloadUC := pages.NewDownloadPageUseCase(deps.tree, deps.assets)

	// docs/ (section)
	//   docs/intro (page)
	//   docs/guides/ (section)
	//     docs/guides/reference (page)
	section, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Docs", Slug: "docs", Kind: sectionKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating section: %v", err)
	}
	sectionID := section.Page.ID

	intro, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &sectionID, Title: "Intro", Slug: "intro", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating intro page: %v", err)
	}
	introContent := "# Intro\n\nIntro body\n"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "user1", ID: intro.Page.ID, Version: intro.Page.Version(),
		Title: "Intro", Slug: "intro", Content: &introContent, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error updating intro page: %v", err)
	}

	guidesSection, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &sectionID, Title: "Guides", Slug: "guides", Kind: sectionKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating guides section: %v", err)
	}
	guidesSectionID := guidesSection.Page.ID

	reference, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &guidesSectionID, Title: "Reference", Slug: "reference", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating reference page: %v", err)
	}
	refContent := "# Reference\n\nRef body\n"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "user1", ID: reference.Page.ID, Version: reference.Page.Version(),
		Title: "Reference", Slug: "reference", Content: &refContent, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error updating reference page: %v", err)
	}

	out, err := downloadUC.Execute(context.Background(), pages.DownloadPageInput{ID: sectionID})
	if err != nil {
		t.Fatalf("unexpected error downloading section: %v", err)
	}

	if out.Kind != tree.NodeKindSection {
		t.Errorf("expected kind %q, got %q", tree.NodeKindSection, out.Kind)
	}
	if out.Filename != "docs.zip" {
		t.Errorf("expected filename %q, got %q", "docs.zip", out.Filename)
	}
	if out.ContentType != "application/zip" {
		t.Errorf("unexpected content type %q", out.ContentType)
	}

	entries := readZipEntries(t, out.Data)

	if _, ok := entries["docs/index.md"]; !ok {
		t.Errorf("expected zip to contain docs/index.md, got entries: %v", keys(entries))
	}
	if got := entries["docs/intro.md"]; got != introContent {
		t.Errorf("expected docs/intro.md = %q, got %q", introContent, got)
	}
	if _, ok := entries["docs/guides/index.md"]; !ok {
		t.Errorf("expected zip to contain docs/guides/index.md, got entries: %v", keys(entries))
	}
	if got := entries["docs/guides/reference.md"]; got != refContent {
		t.Errorf("expected docs/guides/reference.md = %q, got %q", refContent, got)
	}
}

func TestDownloadPageUseCase_SectionWithAssets_RewritesMarkdownLinks(t *testing.T) {
	deps := newTestDeps(t)
	metrics := httpmetrics.NewHTTPMetrics()
	createUC := newDownloadTestCreatePageUseCase(deps, metrics)
	updateUC := newDownloadTestUpdatePageUseCase(deps, metrics)
	downloadUC := pages.NewDownloadPageUseCase(deps.tree, deps.assets)

	section, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Docs", Slug: "docs", Kind: sectionKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating section: %v", err)
	}
	sectionID := section.Page.ID

	intro, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &sectionID, Title: "Intro", Slug: "intro", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating intro page: %v", err)
	}
	assetURL := saveTestAsset(t, deps, intro.Page.PageNode, "diagram.png", []byte("diagram bytes"))

	introContent := "# Intro\n\n![Diagram](" + assetURL + ")\n"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "user1", ID: intro.Page.ID, Version: intro.Page.Version(),
		Title: "Intro", Slug: "intro", Content: &introContent, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error updating intro page: %v", err)
	}

	out, err := downloadUC.Execute(context.Background(), pages.DownloadPageInput{ID: sectionID})
	if err != nil {
		t.Fatalf("unexpected error downloading section: %v", err)
	}

	entries := readZipEntries(t, out.Data)
	expectedMarkdown := "# Intro\n\n![Diagram](intro_assets/diagram.png)\n"
	if got := entries["docs/intro.md"]; got != expectedMarkdown {
		t.Errorf("expected docs/intro.md = %q, got %q", expectedMarkdown, got)
	}
	if got := entries["docs/intro_assets/diagram.png"]; got != "diagram bytes" {
		t.Errorf("expected diagram bytes in zip, got %q", got)
	}
}

func TestDownloadPageUseCase_NotFound_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	downloadUC := pages.NewDownloadPageUseCase(deps.tree, deps.assets)

	if _, err := downloadUC.Execute(context.Background(), pages.DownloadPageInput{ID: "nonexistent"}); err == nil {
		t.Fatal("expected error for non-existent node, got nil")
	}
}

func saveTestAsset(t *testing.T, deps *testDeps, page *tree.PageNode, name string, content []byte) string {
	t.Helper()
	file, filename, err := test_utils.CreateMultipartFile(name, content)
	if err != nil {
		t.Fatalf("failed to create test asset: %v", err)
	}
	defer file.Close()

	url, err := deps.assets.SaveAssetForPage(page, file, filename, 1024)
	if err != nil {
		t.Fatalf("failed to save test asset: %v", err)
	}
	return url
}

func newDownloadTestCreatePageUseCase(deps *testDeps, metrics *httpmetrics.HTTPMetrics) *pages.CreatePageUseCase {
	return pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.orchestrator(), slog.Default(), metrics)
}

func newDownloadTestUpdatePageUseCase(deps *testDeps, metrics *httpmetrics.HTTPMetrics) *pages.UpdatePageUseCase {
	return pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.orchestrator(), slog.Default(), metrics)
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
