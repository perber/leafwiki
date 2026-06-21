package pages

import (
	"testing"
)

func TestFlattenMetadataEntry_FlatString(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("key", "value", result)
	if result["key"] != "value" {
		t.Errorf("expected result[key] = value, got %q", result["key"])
	}
}

func TestFlattenMetadataEntry_NestedOneLevel(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("a", map[string]interface{}{"b": "val"}, result)
	if result["a.b"] != "val" {
		t.Errorf("expected result[a.b] = val, got %q", result["a.b"])
	}
}

func TestFlattenMetadataEntry_NestedTwoLevels(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("a", map[string]interface{}{
		"b": map[string]interface{}{"c": "deep"},
	}, result)
	if result["a.b.c"] != "deep" {
		t.Errorf("expected result[a.b.c] = deep, got %q", result["a.b.c"])
	}
}

func TestFlattenMetadataEntry_SkipsEmptyStringValue(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("key", "", result)
	if _, ok := result["key"]; ok {
		t.Error("expected empty string to be skipped")
	}
}

func TestFlattenMetadataEntry_SkipsWhitespaceOnlyValue(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("key", "   ", result)
	if _, ok := result["key"]; ok {
		t.Error("expected whitespace-only string to be skipped")
	}
}

func TestFlattenMetadataEntry_SkipsValueWithNewline(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("key", "line1\nline2", result)
	if _, ok := result["key"]; ok {
		t.Error("expected multiline string to be skipped")
	}
}

func TestFlattenMetadataEntry_SkipsNonStringLeaf(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("key", 42, result)
	if _, ok := result["key"]; ok {
		t.Error("expected non-string leaf to be skipped")
	}
}

func TestFlattenMetadataEntry_SkipsLeafwikiChildSegment(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("meta", map[string]interface{}{
		"leafwiki_id": "secret",
		"visible":     "yes",
	}, result)
	if _, ok := result["meta.leafwiki_id"]; ok {
		t.Error("expected leafwiki_ child segment to be skipped")
	}
	if result["meta.visible"] != "yes" {
		t.Errorf("expected meta.visible = yes, got %q", result["meta.visible"])
	}
}

func TestFlattenMetadataEntry_SkipsEmptyChildKey(t *testing.T) {
	result := map[string]string{}
	flattenMetadataEntry("a", map[string]interface{}{
		"":  "empty-key-value",
		"b": "ok",
	}, result)
	if _, ok := result["a."]; ok {
		t.Error("expected empty child key to be skipped")
	}
	if result["a.b"] != "ok" {
		t.Errorf("expected a.b = ok, got %q", result["a.b"])
	}
}

// ─── extractPageMetadata ─────────────────────────────────────────────────────

func TestExtractPageMetadata_SkipsTagsAlways(t *testing.T) {
	fields := map[string]interface{}{
		"tags":   []interface{}{"go", "react"},
		"status": "draft",
	}
	tags, props := extractPageMetadata(fields, false)
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", tags)
	}
	if _, ok := props["tags"]; ok {
		t.Error("tags must not appear in properties")
	}
	if props["status"] != "draft" {
		t.Errorf("expected status=draft, got %q", props["status"])
	}
}

func TestExtractPageMetadata_SkipsTitleWhenNoLeafwikiTitle(t *testing.T) {
	// Alias case: "title" is the page-title alias, not a custom property.
	fields := map[string]interface{}{
		"title":  "My Page",
		"status": "draft",
	}
	_, props := extractPageMetadata(fields, false)
	if _, ok := props["title"]; ok {
		t.Error("title without leafwiki_title must not appear in properties")
	}
}

func TestExtractPageMetadata_IncludesTitleWhenLeafwikiTitlePresent(t *testing.T) {
	// Custom property case: both "title" and "leafwiki_title" exist in the file.
	fields := map[string]interface{}{
		"title":  "My Custom Title",
		"status": "draft",
	}
	_, props := extractPageMetadata(fields, true)
	if props["title"] != "My Custom Title" {
		t.Errorf("title with leafwiki_title must appear in properties, got %q", props["title"])
	}
}

func TestExtractPageMetadata_SkipsLeafwikiPrefixAlways(t *testing.T) {
	fields := map[string]interface{}{
		"leafwiki_id": "abc",
		"status":      "draft",
	}
	_, props := extractPageMetadata(fields, true)
	if _, ok := props["leafwiki_id"]; ok {
		t.Error("leafwiki_ keys must never appear in properties")
	}
}

func TestFlattenMetadataEntry_DepthLimitDoesNotPanic(t *testing.T) {
	// Build a map nested maxFlattenDepth+5 levels deep
	inner := map[string]interface{}{"leaf": "value"}
	for i := 0; i < maxFlattenDepth+5; i++ {
		inner = map[string]interface{}{"child": inner}
	}
	result := map[string]string{}
	// Must not panic; depth guard terminates recursion before stack overflow
	flattenMetadataEntry("root", inner, result)
}
