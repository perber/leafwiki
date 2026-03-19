package links

type RefactorLinkMatch struct {
	FromPageID string
	FromTitle  string
	ToPath     string
	Broken     bool
}

type RewriteRule struct {
	OldPath string
	NewPath string
}
