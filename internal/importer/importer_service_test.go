package importer

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
)

// --- Helpers ----------------------------------------------------------------

func mustWrite(t *testing.T, base, rel, content string) string {
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

func newServiceWithFakeWiki(t *testing.T, w *fakeWiki) *ImporterService {
	t.Helper()
	planner := NewPlanner(w, tree.NewSlugService())
	store := NewPlanStore()
	return &ImporterService{
		planner:   planner,
		planStore: store,
		extractor: NewZipExtractor(), // unused in these tests
		logger:    slog.Default().With("component", "ImporterServiceTest"),
	}
}

func importerFixturePath(t *testing.T, rel string) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	candidates := []string{
		filepath.Join(wd, "fixtures", rel),
		filepath.Join(wd, "internal", "importer", "fixtures", rel),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	t.Fatalf("fixture path not found for %q from working directory %q", rel, wd)
	return ""
}

func copyFixtureToTemp(t *testing.T, rel string) string {
	t.Helper()

	sourceRoot := importerFixturePath(t, rel)
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

// --- Tests ------------------------------------------------------------------

func TestImporterService_createImportPlanFromFolder_StoresPlan(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, tmp, "a.md", "# A\nbody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(tmp, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if plan == nil || len(plan.Items) != 1 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	// plan should have correct options

	if _, err := is.GetCurrentPlan(); err != nil {
		t.Fatalf("GetCurrentPlan err: %v", err)
	}
}

func TestImporterService_createImportPlanFromFolder_CleansUpOldWorkspace(t *testing.T) {
	// old workspace with a marker file
	oldWS := t.TempDir()
	marker := mustWrite(t, oldWS, "marker.txt", "x")

	// new workspace with md
	newWS := t.TempDir()
	mustWrite(t, newWS, "b.md", "# B")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	// seed old plan in store
	is.planStore.Set(&StoredPlan{
		Plan:          &PlanResult{ID: "old", TreeHash: "h1"},
		PlanOptions:   PlanOptions{SourceBasePath: oldWS},
		WorkspaceRoot: oldWS,
		CreatedAt:     time.Now(),
	})

	_, err := is.createImportPlanFromFolder(newWS, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}

	// old workspace should be removed
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("expected old workspace removed; statErr=%v", statErr)
	}

	// store should now point to new workspace

	if _, err := is.GetCurrentPlan(); err != nil {
		t.Fatalf("GetCurrentPlan err: %v", err)
	}
}

func TestImporterService_GetCurrentPlan_NoPlan(t *testing.T) {
	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	_, err := is.GetCurrentPlan()
	if !errors.Is(err, ErrNoPlan) {
		t.Fatalf("expected ErrNoPlan, got %v", err)
	}
}

func TestImporterService_ClearCurrentPlan(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, tmp, "a.md", "# A")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	_, err := is.createImportPlanFromFolder(tmp, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}

	is.ClearCurrentPlan()
	_, err = is.GetCurrentPlan()
	if !errors.Is(err, ErrNoPlan) {
		t.Fatalf("expected ErrNoPlan after clear, got %v", err)
	}
}

func TestImporterService_ExecuteCurrentPlan_NoPlan(t *testing.T) {
	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	_, err := is.ExecuteCurrentPlan("user1")
	if !errors.Is(err, ErrNoPlan) {
		t.Fatalf("expected ErrNoPlan, got %v", err)
	}
}

func TestImporterService_ExecuteCurrentPlan_HappyPath_PreservesNonInternalFrontmatter(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "---\naliases:\n  - x\ncustom_key: keep-me\nleafwiki_id: source-id\nleafwiki_title: Source Title\ntitle: X\n---\n\n# Heading\nBody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if plan.TreeHash != "h1" {
		t.Fatalf("plan.TreeHash=%q want h1", plan.TreeHash)
	}

	res, err := is.ExecuteCurrentPlan("user1")
	if err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	if res.ImportedCount != 1 {
		t.Fatalf("ImportedCount=%d want 1", res.ImportedCount)
	}
	if res.SkippedCount != 0 {
		t.Fatalf("SkippedCount=%d want 0", res.SkippedCount)
	}
	if w.ensureCalls != 1 || w.updateCalls != 1 {
		t.Fatalf("wiki calls ensure=%d update=%d", w.ensureCalls, w.updateCalls)
	}

	if w.lastUpdatedContent == nil {
		t.Fatalf("expected UpdatePage content")
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
}

func TestImporterService_ExecuteCurrentPlan_WritesPreservedFrontmatterToDisk(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "Imported.md", "---\naliases:\n  - alpha\ncustom_key: keep-me\nleafwiki_id: source-id\ntitle: Imported Title\n---\n\n# Imported Title\nBody")

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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	planner := NewPlanner(w, tree.NewSlugService())
	is := NewImporterService(planner, NewPlanStore())

	plan, err := is.createImportPlanFromFolder(ws, "")
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
	mustWrite(t, ws, "Guides/index.md", "# Guides")
	mustWrite(t, ws, "Guides/Setup.md", strings.Join([]string{
		"# Setup",
		"",
		"[Guide Home](/Guides/)",
		"[API](../Reference/Endpoints.md#intro)",
		"![[./images/logo.png]]",
		"[Manual](/shared/manual.pdf)",
		"[[Reference/Endpoints|API Alias]]",
	}, "\n"))
	mustWrite(t, ws, "Reference/Endpoints.md", "# Endpoints")
	mustWrite(t, ws, "Guides/images/logo.png", "png-bytes")
	mustWrite(t, ws, "shared/manual.pdf", "pdf-bytes")

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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	planner := NewPlanner(w, tree.NewSlugService())
	is := NewImporterService(planner, NewPlanStore())

	plan, err := is.createImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 3 {
		t.Fatalf("expected three plan items, got %#v", plan.Items)
	}

	if _, err := is.ExecuteCurrentPlan("system"); err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	setupPage, err := w.FindByPath("guides/setup")
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

	assets, err := w.ListAssets(setupPage.ID)
	if err != nil {
		t.Fatalf("ListAssets err: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 uploaded assets, got %#v", assets)
	}
}

func TestImporterService_ExecuteCurrentPlan_ImportsFixturePackage(t *testing.T) {
	ws := copyFixtureToTemp(t, "link-assets-package")

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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	planner := NewPlanner(w, tree.NewSlugService())
	is := NewImporterService(planner, NewPlanStore())

	plan, err := is.createImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 5 {
		t.Fatalf("expected five plan items, got %#v", plan.Items)
	}

	if _, err := is.ExecuteCurrentPlan("system"); err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	setupPage, err := w.FindByPath("guides/setup")
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

	assets, err := w.ListAssets(setupPage.ID)
	if err != nil {
		t.Fatalf("ListAssets err: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 uploaded assets, got %#v", assets)
	}

	if _, err := w.FindByPath("reference/endpoints"); err != nil {
		t.Fatalf("FindByPath reference/endpoints err: %v", err)
	}
	if _, err := w.FindByPath("reference/api-1"); err != nil {
		t.Fatalf("FindByPath reference/api-1 err: %v", err)
	}
	if _, err := w.FindByPath("guides"); err != nil {
		t.Fatalf("FindByPath guides err: %v", err)
	}
	if _, err := w.FindByPath("readme"); err != nil {
		t.Fatalf("FindByPath readme err: %v", err)
	}
}

func TestImporterService_ExecuteCurrentPlan_ImportsLeafWikiNestedFixture(t *testing.T) {
	ws := copyFixtureToTemp(t, "leafwiki-nested-package")

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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	planner := NewPlanner(w, tree.NewSlugService())
	is := NewImporterService(planner, NewPlanStore())

	plan, err := is.createImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 5 {
		t.Fatalf("expected five plan items, got %#v", plan.Items)
	}

	if _, err := is.ExecuteCurrentPlan("system"); err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	introPage, err := w.FindByPath("intro")
	if err != nil {
		t.Fatalf("FindByPath intro err: %v", err)
	}
	gettingStartedPage, err := w.FindByPath("docs/getting-started")
	if err != nil {
		t.Fatalf("FindByPath docs/getting-started err: %v", err)
	}
	basicGuidePage, err := w.FindByPath("docs/guides/basic-guide")
	if err != nil {
		t.Fatalf("FindByPath docs/guides/basic-guide err: %v", err)
	}
	if _, err := w.FindByPath("docs"); err != nil {
		t.Fatalf("FindByPath docs err: %v", err)
	}
	if _, err := w.FindByPath("docs/guides"); err != nil {
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
	ws := copyFixtureToTemp(t, "obsidian-wikilinks-package")

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
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	planner := NewPlanner(w, tree.NewSlugService())
	is := NewImporterService(planner, NewPlanStore())

	plan, err := is.createImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if len(plan.Items) != 3 {
		t.Fatalf("expected three plan items, got %#v", plan.Items)
	}

	if _, err := is.ExecuteCurrentPlan("system"); err != nil {
		t.Fatalf("ExecuteCurrentPlan err: %v", err)
	}

	homePage, err := w.FindByPath("home")
	if err != nil {
		t.Fatalf("FindByPath home err: %v", err)
	}
	projectPlanPage, err := w.FindByPath("project-plan")
	if err != nil {
		t.Fatalf("FindByPath project-plan err: %v", err)
	}
	meetingNotesPage, err := w.FindByPath("daily/meeting-notes")
	if err != nil {
		t.Fatalf("FindByPath daily/meeting-notes err: %v", err)
	}

	for _, expected := range []string{
		"[Project Plan](/project-plan)",
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

	assets, err := w.ListAssets(homePage.ID)
	if err != nil {
		t.Fatalf("ListAssets err: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 uploaded asset, got %#v", assets)
	}
}

func TestImporterService_ExecuteCurrentPlan_ExecutorStalePlanPropagatesError(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, ws, "a.md", "# A")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(ws, "")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	// make plan stale
	plan.TreeHash = "OLD"

	_, err = is.ExecuteCurrentPlan("user1")
	if err == nil {
		t.Fatalf("expected stale plan error")
	}
	if !strings.Contains(err.Error(), "plan is stale") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindMarkdownEntries_FindsMdRecursively_AndNormalizesSlashes(t *testing.T) {
	base := t.TempDir()
	mustWrite(t, base, "a.md", "x")
	mustWrite(t, base, "b.txt", "x")
	mustWrite(t, base, "sub/c.MD", "x")
	mustWrite(t, base, "sub/deeper/d.md", "x")

	got, err := FindMarkdownEntries(base)
	if err != nil {
		t.Fatalf("FindMarkdownEntries err: %v", err)
	}

	// collect paths in a set for stable assertion (WalkDir order is OS-dependent)
	set := map[string]bool{}
	for _, e := range got {
		set[e.SourcePath] = true
		// should be slash-normalized
		if strings.Contains(e.SourcePath, `\`) {
			t.Fatalf("SourcePath should be slash-normalized: %q", e.SourcePath)
		}
	}

	if !set["a.md"] {
		t.Fatalf("missing a.md, got %#v", set)
	}
	if !set["sub/c.MD"] {
		t.Fatalf("missing sub/c.MD, got %#v", set)
	}
	if !set["sub/deeper/d.md"] {
		t.Fatalf("missing sub/deeper/d.md, got %#v", set)
	}
	if set["b.txt"] {
		t.Fatalf("should not include b.txt")
	}
}

func TestImporterService_createImportPlanFromFolder_UsesTargetBasePath(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, tmp, "a.md", "# A\nbody")

	w := &fakeWiki{treeHash: "h1", lookups: map[string]*tree.PathLookup{}}
	is := newServiceWithFakeWiki(t, w)

	plan, err := is.createImportPlanFromFolder(tmp, "docs/imports")
	if err != nil {
		t.Fatalf("createImportPlanFromFolder err: %v", err)
	}
	if plan == nil || len(plan.Items) != 1 {
		t.Fatalf("unexpected plan: %#v", plan)
	}

	// Verify the plan item has the correct target path with the base path
	item := plan.Items[0]
	if item.TargetPath != "docs/imports/a" {
		t.Fatalf("expected TargetPath 'docs/imports/a', got %q", item.TargetPath)
	}

	// Verify the stored plan options has the correct target base path
	sp, err := is.planStore.Get()
	if err != nil {
		t.Fatalf("Get plan err: %v", err)
	}
	if sp.PlanOptions.TargetBasePath != "docs/imports" {
		t.Fatalf("expected TargetBasePath 'docs/imports', got %q", sp.PlanOptions.TargetBasePath)
	}
}

func TestFindMarkdownEntries_FindsMixedCaseMdExtensions(t *testing.T) {
	base := t.TempDir()
	mustWrite(t, base, "a.MD", "x")
	mustWrite(t, base, "b.mD", "x")
	mustWrite(t, base, "c.Md", "x")
	mustWrite(t, base, "d.txt", "x")

	got, err := FindMarkdownEntries(base)
	if err != nil {
		t.Fatalf("FindMarkdownEntries err: %v", err)
	}

	set := map[string]bool{}
	for _, e := range got {
		set[e.SourcePath] = true
	}

	if !set["a.MD"] || !set["b.mD"] || !set["c.Md"] {
		t.Fatalf("expected mixed-case markdown files to be included, got %#v", set)
	}
	if set["d.txt"] {
		t.Fatalf("should not include non-markdown files")
	}
}
