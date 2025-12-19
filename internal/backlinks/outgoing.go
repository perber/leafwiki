package backlinks

type Outgoing struct {
	FromPageID string
	ToPageID   string
	FromTitle  string
	ToPath     string // Path of the target page
	Broken     bool   // Indicates if the link is broken
}

type OutgoingResult struct {
	Outgoings []OutgoingResultItem `json:"outgoings"`
	Count     int                  `json:"count"`
}

type OutgoingResultItem struct {
	ToPageID    string `json:"to_page_id"`
	ToPageTitle string `json:"to_page_title"`
	ToPath      string `json:"to_path"`
	Broken      bool   `json:"broken"`
	FromPageID  string `json:"from_page_id"`
}
