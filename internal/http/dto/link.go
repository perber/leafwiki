package dto

import (
	corelinks "github.com/perber/wiki/internal/links"
)

// LinkStatus is returned by the page link status endpoint.
type LinkStatus struct {
	Backlinks       []Backlink       `json:"backlinks"`
	BrokenIncoming  []Backlink       `json:"broken_incoming"`
	Outgoings       []OutgoingLink   `json:"outgoings"`
	BrokenOutgoings []OutgoingLink   `json:"broken_outgoings"`
	Counts          LinkStatusCounts `json:"counts"`
}

// LinkStatusCounts holds the list lengths of a LinkStatus.
type LinkStatusCounts struct {
	Backlinks       int `json:"backlinks"`
	BrokenIncoming  int `json:"broken_incoming"`
	Outgoings       int `json:"outgoings"`
	BrokenOutgoings int `json:"broken_outgoings"`
}

// Backlink is a link that points at the page.
type Backlink struct {
	FromPageID string `json:"from_page_id"`
	FromTitle  string `json:"from_title"`
	FromPath   string `json:"from_path"`
	Broken     bool   `json:"broken"`
	ToPageID   string `json:"to_page_id"`
}

// OutgoingLink is a link that starts on the page.
type OutgoingLink struct {
	ToPageID    string `json:"to_page_id"`
	ToPageTitle string `json:"to_page_title"`
	ToPath      string `json:"to_path"`
	Broken      bool   `json:"broken"`
	FromPageID  string `json:"from_page_id"`
}

// ToLinkStatus converts a link status; nil stays nil. Backlinks and
// BrokenIncoming stay nil when the input is nil, which marshals as null.
func ToLinkStatus(in *corelinks.LinkStatusResult) *LinkStatus {
	if in == nil {
		return nil
	}
	return &LinkStatus{
		Backlinks:       toBacklinks(in.Backlinks),
		BrokenIncoming:  toBacklinks(in.BrokenIncoming),
		Outgoings:       toOutgoingLinks(in.Outgoings),
		BrokenOutgoings: toOutgoingLinks(in.BrokenOutgoings),
		Counts: LinkStatusCounts{
			Backlinks:       in.Counts.Backlinks,
			BrokenIncoming:  in.Counts.BrokenIncoming,
			Outgoings:       in.Counts.Outgoings,
			BrokenOutgoings: in.Counts.BrokenOutgoings,
		},
	}
}

func toBacklinks(in []corelinks.BacklinkResultItem) []Backlink {
	if in == nil {
		return nil
	}
	out := make([]Backlink, len(in))
	for i, b := range in {
		out[i] = Backlink{
			FromPageID: b.FromPageID,
			FromTitle:  b.FromTitle,
			FromPath:   b.FromPath,
			Broken:     b.Broken,
			ToPageID:   b.ToPageID,
		}
	}
	return out
}

func toOutgoingLinks(in []corelinks.OutgoingResultItem) []OutgoingLink {
	out := make([]OutgoingLink, len(in))
	for i, o := range in {
		out[i] = OutgoingLink{
			ToPageID:    o.ToPageID,
			ToPageTitle: o.ToPageTitle,
			ToPath:      o.ToPath,
			Broken:      o.Broken,
			FromPageID:  o.FromPageID,
		}
	}
	return out
}

// BrokenLinks is returned by the broken links endpoint.
type BrokenLinks struct {
	Links []BrokenLink `json:"links"`
}

// BrokenLink is a link whose target page does not exist.
type BrokenLink struct {
	FromPageID string `json:"from_page_id"`
	FromTitle  string `json:"from_title"`
	FromPath   string `json:"from_path"`
	ToPath     string `json:"to_path"`
}

// ToBrokenLinks converts broken links; nil becomes an empty slice so the
// response is {"links":[]} rather than {"links":null}.
func ToBrokenLinks(in []corelinks.BrokenLink) BrokenLinks {
	links := make([]BrokenLink, len(in))
	for i, l := range in {
		links[i] = BrokenLink{
			FromPageID: l.FromPageID,
			FromTitle:  l.FromTitle,
			FromPath:   l.FromPath,
			ToPath:     l.ToPath,
		}
	}
	return BrokenLinks{Links: links}
}
