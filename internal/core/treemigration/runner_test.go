package treemigration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
)

func writeSchema(t *testing.T, dir string, version int) {
	t.Helper()

	raw, err := json.MarshalIndent(struct {
		Version int `json:"version"`
	}{Version: version}, "", "  ")
	if err != nil {
		t.Fatalf("marshal schema failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "schema.json"), raw, 0o644); err != nil {
		t.Fatalf("write schema failed: %v", err)
	}
}

func ptrKind(kind tree.NodeKind) *tree.NodeKind { return &kind }

func persistLegacyTreeSnapshot(t *testing.T, storageDir string, root *tree.PageNode) {
	t.Helper()
	raw, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal legacy tree snapshot failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "tree.json"), raw, 0o644); err != nil {
		t.Fatalf("write legacy tree snapshot failed: %v", err)
	}
}

func TestTreeMigration_LoadTree_MigratesToV2_AddsFrontmatterAndPreservesBody(t *testing.T) {
	if tree.CurrentSchemaVersion < 2 {
		t.Skip("requires schema v2+")
	}

	tmpDir := t.TempDir()
	writeSchema(t, tmpDir, 1)

	svc := tree.NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Page1", "page1", ptrKind(tree.NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	body := "# Page 1 Content\nHello World\n"
	if err := os.WriteFile(pagePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write old content failed: %v", err)
	}
	writeSchema(t, tmpDir, 1)

	loaded := tree.NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree (migrating) failed: %v", err)
	}

	raw, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}

	fm, migratedBody, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration, got:\n%s", string(raw))
	}
	if fm.LeafWikiID != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if strings.TrimSpace(fm.LeafWikiTitle) == "" {
		t.Fatalf("expected leafwiki_title to be set")
	}
	if migratedBody != body {
		t.Fatalf("expected body preserved exactly.\nGot:\n%q\nWant:\n%q", migratedBody, body)
	}
}

func TestTreeMigration_LoadTree_MigratesToV2_PreservesExistingCustomFrontmatter(t *testing.T) {
	if tree.CurrentSchemaVersion < 2 {
		t.Skip("requires schema v2+")
	}

	tmpDir := t.TempDir()
	writeSchema(t, tmpDir, 1)

	svc := tree.NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Page1", "page1", ptrKind(tree.NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	legacyContent := `---
custom_key: keep-me
tags:
  - alpha
---
# Page 1 Content
Hello World
`
	if err := os.WriteFile(pagePath, []byte(legacyContent), 0o644); err != nil {
		t.Fatalf("write legacy content failed: %v", err)
	}
	writeSchema(t, tmpDir, 1)

	loaded := tree.NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree (migrating) failed: %v", err)
	}

	raw, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}

	migrated := string(raw)
	if !strings.Contains(migrated, "custom_key: keep-me") {
		t.Fatalf("expected custom frontmatter to be preserved, got:\n%s", migrated)
	}
	if !strings.Contains(migrated, "- alpha") {
		t.Fatalf("expected list frontmatter to be preserved, got:\n%s", migrated)
	}

	fm, migratedBody, has, err := markdown.ParseFrontmatter(migrated)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration, got:\n%s", migrated)
	}
	if fm.LeafWikiID != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if strings.TrimSpace(fm.LeafWikiTitle) == "" {
		t.Fatalf("expected leafwiki_title to be set")
	}
	wantBody := "# Page 1 Content\nHello World\n"
	if migratedBody != wantBody {
		t.Fatalf("expected body preserved exactly.\nGot:\n%q\nWant:\n%q", migratedBody, wantBody)
	}
}

func TestTreeMigration_LoadTree_MigratesToV3_BackfillsMetadataFrontmatter(t *testing.T) {
	if tree.CurrentSchemaVersion < 3 {
		t.Skip("requires schema v3+")
	}

	tmpDir := t.TempDir()
	writeSchema(t, tmpDir, 2)

	svc := tree.NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Page1", "page1", ptrKind(tree.NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	node, err := svc.FindPageByID(svc.GetTree().Children, *id)
	if err != nil {
		t.Fatalf("FindPageByID failed: %v", err)
	}
	node.Metadata = tree.PageMetadata{
		CreatedAt:    time.Date(2026, time.March, 21, 10, 15, 30, 0, time.UTC),
		UpdatedAt:    time.Date(2026, time.March, 21, 11, 16, 31, 0, time.UTC),
		CreatorID:    "alice",
		LastAuthorID: "bob",
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	legacyContent := "---\nleafwiki_id: " + *id + "\nleafwiki_title: Page1\n---\n# Page 1 Content\nHello World\n"
	if err := os.WriteFile(pagePath, []byte(legacyContent), 0o644); err != nil {
		t.Fatalf("write legacy content failed: %v", err)
	}
	writeSchema(t, tmpDir, 2)

	loaded := tree.NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree (migrating) failed: %v", err)
	}

	raw, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}

	fm, migratedBody, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration")
	}
	if fm.LeafWikiCreatedAt != "2026-03-21T10:15:30Z" || fm.LeafWikiUpdatedAt != "2026-03-21T11:16:31Z" {
		t.Fatalf("expected metadata timestamps to be backfilled, got %#v", fm)
	}
	if fm.LeafWikiCreatorID != "alice" || fm.LeafWikiLastAuthorID != "bob" {
		t.Fatalf("expected metadata authors to be backfilled, got %#v", fm)
	}
	wantBody := "# Page 1 Content\nHello World\n"
	if migratedBody != wantBody {
		t.Fatalf("expected body preserved exactly.\nGot:\n%q\nWant:\n%q", migratedBody, wantBody)
	}
}

func TestTreeMigration_LoadTree_MigratesToV5_BackfillsChildOrderFiles(t *testing.T) {
	if tree.CurrentSchemaVersion < 5 {
		t.Skip("requires schema v5+")
	}

	tmpDir := t.TempDir()
	writeSchema(t, tmpDir, 4)

	svc := tree.NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	docsID, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(tree.NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode docs failed: %v", err)
	}
	alphaID, err := svc.CreateNode("system", nil, "Alpha", "alpha", ptrKind(tree.NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode alpha failed: %v", err)
	}
	betaID, err := svc.CreateNode("system", docsID, "Beta", "beta", ptrKind(tree.NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode beta failed: %v", err)
	}

	root := svc.GetTree()
	root.Children = []*tree.PageNode{root.Children[1], root.Children[0]}
	for i, child := range root.Children {
		child.Position = i
	}

	if err := os.Remove(filepath.Join(tmpDir, "root", ".order.json")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove root order file: %v", err)
	}
	if err := os.Remove(filepath.Join(tmpDir, "root", "docs", ".order.json")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove docs order file: %v", err)
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())
	writeSchema(t, tmpDir, 4)

	loaded := tree.NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree (migrating) failed: %v", err)
	}

	var rootOrder struct {
		OrderedIDs []string `json:"ordered_ids"`
	}
	rawRootOrder, err := os.ReadFile(filepath.Join(tmpDir, "root", ".order.json"))
	if err != nil {
		t.Fatalf("read root order file: %v", err)
	}
	if err := json.Unmarshal(rawRootOrder, &rootOrder); err != nil {
		t.Fatalf("unmarshal root order file: %v", err)
	}
	wantRoot := []string{*alphaID, *docsID}
	if strings.Join(rootOrder.OrderedIDs, ",") != strings.Join(wantRoot, ",") {
		t.Fatalf("unexpected root order after migration: got %v want %v", rootOrder.OrderedIDs, wantRoot)
	}

	var docsOrder struct {
		OrderedIDs []string `json:"ordered_ids"`
	}
	rawDocsOrder, err := os.ReadFile(filepath.Join(tmpDir, "root", "docs", ".order.json"))
	if err != nil {
		t.Fatalf("read docs order file: %v", err)
	}
	if err := json.Unmarshal(rawDocsOrder, &docsOrder); err != nil {
		t.Fatalf("unmarshal docs order file: %v", err)
	}
	wantDocs := []string{*betaID}
	if strings.Join(docsOrder.OrderedIDs, ",") != strings.Join(wantDocs, ",") {
		t.Fatalf("unexpected docs order after migration: got %v want %v", docsOrder.OrderedIDs, wantDocs)
	}
}

func TestTreeMigration_LoadTree_MigratesToV4_MaterializesMissingSectionIndex(t *testing.T) {
	if tree.CurrentSchemaVersion < 4 {
		t.Skip("requires schema v4+")
	}

	tmpDir := t.TempDir()
	writeSchema(t, tmpDir, 3)

	svc := tree.NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(tree.NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	node, err := svc.FindPageByID(svc.GetTree().Children, *id)
	if err != nil {
		t.Fatalf("FindPageByID failed: %v", err)
	}
	node.Metadata = tree.PageMetadata{
		CreatedAt:    time.Date(2026, time.March, 22, 10, 15, 30, 0, time.UTC),
		UpdatedAt:    time.Date(2026, time.March, 22, 11, 16, 31, 0, time.UTC),
		CreatorID:    "alice",
		LastAuthorID: "bob",
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	indexPath := filepath.Join(tmpDir, "root", "docs", "index.md")
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("remove section index failed: %v", err)
	}
	writeSchema(t, tmpDir, 3)

	loaded := tree.NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree (migrating) failed: %v", err)
	}

	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read migrated section index: %v", err)
	}
	fm, body, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration")
	}
	if fm.LeafWikiID != *id || fm.LeafWikiTitle != "Docs" {
		t.Fatalf("expected section frontmatter to be materialized, got %#v", fm)
	}
	if fm.LeafWikiCreatedAt != "2026-03-22T10:15:30Z" || fm.LeafWikiUpdatedAt != "2026-03-22T11:16:31Z" {
		t.Fatalf("expected timestamps to be materialized, got %#v", fm)
	}
	if fm.LeafWikiCreatorID != "alice" || fm.LeafWikiLastAuthorID != "bob" {
		t.Fatalf("expected author metadata to be materialized, got %#v", fm)
	}
	if strings.TrimSpace(body) != "" {
		t.Fatalf("expected empty section body after migration, got %q", body)
	}
}

func TestTreeMigration_LoadTree_MigratesToV5_ReturnsErrorWhenOrderFileCannotBeWritten(t *testing.T) {
	if tree.CurrentSchemaVersion < 5 {
		t.Skip("requires schema v5+")
	}
	if runtime.GOOS == "windows" {
		t.Skip("permission-based migration failure test is not reliable on Windows")
	}

	tmpDir := t.TempDir()
	writeSchema(t, tmpDir, 4)

	svc := tree.NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	_, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(tree.NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	_, err = svc.CreateNode("system", nil, "Alpha", "alpha", ptrKind(tree.NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode alpha failed: %v", err)
	}

	if err := os.Remove(filepath.Join(tmpDir, "root", ".order.json")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove root order file failed: %v", err)
	}
	if err := os.Chmod(filepath.Join(tmpDir, "root"), 0o555); err != nil {
		t.Fatalf("chmod root dir failed: %v", err)
	}
	defer func() {
		_ = os.Chmod(filepath.Join(tmpDir, "root"), 0o755)
	}()
	writeSchema(t, tmpDir, 4)

	loaded := tree.NewTreeService(tmpDir)
	err = loaded.LoadTree()
	if err == nil {
		t.Fatalf("expected migration error when order file cannot be written")
	}
	if !strings.Contains(err.Error(), "persist child order") {
		t.Fatalf("expected migration error to mention child order persistence, got: %v", err)
	}
}

func TestTreeMigration_LoadTree_MigratesToV4_ReturnsErrorWhenSectionIndexCannotBeWritten(t *testing.T) {
	if tree.CurrentSchemaVersion < 4 {
		t.Skip("requires schema v4+")
	}
	if runtime.GOOS == "windows" {
		t.Skip("permission-based migration failure test is not reliable on Windows")
	}

	tmpDir := t.TempDir()
	writeSchema(t, tmpDir, 3)

	svc := tree.NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(tree.NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	node, err := svc.FindPageByID(svc.GetTree().Children, *id)
	if err != nil {
		t.Fatalf("FindPageByID failed: %v", err)
	}
	node.Metadata = tree.PageMetadata{
		CreatedAt:    time.Date(2026, time.March, 22, 10, 15, 30, 0, time.UTC),
		UpdatedAt:    time.Date(2026, time.March, 22, 11, 16, 31, 0, time.UTC),
		CreatorID:    "alice",
		LastAuthorID: "bob",
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	sectionDir := filepath.Join(tmpDir, "root", "docs")
	indexPath := filepath.Join(sectionDir, "index.md")
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("remove section index failed: %v", err)
	}
	if err := os.Chmod(sectionDir, 0o555); err != nil {
		t.Fatalf("chmod section dir failed: %v", err)
	}
	defer func() {
		_ = os.Chmod(sectionDir, 0o755)
	}()
	writeSchema(t, tmpDir, 3)

	loaded := tree.NewTreeService(tmpDir)
	err = loaded.LoadTree()
	if err == nil {
		t.Fatalf("expected migration error when section index cannot be written")
	}
	if !strings.Contains(err.Error(), "materialize section index") {
		t.Fatalf("expected migration error to mention section index materialization, got: %v", err)
	}
}
