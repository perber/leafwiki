package links

type RefactorLinkMatch struct {
	FromPageID string
	FromTitle  string
	ToPath     string
	Broken     bool
}

type RewriteRule struct {
	OldPath  string
	NewPath  string
	OldTitle string // optional: rewrite [[OldTitle]] wiki-links (rename only)
	NewTitle string
}
