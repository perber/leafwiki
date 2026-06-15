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
