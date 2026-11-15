package pagedisplay

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestPatch_IsEmptyAndChangedFields(t *testing.T) {
	tests := []struct {
		name  string
		patch Patch
		empty bool
		want  []string
	}{
		{name: "no field", patch: Patch{}, empty: true, want: nil},
		{name: "only alwaysShowToc", patch: Patch{AlwaysShowToc: ptr(false)}, want: []string{"alwaysShowToc"}},
		{name: "only contentWidth", patch: Patch{ContentWidth: ptr("")}, want: []string{"contentWidth"}},
		{name: "both fields", patch: Patch{AlwaysShowToc: ptr(true), ContentWidth: ptr(ContentWidthWide)}, want: []string{"alwaysShowToc", "contentWidth"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.patch.IsEmpty(); got != tc.empty {
				t.Fatalf("IsEmpty() = %v, want %v", got, tc.empty)
			}
			if got := tc.patch.ChangedFields(); !slices.Equal(got, tc.want) {
				t.Fatalf("ChangedFields() = %v, want %v", got, tc.want)
			}
		})
	}
}

// The display route decodes its body straight into Patch, so its JSON names
// must be exactly the names Settings uses in /api/config.
func TestPatch_JSONNamesMatchSettings(t *testing.T) {
	names := func(v any) []string {
		var out []string
		typ := reflect.TypeOf(v)
		for i := range typ.NumField() {
			name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			out = append(out, name)
		}
		slices.Sort(out)
		return out
	}
	if got, want := names(Patch{}), names(Settings{}); !slices.Equal(got, want) {
		t.Fatalf("Patch JSON names = %v, Settings JSON names = %v", got, want)
	}
}

func TestPatch_DecodesSettingsJSON(t *testing.T) {
	var p Patch
	if err := json.Unmarshal([]byte(`{"alwaysShowToc":true,"contentWidth":"wide"}`), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.AlwaysShowToc == nil || !*p.AlwaysShowToc || p.ContentWidth == nil || *p.ContentWidth != ContentWidthWide {
		t.Fatalf("decoded patch = %+v", p)
	}
}
