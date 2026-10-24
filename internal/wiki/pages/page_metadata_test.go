package pages

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
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
		{
			name: "keys are trimmed and tags match case-insensitively",
			fields: map[string]interface{}{
				" TAGS ":   []interface{}{"Go", 7, "go"},
				"  owner ": "alice",
			},
			wantTags:  []string{"go"},
			wantProps: map[string]string{"owner": "alice"},
		},
		{
			name: "upper-case system key is excluded",
			fields: map[string]interface{}{
				"LEAFWIKI_ID": "abc",
				"größe":       "groß",
			},
			wantTags:  []string{},
			wantProps: map[string]string{"größe": "groß"},
		},
		{
			name:      "no fields",
			fields:    map[string]interface{}{},
			wantTags:  []string{},
			wantProps: map[string]string{},
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

// TestReadPageMetadata covers the read step shared by the page view use cases
// and the create/update/move responses, including a page whose file is gone
// by the time its metadata is read.
func TestReadPageMetadata(t *testing.T) {
	tests := []struct {
		name      string
		content   *string // nil removes the file
		pageID    string  // overrides the created page's ID when set
		wantTags  []string
		wantProps map[string]string
	}{
		{
			name:      "only tags",
			content:   ptr("---\nleafwiki_id: ID\ntags:\n  - B\n  - a\n---\n# x\n"),
			wantTags:  []string{"b", "a"}, // order kept, lower-cased
			wantProps: map[string]string{},
		},
		{
			name:      "no frontmatter",
			content:   ptr("# just markdown\n"),
			wantTags:  []string{},
			wantProps: map[string]string{},
		},
		{
			name:      "invalid frontmatter",
			content:   ptr("---\ntags: [unclosed\n---\n# x\n"),
			wantTags:  []string{},
			wantProps: map[string]string{},
		},
		{
			name:      "file removed after lookup",
			content:   nil,
			wantTags:  []string{},
			wantProps: map[string]string{},
		},
		{
			name:      "unknown page id",
			content:   ptr("# x\n"),
			pageID:    "does-not-exist",
			wantTags:  []string{},
			wantProps: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageDir := t.TempDir()
			ts := tree.NewTreeService(storageDir)
			if err := ts.LoadTree(); err != nil {
				t.Fatalf("LoadTree failed: %v", err)
			}
			kind := tree.NodeKindPage
			id, err := ts.CreateNode("user1", nil, "Meta", "meta", &kind)
			if err != nil {
				t.Fatalf("CreateNode failed: %v", err)
			}
			file := filepath.Join(storageDir, "root", "meta.md")
			if tt.content == nil {
				if err := os.Remove(file); err != nil {
					t.Fatalf("Remove failed: %v", err)
				}
			} else if err := os.WriteFile(file, []byte(*tt.content), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			pageID := *id
			if tt.pageID != "" {
				pageID = tt.pageID
			}

			tags, props := readPageMetadata(ts, pageID)
			if tags == nil || props == nil {
				t.Fatalf("tags and properties must be non-nil, got %#v / %#v", tags, props)
			}
			if !reflect.DeepEqual(tags, tt.wantTags) {
				t.Errorf("tags = %#v, want %#v", tags, tt.wantTags)
			}
			if !reflect.DeepEqual(props, tt.wantProps) {
				t.Errorf("properties = %#v, want %#v", props, tt.wantProps)
			}
		})
	}
}

func ptr(s string) *string { return &s }
