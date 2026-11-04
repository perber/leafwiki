package dto

import (
	"encoding/json"
	"testing"

	coretags "github.com/perber/wiki/internal/tags"
)

// GET /api/tags used to serialize the core type directly; the dto mapping
// must produce the same bytes, including [] for no results.
func TestToTagCounts_Marshal_MatchesCoreType(t *testing.T) {
	tests := []struct {
		name string
		in   []coretags.TagCount
		want string
	}{
		{"nil is an empty array", nil, `[]`},
		{"empty is an empty array", []coretags.TagCount{}, `[]`},
		{"populated keeps order", []coretags.TagCount{{Tag: "go", Count: 2}, {Tag: "web", Count: 1}}, `[{"tag":"go","count":2},{"tag":"web","count":1}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMarshalsTo(t, ToTagCounts(tt.in), tt.want)
			if len(tt.in) > 0 {
				assertMarshalsTo(t, tt.in, tt.want)
			}
		})
	}
}

func marshalString(t *testing.T, v any) string {
	t.Helper()
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(got)
}

func assertMarshalsTo(t *testing.T, v any, want string) {
	t.Helper()
	if got := marshalString(t, v); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
