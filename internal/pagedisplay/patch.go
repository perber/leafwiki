package pagedisplay

import (
	"encoding/json"
	"slices"
)

// Patch carries only the settings a caller wants to change; nil fields are
// left untouched by Update. The JSON names are the ones Settings uses, so
// PUT /api/admin/settings/display decodes its body straight into a Patch and
// a new setting needs a new field here, not in the HTTP layer.
type Patch struct {
	AlwaysShowToc    *bool   `json:"alwaysShowToc,omitempty"`
	ContentWidth     *string `json:"contentWidth,omitempty"`
	ShowSubpageCount *bool   `json:"showSubpageCount,omitempty"`
}

// IsEmpty reports whether the patch changes nothing.
func (p Patch) IsEmpty() bool { return len(p.ChangedFields()) == 0 }

// ChangedFields returns the JSON names of the fields the patch sets, sorted,
// for validation and log lines.
func (p Patch) ChangedFields() []string {
	// Patch holds only pointer fields with omitempty, so a set field is
	// exactly a key in the encoded object; Marshal cannot fail on it.
	data, _ := json.Marshal(p)
	var set map[string]json.RawMessage
	_ = json.Unmarshal(data, &set)
	var fields []string
	for name := range set {
		fields = append(fields, name)
	}
	slices.Sort(fields)
	return fields
}
