package importer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/importer"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
)

func integMustWrite(t *testing.T, base, rel, content string) string {
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

func integFixturePath(t *testing.T, rel string) string {
	t.Helper()
	return test_utils.FixturePath(t, rel, "fixtures", "internal/importer/fixtures")
}

func integCopyFixtureToTemp(t *testing.T, rel string) string {
	t.Helper()
	sourceRoot := integFixturePath(t, rel)
	destRoot := filepath.Join(t.TempDir(), rel)

	err := filepath.Walk(sourceRoot, func(sourcePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return os.MkdirAll(destRoot, 0o755)
		}
		destPath := filepath.Join(destRoot, relativePath)
		if info.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		raw, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, raw, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture %q: %v", rel, err)
	}
	return destRoot
}

func newTestWiki(t *testing.T) *wiki.Wiki {
	t.Helper()
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWiki err: %v", err)
	}
	return w
}

func newTestImporterService(t *testing.T, w *wiki.Wiki) *importer.ImporterService {
	t.Helper()
	planner := importer.NewPlanner(wiki.NewWikiImportAdapter(w), tree.NewSlugService())
	importerDir := filepath.Join(w.GetStorageDir(), ".importer")
	return importer.NewImporterService(
		planner,
		importer.NewPlanStore(filepath.Join(importerDir, "current-plan.json")),
		filepath.Join(importerDir, "workspaces"),
		0,
	)
}

func newImporterProbe(w *wiki.Wiki) *wiki.WikiImportAdapter {
	return wiki.NewWikiImportAdapter(w)
}

func TestImporterService_ExecuteCurrentPlan_WritesPreservedFrontmatterToDisk(t *testing.T) {
	ws := t.TempDir()
	integMustWrite(t, ws, "Imported.md", "---\naliases:\n  - alpha\ncustom_key: keep-me\nleafwiki_id: source-id\ntitle: Imported Title\n---\n\n# Imported Title\nBody")

	w := newTestWiki(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	is := newTestImporterService(t, w)

	plan, err := is.CreateImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("expected one plan item, got %#v", plan.Items)
	}

	res, err := is.ExecuteCurrentPlan("system")
	if err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}
	if res.ImportedCount != 1 || res.SkippedCount != 0 {
		t.Fatalf("unexpected result: imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}

	rawBytes, err := os.ReadFile(filepath.Join(w.GetStorageDir(), "root", "imported.md"))
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}
	raw := string(rawBytes)

	fm, body, has, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter err: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter in written file, got: %q", raw)
	}
	if body != "\n# Imported Title\nBody" {
		t.Fatalf("unexpected body: %q", body)
	}
	if got := fm.ExtraFields["custom_key"]; got != "keep-me" {
		t.Fatalf("expected custom_key to be preserved, got %#v", got)
	}
	if got := fm.ExtraFields["title"]; got != "Imported Title" {
		t.Fatalf("expected title to be preserved, got %#v", got)
	}
	aliases, ok := fm.ExtraFields["aliases"].([]interface{})
	if !ok || len(aliases) != 1 || aliases[0] != "alpha" {
		t.Fatalf("expected aliases to be preserved, got %#v", fm.ExtraFields["aliases"])
	}
	if strings.Contains(raw, "leafwiki_id: source-id") {
		t.Fatalf("expected source leafwiki_id to be dropped, got: %q", raw)
	}
	if fm.LeafWikiID == "" {
		t.Fatalf("expected written file to contain generated leafwiki_id")
	}
	if fm.LeafWikiTitle != "Imported Title" {
		t.Fatalf("expected written file to contain effective leafwiki_title, got %q", fm.LeafWikiTitle)
	}
}

func TestImporterService_ExecuteCurrentPlan_RewritesLinksAndUploadsAssetsToDisk(t *testing.T) {
	ws := t.TempDir()
	integMustWrite(t, ws, "Guides/index.md", "# Guides")
	integMustWrite(t, ws, "Guides/Setup.md", strings.Join([]string{
		"# Setup",
		"",
		"[Guide Home](/Guides/)",
		"[API](../Reference/Endpoints.md#intro)",
		"![[./images/logo.png]]",
		"[Manual](/shared/manual.pdf)",
		"[[Reference/Endpoints|API Alias]]",
	}, "\n"))
	integMustWrite(t, ws, "Reference/Endpoints.md", "# Endpoints")
	integMustWrite(t, ws, "Guides/images/logo.png", "png-bytes")
	integMustWrite(t, ws, "shared/manual.pdf", "pdf-bytes")

	w := newTestWiki(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	is := newTestImporterService(t, w)
	probe := newImporterProbe(w)

	plan, err := is.CreateImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 3 {
		t.Fatalf("expected three plan items, got %#v", plan.Items)
	}

	if _, err := is.ExecuteCurrentPlan("system"); err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	setupPage, err := probe.FindByPath("guides/setup")
	if err != nil {
		t.Fatalf("FindByPath err: %v", err)
	}

	for _, expected := range []string{
		"[Guide Home](/guides)",
		"[API](/reference/endpoints#intro)",
		"[API Alias](/reference/endpoints)",
		"/assets/" + setupPage.ID + "/logo.png",
		"/assets/" + setupPage.ID + "/manual.pdf",
	} {
		if !strings.Contains(setupPage.Content, expected) {
			t.Fatalf("expected content to contain %q, got:\n%s", expected, setupPage.Content)
		}
	}

	assets, err := probe.ListAssets(setupPage.ID)
	if err != nil {
		t.Fatalf("ListAssets err: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 uploaded assets, got %#v", assets)
	}
}

func TestImporterService_ExecuteCurrentPlan_ImportsFixturePackage(t *testing.T) {
	ws := integCopyFixtureToTemp(t, "link-assets-package")

	w := newTestWiki(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	is := newTestImporterService(t, w)
	probe := newImporterProbe(w)

	plan, err := is.CreateImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 5 {
		t.Fatalf("expected five plan items, got %#v", plan.Items)
	}

	if _, err := is.ExecuteCurrentPlan("system"); err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	setupPage, err := probe.FindByPath("guides/setup")
	if err != nil {
		t.Fatalf("FindByPath guides/setup err: %v", err)
	}

	for _, expected := range []string{
		"[Relative MD](/reference/endpoints)",
		"[Absolute MD](/reference/endpoints)",
		"[Container](/guides)",
		"[Endpoints](/reference/endpoints)",
		"[API Alias](/reference/endpoints)",
		"![Relative Image](/assets/" + setupPage.ID + "/logo.png)",
		"[Manual](/assets/" + setupPage.ID + "/manual.pdf)",
		"![logo.png](/assets/" + setupPage.ID + "/logo.png)",
		"`[Inline](../Reference/Endpoints.md)`",
		"`[[Reference/Endpoints|Inline Alias]]`",
		"[Fenced](../Reference/Endpoints.md)",
		"[[Reference/Endpoints|Fence Alias]]",
		"![[./images/logo.png]]",
	} {
		if !strings.Contains(setupPage.Content, expected) {
			t.Fatalf("expected setup content to contain %q, got:\n%s", expected, setupPage.Content)
		}
	}

	assets, err := probe.ListAssets(setupPage.ID)
	if err != nil {
		t.Fatalf("ListAssets err: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 uploaded assets, got %#v", assets)
	}

	if _, err := probe.FindByPath("reference/endpoints"); err != nil {
		t.Fatalf("FindByPath reference/endpoints err: %v", err)
	}
	if _, err := probe.FindByPath("reference/api-1"); err != nil {
		t.Fatalf("FindByPath reference/api-1 err: %v", err)
	}
	if _, err := probe.FindByPath("guides"); err != nil {
		t.Fatalf("FindByPath guides err: %v", err)
	}
	if _, err := probe.FindByPath("readme"); err != nil {
		t.Fatalf("FindByPath readme err: %v", err)
	}
}

func TestImporterService_ExecuteCurrentPlan_ImportsLeafWikiNestedFixture(t *testing.T) {
	ws := integCopyFixtureToTemp(t, "leafwiki-nested-package")

	w := newTestWiki(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	is := newTestImporterService(t, w)
	probe := newImporterProbe(w)

	plan, err := is.CreateImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 5 {
		t.Fatalf("expected five plan items, got %#v", plan.Items)
	}

	if _, err := is.ExecuteCurrentPlan("system"); err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	introPage, err := probe.FindByPath("intro")
	if err != nil {
		t.Fatalf("FindByPath intro err: %v", err)
	}
	gettingStartedPage, err := probe.FindByPath("docs/getting-started")
	if err != nil {
		t.Fatalf("FindByPath docs/getting-started err: %v", err)
	}
	basicGuidePage, err := probe.FindByPath("docs/guides/basic-guide")
	if err != nil {
		t.Fatalf("FindByPath docs/guides/basic-guide err: %v", err)
	}
	if _, err := probe.FindByPath("docs"); err != nil {
		t.Fatalf("FindByPath docs err: %v", err)
	}
	if _, err := probe.FindByPath("docs/guides"); err != nil {
		t.Fatalf("FindByPath docs/guides err: %v", err)
	}

	for _, expected := range []string{
		"[Getting Started](/docs/getting-started)",
		"[Basic Guide](/docs/guides/basic-guide)",
	} {
		if !strings.Contains(introPage.Content, expected) {
			t.Fatalf("expected intro content to contain %q, got:\n%s", expected, introPage.Content)
		}
	}

	for _, expected := range []string{
		"[Intro](/intro)",
		"[Basic Guide](/docs/guides/basic-guide)",
	} {
		if !strings.Contains(gettingStartedPage.Content, expected) {
			t.Fatalf("expected getting-started content to contain %q, got:\n%s", expected, gettingStartedPage.Content)
		}
	}

	for _, expected := range []string{
		"[Introduction](/intro)",
		"[Documentation](/docs)",
	} {
		if !strings.Contains(basicGuidePage.Content, expected) {
			t.Fatalf("expected basic-guide content to contain %q, got:\n%s", expected, basicGuidePage.Content)
		}
	}

	rawIntroBytes, err := os.ReadFile(filepath.Join(w.GetStorageDir(), "root", "intro.md"))
	if err != nil {
		t.Fatalf("ReadFile intro err: %v", err)
	}
	rawIntro := string(rawIntroBytes)

	fm, body, has, err := markdown.ParseFrontmatter(rawIntro)
	if err != nil {
		t.Fatalf("ParseFrontmatter intro err: %v", err)
	}
	if !has {
		t.Fatalf("expected intro frontmatter, got %q", rawIntro)
	}
	if strings.Contains(rawIntro, "leafwiki_id: intro-source") {
		t.Fatalf("expected source leafwiki_id to be replaced, got: %q", rawIntro)
	}
	if fm.LeafWikiID == "" {
		t.Fatalf("expected regenerated leafwiki_id")
	}
	if fm.LeafWikiTitle != "Introduction" {
		t.Fatalf("expected leafwiki_title Introduction, got %q", fm.LeafWikiTitle)
	}
	if fm.LeafWikiCreatorID != "system" {
		t.Fatalf("expected creator to reflect imported page ownership, got %q", fm.LeafWikiCreatorID)
	}
	if fm.LeafWikiLastAuthorID != "system" {
		t.Fatalf("expected last author to reflect import execution user, got %q", fm.LeafWikiLastAuthorID)
	}
	if fm.LeafWikiCreatedAt == "" {
		t.Fatalf("expected created_at to be written")
	}
	if fm.LeafWikiUpdatedAt == "" {
		t.Fatalf("expected updated_at to be written")
	}
	if got := fm.ExtraFields["category"]; got != "onboarding" {
		t.Fatalf("expected category extra field preserved, got %#v", got)
	}
	aliases, ok := fm.ExtraFields["aliases"].([]interface{})
	if !ok || len(aliases) != 1 || aliases[0] != "start" {
		t.Fatalf("expected aliases to be preserved, got %#v", fm.ExtraFields["aliases"])
	}
	if !strings.Contains(body, "[Getting Started](/docs/getting-started)") {
		t.Fatalf("expected rewritten body in persisted intro file, got:\n%s", body)
	}
}

func TestImporterService_ExecuteCurrentPlan_ImportsObsidianWikiLinksFixture(t *testing.T) {
	ws := integCopyFixtureToTemp(t, "obsidian-wikilinks-package")

	w := newTestWiki(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	is := newTestImporterService(t, w)
	probe := newImporterProbe(w)

	plan, err := is.CreateImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 5 {
		t.Fatalf("expected five plan items, got %#v", plan.Items)
	}

	if _, err := is.ExecuteCurrentPlan("system"); err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	homePage, err := probe.FindByPath("home")
	if err != nil {
		t.Fatalf("FindByPath home err: %v", err)
	}
	projectPlanPage, err := probe.FindByPath("project-plan")
	if err != nil {
		t.Fatalf("FindByPath project-plan err: %v", err)
	}
	brainstormPage, err := probe.FindByPath("daily/brainstorm")
	if err != nil {
		t.Fatalf("FindByPath daily/brainstorm err: %v", err)
	}
	meetingNotesPage, err := probe.FindByPath("daily/meeting-notes")
	if err != nil {
		t.Fatalf("FindByPath daily/meeting-notes err: %v", err)
	}
	if _, err := probe.FindByPath("archive/meeting-notes"); err != nil {
		t.Fatalf("FindByPath archive/meeting-notes err: %v", err)
	}

	for _, expected := range []string{
		"[Project Plan](/project-plan)",
		"[Brainstorm](/daily/brainstorm)",
		"[[Meeting Notes]]",
		"[Meeting Alias](/daily/meeting-notes)",
		"![diagram.png](/assets/" + homePage.ID + "/diagram.png)",
		"`[[Project Plan]]`",
		"[[Daily/Meeting Notes]]",
		"![[Attachments/diagram.png]]",
	} {
		if !strings.Contains(homePage.Content, expected) {
			t.Fatalf("expected home content to contain %q, got:\n%s", expected, homePage.Content)
		}
	}

	for _, expected := range []string{
		"[Meeting Notes](/daily/meeting-notes)",
		"[Home](/home)",
	} {
		if !strings.Contains(projectPlanPage.Content, expected) {
			t.Fatalf("expected project-plan content to contain %q, got:\n%s", expected, projectPlanPage.Content)
		}
	}

	if !strings.Contains(meetingNotesPage.Content, "[Home](/home)") {
		t.Fatalf("expected meeting-notes content to contain rewritten home link, got:\n%s", meetingNotesPage.Content)
	}
	if !strings.Contains(brainstormPage.Content, "[Home](/home)") {
		t.Fatalf("expected brainstorm content to contain rewritten home link, got:\n%s", brainstormPage.Content)
	}

	assets, err := probe.ListAssets(homePage.ID)
	if err != nil {
		t.Fatalf("ListAssets err: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 uploaded asset, got %#v", assets)
	}
}
