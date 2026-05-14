package tags

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

func newTestStore(t *testing.T) *TagsStore {
	t.Helper()
	store, err := NewTagsStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTagsStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	return store
}

// ─── DB lifecycle ────────────────────────────────────────────────────────────

func TestTagsStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewTagsStore(tmp)
	if err != nil {
		t.Fatalf("NewTagsStore: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(tmp, "tags.db")); err != nil {
		t.Fatalf("expected tags.db to exist: %v", err)
	}
}

func TestTagsStore_IdempotentSchema(t *testing.T) {
	tmp := t.TempDir()
	for i := 0; i < 3; i++ {
		store, err := NewTagsStore(tmp)
		if err != nil {
			t.Fatalf("NewTagsStore (run %d): %v", i, err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close (run %d): %v", i, err)
		}
	}
}

// ─── SetTagsForPage ──────────────────────────────────────────────────────────

func TestTagsStore_SetTagsForPage_StoresTags(t *testing.T) {
	store := newTestStore(t)

	if err := store.SetTagsForPage("page-1", []string{"go", "testing"}); err != nil {
		t.Fatalf("SetTagsForPage: %v", err)
	}

	got, err := store.GetTagsForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}

	want := []string{"go", "testing"}
	assertStringSliceEqual(t, got["page-1"], want)
}

func TestTagsStore_SetTagsForPage_ReplacesOnSecondCall(t *testing.T) {
	store := newTestStore(t)

	if err := store.SetTagsForPage("page-1", []string{"go", "testing"}); err != nil {
		t.Fatalf("first SetTagsForPage: %v", err)
	}
	if err := store.SetTagsForPage("page-1", []string{"typescript"}); err != nil {
		t.Fatalf("second SetTagsForPage: %v", err)
	}

	got, err := store.GetTagsForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}
	assertStringSliceEqual(t, got["page-1"], []string{"typescript"})
}

func TestTagsStore_SetTagsForPage_EmptyTagsClearsExisting(t *testing.T) {
	store := newTestStore(t)

	if err := store.SetTagsForPage("page-1", []string{"go", "testing"}); err != nil {
		t.Fatalf("SetTagsForPage: %v", err)
	}
	if err := store.SetTagsForPage("page-1", []string{}); err != nil {
		t.Fatalf("SetTagsForPage (clear): %v", err)
	}

	got, err := store.GetTagsForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}
	if len(got["page-1"]) != 0 {
		t.Fatalf("expected empty tags, got %v", got["page-1"])
	}
}

func TestTagsStore_SetTagsForPage_NilTagsClearsExisting(t *testing.T) {
	store := newTestStore(t)

	if err := store.SetTagsForPage("page-1", []string{"go"}); err != nil {
		t.Fatalf("SetTagsForPage: %v", err)
	}
	if err := store.SetTagsForPage("page-1", nil); err != nil {
		t.Fatalf("SetTagsForPage (nil): %v", err)
	}

	got, err := store.GetTagsForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}
	if len(got["page-1"]) != 0 {
		t.Fatalf("expected empty tags after nil set, got %v", got["page-1"])
	}
}

// ─── DeleteTagsForPage ───────────────────────────────────────────────────────

func TestTagsStore_DeleteTagsForPage_RemovesTags(t *testing.T) {
	store := newTestStore(t)

	if err := store.SetTagsForPage("page-1", []string{"go", "testing"}); err != nil {
		t.Fatalf("SetTagsForPage: %v", err)
	}
	if err := store.DeleteTagsForPage("page-1"); err != nil {
		t.Fatalf("DeleteTagsForPage: %v", err)
	}

	got, err := store.GetTagsForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}
	if len(got["page-1"]) != 0 {
		t.Fatalf("expected empty tags after delete, got %v", got["page-1"])
	}
}

func TestTagsStore_DeleteTagsForPage_NonExistentPageIsNoop(t *testing.T) {
	store := newTestStore(t)
	if err := store.DeleteTagsForPage("does-not-exist"); err != nil {
		t.Fatalf("DeleteTagsForPage on unknown page: %v", err)
	}
}

func TestTagsStore_DeleteTagsForPage_DoesNotAffectOtherPages(t *testing.T) {
	store := newTestStore(t)

	if err := store.SetTagsForPage("page-1", []string{"go"}); err != nil {
		t.Fatalf("SetTagsForPage page-1: %v", err)
	}
	if err := store.SetTagsForPage("page-2", []string{"typescript"}); err != nil {
		t.Fatalf("SetTagsForPage page-2: %v", err)
	}

	if err := store.DeleteTagsForPage("page-1"); err != nil {
		t.Fatalf("DeleteTagsForPage: %v", err)
	}

	got, err := store.GetTagsForPages([]string{"page-2"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}
	assertStringSliceEqual(t, got["page-2"], []string{"typescript"})
}

// ─── GetAllTags ──────────────────────────────────────────────────────────────

func TestTagsStore_GetAllTags_EmptyDB(t *testing.T) {
	store := newTestStore(t)
	tags, err := store.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected empty result, got %v", tags)
	}
}

func TestTagsStore_GetAllTags_ReturnsTagsWithCount(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"go", "testing"})
	_ = store.SetTagsForPage("page-2", []string{"go", "typescript"})
	_ = store.SetTagsForPage("page-3", []string{"typescript"})

	tags, err := store.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}

	byTag := make(map[string]int, len(tags))
	for _, tc := range tags {
		byTag[tc.Tag] = tc.Count
	}

	if byTag["go"] != 2 {
		t.Errorf("go count = %d, want 2", byTag["go"])
	}
	if byTag["typescript"] != 2 {
		t.Errorf("typescript count = %d, want 2", byTag["typescript"])
	}
	if byTag["testing"] != 1 {
		t.Errorf("testing count = %d, want 1", byTag["testing"])
	}
}

func TestTagsStore_GetAllTags_OrderByCountDescThenTagAsc(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"alpha", "beta", "gamma"})
	_ = store.SetTagsForPage("page-2", []string{"alpha", "beta"})
	_ = store.SetTagsForPage("page-3", []string{"alpha"})

	tags, err := store.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}

	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	// alpha: 3, beta: 2, gamma: 1
	if tags[0].Tag != "alpha" || tags[0].Count != 3 {
		t.Errorf("tags[0] = %+v, want {alpha 3}", tags[0])
	}
	if tags[1].Tag != "beta" || tags[1].Count != 2 {
		t.Errorf("tags[1] = %+v, want {beta 2}", tags[1])
	}
	if tags[2].Tag != "gamma" || tags[2].Count != 1 {
		t.Errorf("tags[2] = %+v, want {gamma 1}", tags[2])
	}
}

func TestTagsStore_GetAllTags_OrderAlphaForSameCount(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"zebra", "apple"})

	tags, err := store.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	if len(tags) < 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Tag != "apple" {
		t.Errorf("expected apple first (same count, alphabetic), got %q", tags[0].Tag)
	}
}

func TestTagsStore_GetAllTags_FilterByPrefix(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"react", "redux", "rails", "node"})

	tags, err := store.GetAllTags("re", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}

	for _, tc := range tags {
		if len(tc.Tag) < 2 || tc.Tag[:2] != "re" {
			t.Errorf("tag %q does not start with 're'", tc.Tag)
		}
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags matching 're', got %d: %v", len(tags), tags)
	}
}

func TestTagsStore_GetAllTags_EmptyFilterReturnsAll(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"alpha", "beta", "gamma"})

	tags, err := store.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}
}

func TestTagsStore_GetAllTags_RespectsLimit(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"a", "b", "c", "d", "e"})

	tags, err := store.GetAllTags("", 3)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 tags (limit), got %d", len(tags))
	}
}

func TestTagsStore_GetAllTags_ZeroLimitReturnsAll(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"a", "b", "c", "d", "e"})

	tags, err := store.GetAllTags("", 0)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	if len(tags) != 5 {
		t.Errorf("expected all 5 tags, got %d", len(tags))
	}
}

// ─── GetPageIDsByTags ────────────────────────────────────────────────────────

func TestTagsStore_GetPageIDsByTags_ANDLogic(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"react", "typescript"})
	_ = store.SetTagsForPage("page-2", []string{"react"})
	_ = store.SetTagsForPage("page-3", []string{"typescript"})
	_ = store.SetTagsForPage("page-4", []string{"vue", "typescript"})

	// Only page-1 has BOTH react AND typescript
	ids, err := store.GetPageIDsByTags([]string{"react", "typescript"})
	if err != nil {
		t.Fatalf("GetPageIDsByTags: %v", err)
	}
	if len(ids) != 1 || ids[0] != "page-1" {
		t.Errorf("expected [page-1], got %v", ids)
	}
}

func TestTagsStore_GetPageIDsByTags_SingleTag(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"react", "typescript"})
	_ = store.SetTagsForPage("page-2", []string{"react"})
	_ = store.SetTagsForPage("page-3", []string{"vue"})

	ids, err := store.GetPageIDsByTags([]string{"react"})
	if err != nil {
		t.Fatalf("GetPageIDsByTags: %v", err)
	}

	sort.Strings(ids)
	want := []string{"page-1", "page-2"}
	assertStringSliceEqual(t, ids, want)
}

func TestTagsStore_GetPageIDsByTags_NoMatch(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"react"})

	ids, err := store.GetPageIDsByTags([]string{"vue"})
	if err != nil {
		t.Fatalf("GetPageIDsByTags: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected no matches, got %v", ids)
	}
}

func TestTagsStore_GetPageIDsByTags_EmptyInputReturnsNil(t *testing.T) {
	store := newTestStore(t)
	_ = store.SetTagsForPage("page-1", []string{"react"})

	ids, err := store.GetPageIDsByTags([]string{})
	if err != nil {
		t.Fatalf("GetPageIDsByTags: %v", err)
	}
	if ids != nil {
		t.Errorf("expected nil, got %v", ids)
	}
}

func TestTagsStore_GetPageIDsByTags_ThreeTagAND(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"a", "b", "c"})
	_ = store.SetTagsForPage("page-2", []string{"a", "b"})
	_ = store.SetTagsForPage("page-3", []string{"a"})

	ids, err := store.GetPageIDsByTags([]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("GetPageIDsByTags: %v", err)
	}
	if len(ids) != 1 || ids[0] != "page-1" {
		t.Errorf("expected [page-1], got %v", ids)
	}
}

// ─── GetTagsForPages ─────────────────────────────────────────────────────────

func TestTagsStore_GetTagsForPages_ReturnsTagsForMultiplePages(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"go", "testing"})
	_ = store.SetTagsForPage("page-2", []string{"typescript"})
	_ = store.SetTagsForPage("page-3", []string{"react", "vue"})

	got, err := store.GetTagsForPages([]string{"page-1", "page-3"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}

	assertStringSliceEqual(t, got["page-1"], []string{"go", "testing"})
	assertStringSliceEqual(t, got["page-3"], []string{"react", "vue"})
	if _, ok := got["page-2"]; ok {
		t.Errorf("page-2 should not be in result")
	}
}

func TestTagsStore_GetTagsForPages_EmptyInputReturnsEmptyMap(t *testing.T) {
	store := newTestStore(t)

	got, err := store.GetTagsForPages([]string{})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}
	if got == nil {
		t.Fatalf("expected empty map, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestTagsStore_GetTagsForPages_UnknownIDReturnsNoEntry(t *testing.T) {
	store := newTestStore(t)

	got, err := store.GetTagsForPages([]string{"does-not-exist"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map for unknown IDs, got %v", got)
	}
}

// ─── Clear ───────────────────────────────────────────────────────────────────

func TestTagsStore_Clear_RemovesAllEntries(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetTagsForPage("page-1", []string{"go"})
	_ = store.SetTagsForPage("page-2", []string{"typescript"})

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	tags, err := store.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags after Clear: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected empty after Clear, got %v", tags)
	}
}

// ─── GetAllTags — LIKE wildcard escaping ─────────────────────────────────────

func TestTagsStore_GetAllTags_FilterEscapesLikeWildcards(t *testing.T) {
	store := newTestStore(t)

	// Tags that would accidentally match if % or _ were treated as wildcards.
	_ = store.SetTagsForPage("page-1", []string{"react", "redux"})

	// A filter of "%" would match everything without escaping.
	tags, err := store.GetAllTags("%", 50)
	if err != nil {
		t.Fatalf("GetAllTags with %% filter: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("filter '%%' should match no tags (literal), got %v", tags)
	}

	// A filter of "_eact" would match "react" without escaping.
	tags, err = store.GetAllTags("_eact", 50)
	if err != nil {
		t.Fatalf("GetAllTags with _ filter: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("filter '_eact' should match no tags (literal), got %v", tags)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// assertStringSliceEqual checks that got and want contain the same elements,
// sorting both to allow for any insertion order.
func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()

	gc := append([]string(nil), got...)
	wc := append([]string(nil), want...)
	sort.Strings(gc)
	sort.Strings(wc)

	if len(gc) != len(wc) {
		t.Errorf("len = %d, want %d\n got:  %v\n want: %v", len(gc), len(wc), gc, wc)
		return
	}
	for i := range gc {
		if gc[i] != wc[i] {
			t.Errorf("[%d] = %q, want %q\n got:  %v\n want: %v", i, gc[i], wc[i], gc, wc)
			return
		}
	}
}
