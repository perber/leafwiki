package scopedread

import (
	"context"
	"errors"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

// TestPageReader_Page_RealTreeLifecycle exercises the reader against a real
// TreeService: moves in and out, deletion of pages and of the scope root,
// the tree root ID, and a tree that is not loaded.
func TestPageReader_Page_RealTreeLifecycle(t *testing.T) {
	svc := tree.NewTreeService(t.TempDir())
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	section := tree.NodeKindSection
	mk := func(title string, parent *string, kind *tree.NodeKind) string {
		id, err := svc.CreateNode("u", parent, title, title, kind)
		if err != nil {
			t.Fatalf("CreateNode(%s) failed: %v", title, err)
		}
		return *id
	}
	docs := mk("docs", nil, &section)
	guide := mk("guide", &docs, nil)
	other := mk("other", nil, nil)

	r := NewPageReader(svc)
	ctx := context.Background()
	sub := ReadScope{RootID: docs, IncludeDescendants: true}

	read := func(scope ReadScope, id string) error {
		_, err := r.Page(ctx, scope, id)
		return err
	}

	if err := read(sub, guide); err != nil {
		t.Fatalf("child in scope: %v", err)
	}
	if err := read(sub, other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sibling: err = %v, want ErrNotFound", err)
	}
	// the tree root ID is not a page inside any scope
	for _, scope := range []ReadScope{sub, {RootID: docs}} {
		if err := read(scope, "root"); !errors.Is(err, ErrNotFound) {
			t.Errorf("scope %+v, id root: err = %v, want ErrNotFound", scope, err)
		}
	}
	// path-ish and padded ids never match
	for _, id := range []string{"../" + docs, " " + docs, docs + "/" + guide} {
		if err := read(sub, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("id %q: err = %v, want ErrNotFound", id, err)
		}
	}

	// moves take effect immediately
	if err := svc.MoveNode("u", other, docs, tree.VersionUnchecked); err != nil {
		t.Fatalf("MoveNode in: %v", err)
	}
	if err := read(sub, other); err != nil {
		t.Errorf("after move in: %v", err)
	}
	if err := svc.MoveNode("u", guide, "root", tree.VersionUnchecked); err != nil {
		t.Fatalf("MoveNode out: %v", err)
	}
	if err := read(sub, guide); !errors.Is(err, ErrNotFound) {
		t.Errorf("after move out: err = %v, want ErrNotFound", err)
	}

	// deleted page and deleted scope root
	if err := svc.DeleteNode("u", other, false, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode page: %v", err)
	}
	if err := read(sub, other); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted page: err = %v, want ErrNotFound", err)
	}
	if err := svc.DeleteNode("u", docs, true, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode root: %v", err)
	}
	for _, scope := range []ReadScope{sub, {RootID: docs}} {
		if err := read(scope, docs); !errors.Is(err, ErrNotFound) {
			t.Errorf("deleted root, scope %+v: err = %v, want ErrNotFound", scope, err)
		}
	}
}

func TestPageReader_Page_RealTreeNotLoaded(t *testing.T) {
	r := NewPageReader(tree.NewTreeService(t.TempDir()))
	for _, scope := range []ReadScope{{RootID: "a"}, {RootID: "a", IncludeDescendants: true}} {
		_, err := r.Page(context.Background(), scope, "a")
		if !errors.Is(err, tree.ErrTreeNotLoaded) {
			t.Errorf("scope %+v: err = %v, want ErrTreeNotLoaded", scope, err)
		}
	}
}
