package tree

import (
	"errors"
	"testing"
)

// subtreeFixture builds:
//
//	docs (section)
//	├── guide
//	│   └── intro
//	└── faq
//	other
func subtreeFixture(t *testing.T) (svc *TreeService, ids map[string]string) {
	t.Helper()
	svc, _ = newLoadedService(t)
	ids = map[string]string{}

	section := NodeKindSection
	create := func(name string, parent string, kind *NodeKind) {
		var parentID *string
		if parent != "" {
			p := ids[parent]
			parentID = &p
		}
		id, err := svc.CreateNode("user", parentID, name, name, kind)
		if err != nil {
			t.Fatalf("CreateNode(%s) failed: %v", name, err)
		}
		ids[name] = *id
	}
	create("docs", "", &section)
	create("guide", "docs", nil)
	create("intro", "guide", nil)
	create("faq", "docs", nil)
	create("other", "", nil)
	return svc, ids
}

func TestTreeService_IsInSubtree_ReportsMembership(t *testing.T) {
	svc, ids := subtreeFixture(t)

	tests := []struct {
		name string
		id   string
		root string
		want bool
	}{
		{name: "root itself", id: "docs", root: "docs", want: true},
		{name: "child", id: "guide", root: "docs", want: true},
		{name: "grandchild", id: "intro", root: "docs", want: true},
		{name: "sibling of the root", id: "other", root: "docs", want: false},
		{name: "parent is not in the subtree of its child", id: "docs", root: "guide", want: false},
		{name: "sibling subtree", id: "faq", root: "guide", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.IsInSubtree(ids[tc.id], ids[tc.root])
			if err != nil {
				t.Fatalf("IsInSubtree failed: %v", err)
			}
			if got != tc.want {
				t.Errorf("IsInSubtree(%s, %s) = %v, want %v", tc.id, tc.root, got, tc.want)
			}
		})
	}
}

func TestTreeService_IsInSubtree_FollowsMoves(t *testing.T) {
	svc, ids := subtreeFixture(t)

	if err := svc.MoveNode("user", ids["other"], ids["faq"], VersionUnchecked); err != nil {
		t.Fatalf("MoveNode into subtree failed: %v", err)
	}
	if in, err := svc.IsInSubtree(ids["other"], ids["docs"]); err != nil || !in {
		t.Fatalf("after move in: IsInSubtree = %v, %v; want true, nil", in, err)
	}

	if err := svc.MoveNode("user", ids["other"], "root", VersionUnchecked); err != nil {
		t.Fatalf("MoveNode out of subtree failed: %v", err)
	}
	if in, err := svc.IsInSubtree(ids["other"], ids["docs"]); err != nil || in {
		t.Fatalf("after move out: IsInSubtree = %v, %v; want false, nil", in, err)
	}
}

func TestTreeService_IsInSubtree_UnknownIDsAreNotFound(t *testing.T) {
	svc, ids := subtreeFixture(t)

	tests := []struct {
		name string
		id   string
		root string
	}{
		{name: "unknown page", id: "does-not-exist", root: ids["docs"]},
		{name: "unknown root", id: ids["guide"], root: "does-not-exist"},
		{name: "empty page id", id: "", root: ids["docs"]},
		{name: "empty root id", id: ids["guide"], root: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in, err := svc.IsInSubtree(tc.id, tc.root)
			if !errors.Is(err, ErrPageNotFound) {
				t.Fatalf("err = %v, want ErrPageNotFound", err)
			}
			if in {
				t.Error("unknown ids must never report membership")
			}
		})
	}
}

func TestTreeService_IsInSubtree_TreeNotLoaded(t *testing.T) {
	svc := NewTreeService(t.TempDir())

	if _, err := svc.IsInSubtree("a", "b"); !errors.Is(err, ErrTreeNotLoaded) {
		t.Fatalf("err = %v, want ErrTreeNotLoaded", err)
	}
}
