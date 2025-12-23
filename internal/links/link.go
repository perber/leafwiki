package links

type LinkStatusResult struct {
	Backlinks       []BacklinkResultItem `json:"backlinks"`
	BrokenIncoming  []BacklinkResultItem `json:"broken_incoming"`
	Outgoings       []OutgoingResultItem `json:"outgoings"`
	BrokenOutgoings []OutgoingResultItem `json:"broken_outgoings"`
	Counts          LinkStatusCounts     `json:"counts"`
}

type LinkStatusCounts struct {
	Backlinks       int `json:"backlinks"`
	BrokenIncoming  int `json:"broken_incoming"`
	Outgoings       int `json:"outgoings"`
	BrokenOutgoings int `json:"broken_outgoings"`
}
