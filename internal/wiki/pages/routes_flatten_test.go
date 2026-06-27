package pages

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

// ─── extractPageMetadata ─────────────────────────────────────────────────────

func TestExtractPageMetadata_ReturnsTypedScalarProperties(t *testing.T) {
	fields := map[string]interface{}{
		"status": "draft",
		"count":  42,
		"active": true,
	}
	_, props := extractPageMetadata(fields)

	if props["status"].Type != tree.MetadataTypeText || props["status"].Value != "draft" {
		t.Errorf("status wrong: %+v", props["status"])
	}
	if props["count"].Type != tree.MetadataTypeNumber || props["count"].Value != "42" {
		t.Errorf("count wrong: %+v", props["count"])
	}
	if props["active"].Type != tree.MetadataTypeBoolean || props["active"].Value != "true" {
		t.Errorf("active wrong: %+v", props["active"])
	}
}

func TestExtractPageMetadata_ReturnsNestedObjectTree(t *testing.T) {
	fields := map[string]interface{}{
		"meta": map[string]interface{}{
			"author": "alice",
		},
	}
	_, props := extractPageMetadata(fields)

	mv, ok := props["meta"]
	if !ok {
		t.Fatal("expected 'meta' in properties")
	}
	if mv.Type != tree.MetadataTypeObject {
		t.Errorf("expected object type, got %q", mv.Type)
	}
	if mv.Fields["author"].Value != "alice" {
		t.Errorf("expected author=alice, got %+v", mv.Fields["author"])
	}
}

func TestExtractPageMetadata_ReturnsMixedListTree(t *testing.T) {
	fields := map[string]interface{}{
		"items": []interface{}{"a", 1},
	}
	_, props := extractPageMetadata(fields)

	mv, ok := props["items"]
	if !ok {
		t.Fatal("expected 'items' in properties")
	}
	if mv.Type != tree.MetadataTypeList {
		t.Errorf("expected list type, got %q", mv.Type)
	}
	if len(mv.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(mv.Items))
	}
}

func TestExtractPageMetadata_SkipsTagsAndSystemKeys(t *testing.T) {
	fields := map[string]interface{}{
		"tags":        []interface{}{"go", "react"},
		"leafwiki_id": "abc",
		"status":      "draft",
	}
	tags, props := extractPageMetadata(fields)

	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", tags)
	}
	if _, ok := props["tags"]; ok {
		t.Error("tags must not appear in properties")
	}
	if _, ok := props["leafwiki_id"]; ok {
		t.Error("leafwiki_ key must not appear in properties")
	}
	if props["status"].Value != "draft" {
		t.Errorf("expected status=draft, got %+v", props["status"])
	}
}

func TestExtractPageMetadata_TitleInExtraFieldsIsCustomProperty(t *testing.T) {
	fields := map[string]interface{}{
		"title":  "My Custom Title",
		"status": "draft",
	}
	_, props := extractPageMetadata(fields)
	if props["title"].Value != "My Custom Title" {
		t.Errorf("title must appear in properties, got %+v", props["title"])
	}
}

// ─── validatePageMetadataInput ───────────────────────────────────────────────

func TestValidatePageMetadataInput_RejectsReservedRootKey(t *testing.T) {
	props := map[string]tree.MetadataValue{
		"tags": {Type: tree.MetadataTypeText, Value: "x"},
	}
	err := validatePageMetadataInput(nil, props)
	if err == nil {
		t.Fatal("expected validation error for reserved key 'tags'")
	}
}

func TestValidatePageMetadataInput_RejectsLeafwikiKey(t *testing.T) {
	props := map[string]tree.MetadataValue{
		"leafwiki_id": {Type: tree.MetadataTypeText, Value: "x"},
	}
	err := validatePageMetadataInput(nil, props)
	if err == nil {
		t.Fatal("expected validation error for leafwiki_ key")
	}
}

func TestValidatePageMetadataInput_RejectsUnknownType(t *testing.T) {
	props := map[string]tree.MetadataValue{
		"status": {Type: "unknown-type", Value: "x"},
	}
	err := validatePageMetadataInput(nil, props)
	if err == nil {
		t.Fatal("expected validation error for unknown type")
	}
}

func TestValidatePageMetadataInput_RejectsListWithoutItems(t *testing.T) {
	props := map[string]tree.MetadataValue{
		"items": {Type: tree.MetadataTypeList, Items: nil},
	}
	err := validatePageMetadataInput(nil, props)
	if err == nil {
		t.Fatal("expected validation error for list without items")
	}
}

func TestValidatePageMetadataInput_RejectsObjectWithoutFields(t *testing.T) {
	props := map[string]tree.MetadataValue{
		"meta": {Type: tree.MetadataTypeObject, Fields: nil},
	}
	err := validatePageMetadataInput(nil, props)
	if err == nil {
		t.Fatal("expected validation error for object without fields")
	}
}

func TestValidatePageMetadataInput_RejectsScalarWithItems(t *testing.T) {
	props := map[string]tree.MetadataValue{
		"x": {
			Type:  tree.MetadataTypeText,
			Value: "hi",
			Items: []tree.MetadataValue{{Type: tree.MetadataTypeText, Value: "a"}},
		},
	}
	err := validatePageMetadataInput(nil, props)
	if err == nil {
		t.Fatal("expected validation error for scalar with items")
	}
}

func TestValidatePageMetadataInput_AcceptsValidTypedProperties(t *testing.T) {
	props := map[string]tree.MetadataValue{
		"text":   {Type: tree.MetadataTypeText, Value: "hello"},
		"num":    {Type: tree.MetadataTypeNumber, Value: "42"},
		"flag":   {Type: tree.MetadataTypeBoolean, Value: "true"},
		"dt":     {Type: tree.MetadataTypeDate, Value: "2024-01-15"},
		"ts":     {Type: tree.MetadataTypeDatetime, Value: "2024-01-15T12:00:00Z"},
		"nil":    {Type: tree.MetadataTypeNull},
		"list":   {Type: tree.MetadataTypeList, Items: []tree.MetadataValue{{Type: tree.MetadataTypeText, Value: "a"}}},
		"object": {Type: tree.MetadataTypeObject, Fields: map[string]tree.MetadataValue{"k": {Type: tree.MetadataTypeText, Value: "v"}}},
	}
	if err := validatePageMetadataInput(nil, props); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
