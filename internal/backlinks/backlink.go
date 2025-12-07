package backlinks

type Backlink struct {
	FromPageID string `json:"from_page_id"`
	ToPageID   string `json:"to_page_id"`
	FromTitle  string `json:"from_title"`
}
