package dto

import (
	"testing"

	corelinks "github.com/perber/wiki/internal/links"
)

// GET /api/pages/{id}/links used to serialize the core type directly; the dto
// mapping must produce the same bytes.
func TestToLinkStatus_Marshal_MatchesCoreType(t *testing.T) {
	in := &corelinks.LinkStatusResult{
		Backlinks:       []corelinks.BacklinkResultItem{{FromPageID: "a", FromTitle: "A", FromPath: "a", ToPageID: "b"}},
		BrokenIncoming:  []corelinks.BacklinkResultItem{{FromPageID: "c", FromTitle: "C", FromPath: "c", Broken: true, ToPageID: "b"}},
		Outgoings:       []corelinks.OutgoingResultItem{{ToPageID: "d", ToPageTitle: "D", ToPath: "d", FromPageID: "b"}},
		BrokenOutgoings: []corelinks.OutgoingResultItem{{ToPath: "gone", Broken: true, FromPageID: "b"}},
		Counts:          corelinks.LinkStatusCounts{Backlinks: 1, BrokenIncoming: 1, Outgoings: 1, BrokenOutgoings: 1},
	}

	assertMarshalsTo(t, ToLinkStatus(in), marshalString(t, in))
}

func TestToLinkStatus_EmptyLists(t *testing.T) {
	in := &corelinks.LinkStatusResult{
		Outgoings:       []corelinks.OutgoingResultItem{},
		BrokenOutgoings: []corelinks.OutgoingResultItem{},
	}

	// backlinks and broken_incoming stay null until the next commit flips them.
	assertMarshalsTo(t, ToLinkStatus(in),
		`{"backlinks":null,"broken_incoming":null,"outgoings":[],"broken_outgoings":[],"counts":{"backlinks":0,"broken_incoming":0,"outgoings":0,"broken_outgoings":0}}`)
	assertMarshalsTo(t, in, marshalString(t, ToLinkStatus(in)))
}

func TestToLinkStatus_NilOutgoings_AreArrays(t *testing.T) {
	got := marshalString(t, ToLinkStatus(&corelinks.LinkStatusResult{}))
	want := `{"backlinks":null,"broken_incoming":null,"outgoings":[],"broken_outgoings":[],"counts":{"backlinks":0,"broken_incoming":0,"outgoings":0,"broken_outgoings":0}}`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestToLinkStatus_Nil_MarshalsAsNull(t *testing.T) {
	assertMarshalsTo(t, ToLinkStatus(nil), `null`)
}

// GET /api/links/broken used to serialize links.GetBrokenLinksOutput directly.
func TestToBrokenLinks_Marshal(t *testing.T) {
	tests := []struct {
		name string
		in   []corelinks.BrokenLink
		want string
	}{
		{"nil is an empty array", nil, `{"links":[]}`},
		{"empty is an empty array", []corelinks.BrokenLink{}, `{"links":[]}`},
		{"populated keeps order", []corelinks.BrokenLink{
			{FromPageID: "a", FromTitle: "A", FromPath: "a", ToPath: "/gone"},
			{FromPageID: "b", FromTitle: "B", FromPath: "b", ToPath: "wikilink:X"},
		}, `{"links":[{"from_page_id":"a","from_title":"A","from_path":"a","to_path":"/gone"},{"from_page_id":"b","from_title":"B","from_path":"b","to_path":"wikilink:X"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMarshalsTo(t, ToBrokenLinks(tt.in), tt.want)
		})
	}
}
