package properties

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
)

// ─── ExtractPropertiesFromContent ────────────────────────────────────────────

func TestExtractPropertiesFromContent_TextValue(t *testing.T) {
	content := "---\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_MultipleStringValues(t *testing.T) {
	content := "---\nstatus: draft\nauthor: alice\nenvironment: staging\n---\n"
	got := ExtractPropertiesFromContent(content)

	if len(got) != 3 {
		t.Fatalf("expected 3 properties, got %d: %v", len(got), got)
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
	assertEntry(t, got, "author", PropertyEntry{Value: "alice", Type: "text"})
	assertEntry(t, got, "environment", PropertyEntry{Value: "staging", Type: "text"})
}

func TestExtractPropertiesFromContent_SkipsNumberValues(t *testing.T) {
	content := "---\nscore: 42\nrating: 4.5\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["score"]; ok {
		t.Error("integer value must not be stored as a property")
	}
	if _, ok := got["rating"]; ok {
		t.Error("float value must not be stored as a property")
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_SkipsBooleanValues(t *testing.T) {
	content := "---\nfeatured: true\narchived: false\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["featured"]; ok {
		t.Error("boolean true must not be stored as a property")
	}
	if _, ok := got["archived"]; ok {
		t.Error("boolean false must not be stored as a property")
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_SkipsEmptyStringValues(t *testing.T) {
	content := "---\nempty: \"\"\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["empty"]; ok {
		t.Error("empty string value must not be stored as a property")
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_SkipsWhitespaceOnlyValues(t *testing.T) {
	content := "---\nblank: \"   \"\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["blank"]; ok {
		t.Error("whitespace-only string value must not be stored as a property")
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_TrimsValueWhitespace(t *testing.T) {
	content := "---\nstatus: \"  draft  \"\n---\n"
	got := ExtractPropertiesFromContent(content)
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_SkipsMultilineStringValues(t *testing.T) {
	content := "---\ndescription: |\n  line one\n  line two\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["description"]; ok {
		t.Error("multi-line string value must not be stored as a property")
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

// ─── Reserved keys must be skipped ───────────────────────────────────────────

func TestExtractPropertiesFromContent_SkipsTagsKey(t *testing.T) {
	content := "---\ntags:\n  - react\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["tags"]; ok {
		t.Error("'tags' must not be stored as a property")
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_SkipsTagsKeyCaseInsensitive(t *testing.T) {
	content := "---\nTags:\n  - react\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["Tags"]; ok {
		t.Error("'Tags' (mixed case) must not be stored as a property")
	}
}

func TestExtractPropertiesFromContent_SkipsTitleKey(t *testing.T) {
	content := "---\ntitle: My Page\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["title"]; ok {
		t.Error("'title' must not be stored as a property")
	}
}

func TestExtractPropertiesFromContent_SkipsTitleKeyCaseInsensitive(t *testing.T) {
	content := "---\nTitle: My Page\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["Title"]; ok {
		t.Error("'Title' (mixed case) must not be stored as a property")
	}
}

func TestExtractPropertiesFromContent_SkipsLeafwikiPrefix(t *testing.T) {
	content := "---\nleafwiki_id: abc123\nleafwiki_created: 2024-01-01\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	for key := range got {
		if len(key) >= 9 && key[:9] == "leafwiki_" {
			t.Errorf("key %q with leafwiki_ prefix must not be stored", key)
		}
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_SkipsLeafwikiPrefixCaseInsensitive(t *testing.T) {
	content := "---\nLeafwiki_ID: abc\nstatus: ok\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["Leafwiki_ID"]; ok {
		t.Error("'Leafwiki_ID' must not be stored (reserved prefix, any case)")
	}
}

// ─── Non-scalar values must be skipped ───────────────────────────────────────

func TestExtractPropertiesFromContent_SkipsListValues(t *testing.T) {
	content := "---\nkeywords: [go, testing]\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["keywords"]; ok {
		t.Error("list values must not be stored as properties")
	}
	assertEntry(t, got, "status", PropertyEntry{Value: "draft", Type: "text"})
}

func TestExtractPropertiesFromContent_SkipsBlockListValues(t *testing.T) {
	content := "---\nkeywords:\n  - go\n  - testing\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["keywords"]; ok {
		t.Error("block list values must not be stored as properties")
	}
}

func TestExtractPropertiesFromContent_SkipsNilValues(t *testing.T) {
	content := "---\nempty:\nstatus: draft\n---\n"
	got := ExtractPropertiesFromContent(content)
	if _, ok := got["empty"]; ok {
		t.Error("nil/empty values must not be stored as properties")
	}
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

func TestExtractPropertiesFromContent_NoFrontmatterReturnsNil(t *testing.T) {
	got := ExtractPropertiesFromContent("# Page\n\nNo frontmatter.")
	if got != nil {
		t.Errorf("expected nil for content without frontmatter, got %v", got)
	}
}

func TestExtractPropertiesFromContent_EmptyContentReturnsNil(t *testing.T) {
	got := ExtractPropertiesFromContent("")
	if got != nil {
		t.Errorf("expected nil for empty content, got %v", got)
	}
}

func TestExtractPropertiesFromContent_OnlyReservedKeysReturnsNil(t *testing.T) {
	content := "---\ntags:\n  - go\ntitle: My Page\nleafwiki_id: abc\n---\n"
	got := ExtractPropertiesFromContent(content)
	if got != nil {
		t.Errorf("expected nil when all keys are reserved, got %v", got)
	}
}

func TestExtractPropertiesFromContent_OnlyNonStringValuesReturnsNil(t *testing.T) {
	content := "---\nscore: 42\nfeatured: true\nrating: 4.5\n---\n"
	got := ExtractPropertiesFromContent(content)
	if got != nil {
		t.Errorf("expected nil when all values are non-string, got %v", got)
	}
}

// ─── PropertiesService integration (with real tree + store) ──────────────────

func setupPropertiesService(t *testing.T) (*PropertiesService, *tree.TreeService) {
	t.Helper()

	dir := t.TempDir()
	ts := tree.NewTreeService(dir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	store, err := NewPropertiesStore(dir)
	if err != nil {
		t.Fatalf("NewPropertiesStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })

	return NewPropertiesService(ts, store), ts
}

func pageKind() *tree.NodeKind {
	k := tree.NodeKindPage
	return &k
}

func createPageWithContent(t *testing.T, ts *tree.TreeService, title, slug, content string) string {
	t.Helper()
	idPtr, err := ts.CreateNode("system", nil, title, slug, pageKind())
	if err != nil {
		t.Fatalf("CreateNode %q: %v", slug, err)
	}
	if err := ts.UpdateNode("system", *idPtr, title, slug, &content, tree.VersionUnchecked, false); err != nil {
		t.Fatalf("UpdateNode %q: %v", slug, err)
	}
	return *idPtr
}

func TestPropertiesService_IndexAllPages_BuildsIndex(t *testing.T) {
	svc, ts := setupPropertiesService(t)

	id1 := createPageWithContent(t, ts, "Page A", "page-a", "---\nstatus: draft\n---\n# A")
	id2 := createPageWithContent(t, ts, "Page B", "page-b", "---\nstatus: published\n---\n# B")

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	ids, err := svc.GetPageIDsByProperty("status", "draft")
	if err != nil {
		t.Fatalf("GetPageIDsByProperty: %v", err)
	}
	if len(ids) != 1 || ids[0] != id1 {
		t.Errorf("expected [%s], got %v", id1, ids)
	}

	ids2, err := svc.GetPageIDsByProperty("status", "published")
	if err != nil {
		t.Fatalf("GetPageIDsByProperty: %v", err)
	}
	if len(ids2) != 1 || ids2[0] != id2 {
		t.Errorf("expected [%s], got %v", id2, ids2)
	}
}

func TestPropertiesService_IndexAllPages_IsIdempotent(t *testing.T) {
	svc, ts := setupPropertiesService(t)
	createPageWithContent(t, ts, "Page A", "page-a", "---\nstatus: draft\n---\n# A")

	for i := 0; i < 3; i++ {
		if err := svc.IndexAllPages(); err != nil {
			t.Fatalf("IndexAllPages (run %d): %v", i, err)
		}
	}

	keys, err := svc.GetAllPropertyKeys("", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].Key != "status" || keys[0].Count != 1 {
		t.Errorf("expected [{status 1}], got %v", keys)
	}
}

func TestPropertiesService_IndexAllPages_SkipsReservedKeys(t *testing.T) {
	svc, ts := setupPropertiesService(t)
	createPageWithContent(t, ts, "Page A", "page-a",
		"---\ntags:\n  - go\ntitle: Custom\nleafwiki_id: abc\nstatus: draft\n---\n# A")

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	keys, err := svc.GetAllPropertyKeys("", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}

	for _, kc := range keys {
		switch kc.Key {
		case "tags", "title":
			t.Errorf("reserved key %q must not be indexed", kc.Key)
		default:
			if len(kc.Key) >= 9 && kc.Key[:9] == "leafwiki_" {
				t.Errorf("reserved prefix key %q must not be indexed", kc.Key)
			}
		}
	}
	if len(keys) != 1 || keys[0].Key != "status" {
		t.Errorf("expected only [status], got %v", keys)
	}
}

func TestPropertiesService_IndexAllPages_SkipsListValues(t *testing.T) {
	svc, ts := setupPropertiesService(t)
	createPageWithContent(t, ts, "Page A", "page-a",
		"---\nkeywords: [go, testing]\nstatus: draft\n---\n# A")

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	keys, err := svc.GetAllPropertyKeys("", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}
	for _, kc := range keys {
		if kc.Key == "keywords" {
			t.Errorf("list-valued property 'keywords' must not be indexed")
		}
	}
}

func TestPropertiesService_IndexAllPages_PagesWithoutPropertiesAreSkipped(t *testing.T) {
	svc, ts := setupPropertiesService(t)
	createPageWithContent(t, ts, "No Props", "no-props", "# Just content")

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	keys, err := svc.GetAllPropertyKeys("", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected no keys for page without properties, got %v", keys)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func assertEntry(t *testing.T, got map[string]PropertyEntry, key string, want PropertyEntry) {
	t.Helper()
	entry, ok := got[key]
	if !ok {
		t.Errorf("key %q missing from result %v", key, got)
		return
	}
	if entry != want {
		t.Errorf("key %q: got %+v, want %+v", key, entry, want)
	}
}
