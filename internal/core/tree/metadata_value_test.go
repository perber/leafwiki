package tree

import (
	"testing"
	"time"
)

// ─── YamlValueToMetadataValue ────────────────────────────────────────────────

func TestYamlValueToMetadataValue_NilReturnsNull(t *testing.T) {
	got, err := YamlValueToMetadataValue(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeNull {
		t.Errorf("expected type %q, got %q", MetadataTypeNull, got.Type)
	}
}

func TestYamlValueToMetadataValue_StringReturnsText(t *testing.T) {
	got, err := YamlValueToMetadataValue("hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeText {
		t.Errorf("expected type %q, got %q", MetadataTypeText, got.Type)
	}
	if got.Value != "hello world" {
		t.Errorf("expected value %q, got %q", "hello world", got.Value)
	}
}

func TestYamlValueToMetadataValue_StringDatePatternReturnsDate(t *testing.T) {
	got, err := YamlValueToMetadataValue("2024-01-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeDate {
		t.Errorf("expected type %q, got %q", MetadataTypeDate, got.Type)
	}
	if got.Value != "2024-01-15" {
		t.Errorf("expected value %q, got %q", "2024-01-15", got.Value)
	}
}

func TestYamlValueToMetadataValue_StringDatetimePatternReturnsDatetime(t *testing.T) {
	got, err := YamlValueToMetadataValue("2024-01-15T12:30:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeDatetime {
		t.Errorf("expected type %q, got %q", MetadataTypeDatetime, got.Type)
	}
}

func TestYamlValueToMetadataValue_IntReturnsNumber(t *testing.T) {
	got, err := YamlValueToMetadataValue(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeNumber {
		t.Errorf("expected type %q, got %q", MetadataTypeNumber, got.Type)
	}
	if got.Value != "42" {
		t.Errorf("expected value %q, got %q", "42", got.Value)
	}
}

func TestYamlValueToMetadataValue_Float64ReturnsNumber(t *testing.T) {
	got, err := YamlValueToMetadataValue(3.14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeNumber {
		t.Errorf("expected type %q, got %q", MetadataTypeNumber, got.Type)
	}
	if got.Value != "3.14" {
		t.Errorf("expected value %q, got %q", "3.14", got.Value)
	}
}

func TestYamlValueToMetadataValue_BoolTrueReturnsBoolean(t *testing.T) {
	got, err := YamlValueToMetadataValue(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeBoolean {
		t.Errorf("expected type %q, got %q", MetadataTypeBoolean, got.Type)
	}
	if got.Value != "true" {
		t.Errorf("expected value %q, got %q", "true", got.Value)
	}
}

func TestYamlValueToMetadataValue_BoolFalseReturnsBoolean(t *testing.T) {
	got, err := YamlValueToMetadataValue(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeBoolean {
		t.Errorf("expected type %q, got %q", MetadataTypeBoolean, got.Type)
	}
	if got.Value != "false" {
		t.Errorf("expected value %q, got %q", "false", got.Value)
	}
}

func TestYamlValueToMetadataValue_TimeDateOnlyReturnsDate(t *testing.T) {
	// YAML parses bare dates like 2024-01-15 as time.Time at midnight UTC
	ts := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	got, err := YamlValueToMetadataValue(ts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeDate {
		t.Errorf("expected type %q, got %q", MetadataTypeDate, got.Type)
	}
	if got.Value != "2024-01-15" {
		t.Errorf("expected value %q, got %q", "2024-01-15", got.Value)
	}
}

func TestYamlValueToMetadataValue_TimestampReturnsDatetime(t *testing.T) {
	ts := time.Date(2024, 1, 15, 12, 30, 0, 0, time.UTC)
	got, err := YamlValueToMetadataValue(ts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeDatetime {
		t.Errorf("expected type %q, got %q", MetadataTypeDatetime, got.Type)
	}
}

func TestYamlValueToMetadataValue_SliceReturnsList(t *testing.T) {
	got, err := YamlValueToMetadataValue([]interface{}{"a", 1, true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeList {
		t.Errorf("expected type %q, got %q", MetadataTypeList, got.Type)
	}
	if len(got.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(got.Items))
	}
	if got.Items[0].Type != MetadataTypeText || got.Items[0].Value != "a" {
		t.Errorf("items[0] wrong: %+v", got.Items[0])
	}
	if got.Items[1].Type != MetadataTypeNumber || got.Items[1].Value != "1" {
		t.Errorf("items[1] wrong: %+v", got.Items[1])
	}
	if got.Items[2].Type != MetadataTypeBoolean || got.Items[2].Value != "true" {
		t.Errorf("items[2] wrong: %+v", got.Items[2])
	}
}

func TestYamlValueToMetadataValue_MapReturnsObject(t *testing.T) {
	m := map[string]interface{}{"author": "alice", "count": 42}
	got, err := YamlValueToMetadataValue(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != MetadataTypeObject {
		t.Errorf("expected type %q, got %q", MetadataTypeObject, got.Type)
	}
	if len(got.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(got.Fields))
	}
	if got.Fields["author"].Type != MetadataTypeText {
		t.Errorf("fields.author wrong type: %q", got.Fields["author"].Type)
	}
	if got.Fields["count"].Type != MetadataTypeNumber {
		t.Errorf("fields.count wrong type: %q", got.Fields["count"].Type)
	}
}

// ─── metadataValueToYAML ─────────────────────────────────────────────────────

func TestMetadataValueToYAML_TextReturnsString(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeText, Value: "hello"}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if s != "hello" {
		t.Errorf("expected %q, got %q", "hello", s)
	}
}

func TestMetadataValueToYAML_NumberIntReturnsInt64(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeNumber, Value: "42"}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n, ok := got.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T: %v", got, got)
	}
	if n != 42 {
		t.Errorf("expected 42, got %d", n)
	}
}

func TestMetadataValueToYAML_NumberFloatReturnsFloat64(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeNumber, Value: "3.14"}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T: %v", got, got)
	}
	if f != 3.14 {
		t.Errorf("expected 3.14, got %v", f)
	}
}

func TestMetadataValueToYAML_InvalidNumberReturnsError(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeNumber, Value: "not-a-number"}
	_, err := metadataValueToYAML(v)
	if err == nil {
		t.Fatal("expected error for invalid number, got nil")
	}
}

func TestMetadataValueToYAML_BooleanTrueReturnsBool(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeBoolean, Value: "true"}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, ok := got.(bool)
	if !ok {
		t.Fatalf("expected bool, got %T", got)
	}
	if !b {
		t.Errorf("expected true, got false")
	}
}

func TestMetadataValueToYAML_BooleanFalseReturnsBool(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeBoolean, Value: "false"}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, ok := got.(bool)
	if !ok {
		t.Fatalf("expected bool, got %T", got)
	}
	if b {
		t.Errorf("expected false, got true")
	}
}

func TestMetadataValueToYAML_DateReturnsString(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeDate, Value: "2024-01-15"}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if s != "2024-01-15" {
		t.Errorf("expected %q, got %q", "2024-01-15", s)
	}
}

func TestMetadataValueToYAML_DatetimeReturnsString(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeDatetime, Value: "2024-01-15T12:30:00Z"}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if s != "2024-01-15T12:30:00Z" {
		t.Errorf("expected RFC3339, got %q", s)
	}
}

func TestMetadataValueToYAML_NullReturnsNil(t *testing.T) {
	v := MetadataValue{Type: MetadataTypeNull}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestMetadataValueToYAML_ListReturnsSlice(t *testing.T) {
	v := MetadataValue{
		Type: MetadataTypeList,
		Items: []MetadataValue{
			{Type: MetadataTypeText, Value: "a"},
			{Type: MetadataTypeNumber, Value: "1"},
		},
	}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slice, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", got)
	}
	if len(slice) != 2 {
		t.Fatalf("expected 2 items, got %d", len(slice))
	}
	if slice[0] != "a" {
		t.Errorf("expected 'a', got %v", slice[0])
	}
	if slice[1] != int64(1) {
		t.Errorf("expected int64(1), got %T(%v)", slice[1], slice[1])
	}
}

func TestMetadataValueToYAML_ObjectReturnsMap(t *testing.T) {
	v := MetadataValue{
		Type: MetadataTypeObject,
		Fields: map[string]MetadataValue{
			"author": {Type: MetadataTypeText, Value: "alice"},
		},
	}
	got, err := metadataValueToYAML(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", got)
	}
	if m["author"] != "alice" {
		t.Errorf("expected author=alice, got %v", m["author"])
	}
}

func TestMetadataValueToYAML_UnknownTypeReturnsError(t *testing.T) {
	v := MetadataValue{Type: "unknown-type", Value: "x"}
	_, err := metadataValueToYAML(v)
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
}

// ─── IsValidMetadataType ──────────────────────────────────────────────────────

func TestIsValidMetadataType_KnownTypesReturnTrue(t *testing.T) {
	known := []string{
		MetadataTypeText, MetadataTypeNumber, MetadataTypeBoolean,
		MetadataTypeDate, MetadataTypeDatetime, MetadataTypeNull,
		MetadataTypeList, MetadataTypeObject,
	}
	for _, typ := range known {
		if !IsValidMetadataType(typ) {
			t.Errorf("expected %q to be valid", typ)
		}
	}
}

func TestIsValidMetadataType_UnknownTypeReturnsFalse(t *testing.T) {
	if IsValidMetadataType("unknown") {
		t.Error("expected 'unknown' to be invalid")
	}
	if IsValidMetadataType("") {
		t.Error("expected empty string to be invalid")
	}
}

// ─── IsScalar ─────────────────────────────────────────────────────────────────

func TestIsScalar_ScalarTypesReturnTrue(t *testing.T) {
	scalars := []string{
		MetadataTypeText, MetadataTypeNumber, MetadataTypeBoolean,
		MetadataTypeDate, MetadataTypeDatetime, MetadataTypeNull,
	}
	for _, typ := range scalars {
		mv := MetadataValue{Type: typ}
		if !mv.IsScalar() {
			t.Errorf("expected %q to be scalar", typ)
		}
	}
}

func TestIsScalar_ContainerTypesReturnFalse(t *testing.T) {
	containers := []string{MetadataTypeList, MetadataTypeObject}
	for _, typ := range containers {
		mv := MetadataValue{Type: typ}
		if mv.IsScalar() {
			t.Errorf("expected %q to not be scalar", typ)
		}
	}
}
