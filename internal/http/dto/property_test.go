package dto

import (
	"testing"

	coreprop "github.com/perber/wiki/internal/properties"
)

// GET /api/properties used to serialize the core type directly; the dto
// mapping must produce the same bytes, including [] for no results.
func TestToPropertyKeyCounts_Marshal_MatchesCoreType(t *testing.T) {
	tests := []struct {
		name string
		in   []coreprop.PropertyKeyCount
		want string
	}{
		{"nil is an empty array", nil, `[]`},
		{"empty is an empty array", []coreprop.PropertyKeyCount{}, `[]`},
		{"populated keeps order", []coreprop.PropertyKeyCount{{Key: "status", Count: 3}, {Key: "owner", Count: 1}}, `[{"key":"status","count":3},{"key":"owner","count":1}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMarshalsTo(t, ToPropertyKeyCounts(tt.in), tt.want)
			if len(tt.in) > 0 {
				assertMarshalsTo(t, tt.in, tt.want)
			}
		})
	}
}
