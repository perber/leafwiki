package pages

import (
	"reflect"
	"testing"
)

func TestFlattenMetadataEntry(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		value  interface{}
		want   map[string]string
	}{
		{
			name:   "flat string",
			prefix: "key",
			value:  "value",
			want:   map[string]string{"key": "value"},
		},
		{
			name:   "nested one level",
			prefix: "a",
			value:  map[string]interface{}{"b": "val"},
			want:   map[string]string{"a.b": "val"},
		},
		{
			name:   "nested two levels",
			prefix: "a",
			value: map[string]interface{}{
				"b": map[string]interface{}{"c": "deep"},
			},
			want: map[string]string{"a.b.c": "deep"},
		},
		{
			name:   "skips empty string value",
			prefix: "key",
			value:  "",
			want:   map[string]string{},
		},
		{
			name:   "skips whitespace-only value",
			prefix: "key",
			value:  "   ",
			want:   map[string]string{},
		},
		{
			name:   "skips value with newline",
			prefix: "key",
			value:  "line1\nline2",
			want:   map[string]string{},
		},
		{
			name:   "skips non-string leaf",
			prefix: "key",
			value:  42,
			want:   map[string]string{},
		},
		{
			name:   "skips leafwiki_ child segment",
			prefix: "meta",
			value: map[string]interface{}{
				"leafwiki_id": "secret",
				"visible":     "yes",
			},
			want: map[string]string{"meta.visible": "yes"},
		},
		{
			name:   "skips empty child key",
			prefix: "a",
			value: map[string]interface{}{
				"":  "empty-key-value",
				"b": "ok",
			},
			want: map[string]string{"a.b": "ok"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := map[string]string{}
			flattenMetadataEntry(tt.prefix, tt.value, result)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("flattenMetadataEntry(%q) = %#v, want %#v", tt.prefix, result, tt.want)
			}
		})
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

func TestExtractPageMetadata(t *testing.T) {
	tests := []struct {
		name      string
		fields    map[string]interface{}
		wantTags  []string
		wantProps map[string]string
	}{
		{
			name: "tags never appear in properties",
			fields: map[string]interface{}{
				"tags":   []interface{}{"go", "react"},
				"status": "draft",
			},
			wantTags:  []string{"go", "react"},
			wantProps: map[string]string{"status": "draft"},
		},
		{
			// "title" always lands in ExtraFields and is always treated as a
			// custom property by extractPageMetadata.
			name: "title in extra fields is a custom property",
			fields: map[string]interface{}{
				"title":  "My Custom Title",
				"status": "draft",
			},
			wantTags:  []string{},
			wantProps: map[string]string{"title": "My Custom Title", "status": "draft"},
		},
		{
			name: "leafwiki_ keys never appear in properties",
			fields: map[string]interface{}{
				"leafwiki_id": "abc",
				"status":      "draft",
			},
			wantTags:  []string{},
			wantProps: map[string]string{"status": "draft"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags, props := extractPageMetadata(tt.fields)
			if !reflect.DeepEqual(tags, tt.wantTags) {
				t.Errorf("tags = %#v, want %#v", tags, tt.wantTags)
			}
			if !reflect.DeepEqual(props, tt.wantProps) {
				t.Errorf("properties = %#v, want %#v", props, tt.wantProps)
			}
		})
	}
}
