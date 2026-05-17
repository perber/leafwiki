package properties

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

func newTestStore(t *testing.T) *PropertiesStore {
	t.Helper()
	store, err := NewPropertiesStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewPropertiesStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })
	return store
}

func props(kv ...string) map[string]PropertyEntry {
	m := make(map[string]PropertyEntry, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = PropertyEntry{Value: kv[i+1], Type: "text"}
	}
	return m
}

// ─── DB lifecycle ────────────────────────────────────────────────────────────

func TestPropertiesStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewPropertiesStore(tmp)
	if err != nil {
		t.Fatalf("NewPropertiesStore: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(tmp, "properties.db")); err != nil {
		t.Fatalf("expected properties.db to exist: %v", err)
	}
}

func TestPropertiesStore_IdempotentSchema(t *testing.T) {
	tmp := t.TempDir()
	for i := 0; i < 3; i++ {
		store, err := NewPropertiesStore(tmp)
		if err != nil {
			t.Fatalf("NewPropertiesStore (run %d): %v", i, err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close (run %d): %v", i, err)
		}
	}
}

// ─── SetPropertiesForPage ────────────────────────────────────────────────────

func TestPropertiesStore_SetPropertiesForPage_StoresEntries(t *testing.T) {
	store := newTestStore(t)

	input := props("status", "draft", "author", "alice", "environment", "staging")
	if err := store.SetPropertiesForPage("page-1", input); err != nil {
		t.Fatalf("SetPropertiesForPage: %v", err)
	}

	got, err := store.GetPropertiesForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}

	p := got["page-1"]
	if p["status"] != (PropertyEntry{Value: "draft", Type: "text"}) {
		t.Errorf("status = %+v", p["status"])
	}
	if p["author"] != (PropertyEntry{Value: "alice", Type: "text"}) {
		t.Errorf("author = %+v", p["author"])
	}
	if p["environment"] != (PropertyEntry{Value: "staging", Type: "text"}) {
		t.Errorf("environment = %+v", p["environment"])
	}
}

func TestPropertiesStore_SetPropertiesForPage_ReplacesOnSecondCall(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft", "author", "alice"))
	_ = store.SetPropertiesForPage("page-1", props("status", "published"))

	got, err := store.GetPropertiesForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}
	p := got["page-1"]
	if _, ok := p["author"]; ok {
		t.Error("author should have been replaced away")
	}
	if p["status"].Value != "published" {
		t.Errorf("status = %q, want 'published'", p["status"].Value)
	}
}

func TestPropertiesStore_SetPropertiesForPage_EmptyMapClearsExisting(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft"))
	if err := store.SetPropertiesForPage("page-1", map[string]PropertyEntry{}); err != nil {
		t.Fatalf("SetPropertiesForPage (clear): %v", err)
	}

	got, err := store.GetPropertiesForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}
	if len(got["page-1"]) != 0 {
		t.Errorf("expected empty props, got %v", got["page-1"])
	}
}

func TestPropertiesStore_SetPropertiesForPage_NilMapClearsExisting(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft"))
	if err := store.SetPropertiesForPage("page-1", nil); err != nil {
		t.Fatalf("SetPropertiesForPage (nil): %v", err)
	}

	got, err := store.GetPropertiesForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}
	if len(got["page-1"]) != 0 {
		t.Errorf("expected empty props after nil set, got %v", got["page-1"])
	}
}

// ─── DeletePropertiesForPage ─────────────────────────────────────────────────

func TestPropertiesStore_DeletePropertiesForPage_RemovesEntries(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft"))
	if err := store.DeletePropertiesForPage("page-1"); err != nil {
		t.Fatalf("DeletePropertiesForPage: %v", err)
	}

	got, err := store.GetPropertiesForPages([]string{"page-1"})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}
	if len(got["page-1"]) != 0 {
		t.Errorf("expected empty after delete, got %v", got["page-1"])
	}
}

func TestPropertiesStore_DeletePropertiesForPage_NonExistentIsNoop(t *testing.T) {
	store := newTestStore(t)
	if err := store.DeletePropertiesForPage("does-not-exist"); err != nil {
		t.Fatalf("DeletePropertiesForPage on unknown page: %v", err)
	}
}

func TestPropertiesStore_DeletePropertiesForPage_DoesNotAffectOtherPages(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft"))
	_ = store.SetPropertiesForPage("page-2", props("status", "published"))

	_ = store.DeletePropertiesForPage("page-1")

	got, err := store.GetPropertiesForPages([]string{"page-2"})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}
	if got["page-2"]["status"].Value != "published" {
		t.Errorf("page-2 status should be unaffected, got %v", got["page-2"])
	}
}

// ─── GetAllPropertyKeys ──────────────────────────────────────────────────────

func TestPropertiesStore_GetAllPropertyKeys_EmptyDB(t *testing.T) {
	store := newTestStore(t)
	keys, err := store.GetAllPropertyKeys("", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty result, got %v", keys)
	}
}

func TestPropertiesStore_GetAllPropertyKeys_ReturnsDistinctKeysWithCount(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft", "priority", "high"))
	_ = store.SetPropertiesForPage("page-2", props("status", "published"))
	_ = store.SetPropertiesForPage("page-3", props("status", "draft", "author", "alice"))

	keys, err := store.GetAllPropertyKeys("", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}

	byKey := make(map[string]int, len(keys))
	for _, kc := range keys {
		byKey[kc.Key] = kc.Count
	}

	if byKey["status"] != 3 {
		t.Errorf("status count = %d, want 3", byKey["status"])
	}
	if byKey["priority"] != 1 {
		t.Errorf("priority count = %d, want 1", byKey["priority"])
	}
	if byKey["author"] != 1 {
		t.Errorf("author count = %d, want 1", byKey["author"])
	}
}

func TestPropertiesStore_GetAllPropertyKeys_OrderByCountDescThenKeyAsc(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("alpha", "x", "beta", "x", "gamma", "x"))
	_ = store.SetPropertiesForPage("page-2", props("alpha", "x", "beta", "x"))
	_ = store.SetPropertiesForPage("page-3", props("alpha", "x"))

	keys, err := store.GetAllPropertyKeys("", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0].Key != "alpha" || keys[0].Count != 3 {
		t.Errorf("keys[0] = %+v, want {alpha 3}", keys[0])
	}
	if keys[1].Key != "beta" || keys[1].Count != 2 {
		t.Errorf("keys[1] = %+v, want {beta 2}", keys[1])
	}
	if keys[2].Key != "gamma" || keys[2].Count != 1 {
		t.Errorf("keys[2] = %+v, want {gamma 1}", keys[2])
	}
}

func TestPropertiesStore_GetAllPropertyKeys_FilterByPrefix(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "x", "stage", "x", "score", "x", "author", "x"))

	keys, err := store.GetAllPropertyKeys("st", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}

	for _, kc := range keys {
		if len(kc.Key) < 2 || kc.Key[:2] != "st" {
			t.Errorf("key %q does not start with 'st'", kc.Key)
		}
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys matching 'st', got %d: %v", len(keys), keys)
	}
}

func TestPropertiesStore_GetAllPropertyKeys_RespectsLimit(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("a", "1", "b", "2", "c", "3", "d", "4", "e", "5"))

	keys, err := store.GetAllPropertyKeys("", 3)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("expected 3 keys (limit), got %d", len(keys))
	}
}

func TestPropertiesStore_GetAllPropertyKeys_ZeroLimitReturnsAll(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("a", "1", "b", "2", "c", "3"))

	keys, err := store.GetAllPropertyKeys("", 0)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("expected all 3 keys, got %d", len(keys))
	}
}

func TestPropertiesStore_GetAllPropertyKeys_FilterEscapesLikeWildcards(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft", "stage", "alpha"))

	// "%" should match nothing (literal, not wildcard)
	keys, err := store.GetAllPropertyKeys("%", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys with %% filter: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("filter '%%' should match no keys (literal), got %v", keys)
	}

	// "_tatus" should match nothing (literal underscore)
	keys, err = store.GetAllPropertyKeys("_tatus", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys with _ filter: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("filter '_tatus' should match no keys (literal), got %v", keys)
	}
}

// ─── GetPageIDsByProperty ─────────────────────────────────────────────────────

func TestPropertiesStore_GetPageIDsByProperty_ExactMatch(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft"))
	_ = store.SetPropertiesForPage("page-2", props("status", "published"))
	_ = store.SetPropertiesForPage("page-3", props("status", "draft"))

	ids, err := store.GetPageIDsByProperty("status", "draft")
	if err != nil {
		t.Fatalf("GetPageIDsByProperty: %v", err)
	}

	sort.Strings(ids)
	want := []string{"page-1", "page-3"}
	if len(ids) != len(want) {
		t.Fatalf("expected %v, got %v", want, ids)
	}
	for i, w := range want {
		if ids[i] != w {
			t.Errorf("[%d] = %q, want %q", i, ids[i], w)
		}
	}
}

func TestPropertiesStore_GetPageIDsByProperty_NoMatch(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft"))

	ids, err := store.GetPageIDsByProperty("status", "published")
	if err != nil {
		t.Fatalf("GetPageIDsByProperty: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected no matches, got %v", ids)
	}
}

func TestPropertiesStore_GetPageIDsByProperty_KeyNotExistsReturnsEmpty(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft"))

	ids, err := store.GetPageIDsByProperty("nonexistent", "draft")
	if err != nil {
		t.Fatalf("GetPageIDsByProperty: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty for unknown key, got %v", ids)
	}
}

func TestPropertiesStore_GetPageIDsByProperty_ValueIsCaseSensitive(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "Draft"))

	// Exact case "draft" should NOT match "Draft"
	ids, err := store.GetPageIDsByProperty("status", "draft")
	if err != nil {
		t.Fatalf("GetPageIDsByProperty: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("value matching should be case-sensitive, got %v", ids)
	}
}

// ─── GetPropertiesForPages ────────────────────────────────────────────────────

func TestPropertiesStore_GetPropertiesForPages_MultiplePages(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft", "score", "10"))
	_ = store.SetPropertiesForPage("page-2", props("author", "alice"))
	_ = store.SetPropertiesForPage("page-3", props("status", "published"))

	got, err := store.GetPropertiesForPages([]string{"page-1", "page-3"})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}

	if got["page-1"]["status"].Value != "draft" {
		t.Errorf("page-1 status = %v", got["page-1"]["status"])
	}
	if _, ok := got["page-2"]; ok {
		t.Error("page-2 should not be in result")
	}
	if got["page-3"]["status"].Value != "published" {
		t.Errorf("page-3 status = %v", got["page-3"]["status"])
	}
}

func TestPropertiesStore_GetPropertiesForPages_EmptyInputReturnsEmptyMap(t *testing.T) {
	store := newTestStore(t)

	got, err := store.GetPropertiesForPages([]string{})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}
	if got == nil {
		t.Fatal("expected empty map, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestPropertiesStore_GetPropertiesForPages_UnknownIDReturnsNoEntry(t *testing.T) {
	store := newTestStore(t)

	got, err := store.GetPropertiesForPages([]string{"does-not-exist"})
	if err != nil {
		t.Fatalf("GetPropertiesForPages: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map for unknown ID, got %v", got)
	}
}

// ─── Clear ───────────────────────────────────────────────────────────────────

func TestPropertiesStore_Clear_RemovesAllEntries(t *testing.T) {
	store := newTestStore(t)

	_ = store.SetPropertiesForPage("page-1", props("status", "draft"))
	_ = store.SetPropertiesForPage("page-2", props("author", "alice"))

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	keys, err := store.GetAllPropertyKeys("", 50)
	if err != nil {
		t.Fatalf("GetAllPropertyKeys after Clear: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty after Clear, got %v", keys)
	}
}
