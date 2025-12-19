package tree

type EnsurePathResult struct {
	Page    *PageNode
	Exists  bool
	Created []*PageNode
}
