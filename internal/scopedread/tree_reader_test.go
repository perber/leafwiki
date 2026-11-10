package scopedread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/tree"
)

// fakeTree serves SubtreeRoutes from a fixed list: the subtree of "docs" in
// newFakeSource's shape, with route paths and timestamps that must never
// reach a TreeNodeView.
type fakeTree struct {
	entries []tree.RouteEntry
	err     error
	gotID   string
}

func newFakeTree() *fakeTree {
	at := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	return &fakeTree{entries: []tree.RouteEntry{
		{ID: "docs", ParentID: "company", Title: "Docs", RoutePath: "company/docs", Kind: tree.NodeKindSection, UpdatedAt: at},
		{ID: "guide", ParentID: "docs", Title: "Guide", RoutePath: "company/docs/guide", Kind: tree.NodeKindPage, UpdatedAt: at},
		{ID: "intro", ParentID: "guide", Title: "Intro", RoutePath: "company/docs/guide/intro", Kind: tree.NodeKindPage, UpdatedAt: at},
		{ID: "faq", ParentID: "docs", Title: "FAQ", RoutePath: "company/docs/faq", Kind: tree.NodeKindPage, UpdatedAt: at},
	}}
}

func (f *fakeTree) SubtreeRoutes(id string) ([]tree.RouteEntry, error) {
	f.gotID = id
	return f.entries, f.err
}

// outline renders a view as "Docs(Guide(Intro),FAQ)" for compact comparisons.
func outline(v *TreeNodeView) string {
	if len(v.Children) == 0 {
		return v.Title
	}
	parts := make([]string, len(v.Children))
	for i, c := range v.Children {
		parts[i] = outline(c)
	}
	return v.Title + "(" + strings.Join(parts, ",") + ")"
}

func TestTreeReader_Tree_NestsTheScope(t *testing.T) {
	tests := []struct {
		name  string
		scope ReadScope
		want  string
	}{
		{name: "subtree link", scope: ReadScope{RootID: "docs", IncludeDescendants: true}, want: "Docs(Guide(Intro),FAQ)"},
		{name: "single-page link shows no subpages", scope: ReadScope{RootID: "docs"}, want: "Docs"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := newFakeTree()
			got, err := NewTreeReader(src).Tree(context.Background(), tc.scope)
			if err != nil {
				t.Fatalf("Tree: %v", err)
			}
			if src.gotID != "docs" {
				t.Errorf("read subtree of %q, want docs", src.gotID)
			}
			if outline(got) != tc.want {
				t.Errorf("tree = %s, want %s", outline(got), tc.want)
			}
		})
	}
}

func TestTreeReader_Tree_ProjectsOnlyAllowListedFields(t *testing.T) {
	got, err := NewTreeReader(newFakeTree()).Tree(context.Background(), ReadScope{RootID: "docs"})
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// children is [] rather than null, so a client needs no null check
	if want := `{"id":"docs","title":"Docs","kind":"section","children":[]}`; string(body) != want {
		t.Errorf("JSON = %s, want %s", body, want)
	}
	for _, leak := range []string{"company", "2026", "parent"} {
		if strings.Contains(strings.ToLower(string(body)), leak) {
			t.Errorf("JSON %s contains %q", body, leak)
		}
	}
}

// TestTreeNodeView_FieldsAreAllowListed fails when a field is added to
// TreeNodeView. Like PageView (ADR-0015), a new field is visible to anyone
// holding a share link; a slug or path would reveal the tree above the scope.
func TestTreeNodeView_FieldsAreAllowListed(t *testing.T) {
	allowed := []string{"children", "id", "kind", "title"}

	var got []string
	typ := reflect.TypeOf(TreeNodeView{})
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		got = append(got, name)
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, allowed) {
		t.Errorf("TreeNodeView JSON fields = %v, want exactly %v", got, allowed)
	}
}

func TestTreeReader_Tree_Errors(t *testing.T) {
	disk := errors.New("tree not loaded")
	tests := []struct {
		name    string
		scope   ReadScope
		srcErr  error
		wantErr error
	}{
		{name: "empty root", scope: ReadScope{}, wantErr: ErrInvalidScope},
		{name: "tree root", scope: ReadScope{RootID: "root", IncludeDescendants: true}, wantErr: ErrInvalidScope},
		{name: "padded root", scope: ReadScope{RootID: " docs"}, wantErr: ErrInvalidScope},
		{name: "unknown or deleted root", scope: ReadScope{RootID: "docs"}, srcErr: tree.ErrPageNotFound, wantErr: ErrNotFound},
		{name: "operational error passes through", scope: ReadScope{RootID: "docs"}, srcErr: disk, wantErr: disk},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := newFakeTree()
			src.err = tc.srcErr
			got, err := NewTreeReader(src).Tree(context.Background(), tc.scope)
			if !errors.Is(err, tc.wantErr) || got != nil {
				t.Fatalf("Tree = %v, %v, want nil, %v", got, err, tc.wantErr)
			}
			if tc.wantErr == ErrInvalidScope && src.gotID != "" {
				t.Errorf("read the tree for an invalid scope")
			}
		})
	}
}

func TestTreeReader_Tree_ReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := newFakeTree()
	if _, err := NewTreeReader(src).Tree(ctx, ReadScope{RootID: "docs"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if src.gotID != "" {
		t.Errorf("read the tree after the context was cancelled")
	}
}

// TestTreeReader_Tree_RealTreeLifecycle runs the reader against a real
// TreeService: nothing above or beside the scope, pages created or moved
// below the root appear, pages moved out disappear, and a deleted root is
// ErrNotFound.
func TestTreeReader_Tree_RealTreeLifecycle(t *testing.T) {
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
	company := mk("company", nil, &section)
	docs := mk("docs", &company, &section)
	guide := mk("guide", &docs, nil)
	mk("sibling", &company, nil)
	other := mk("other", nil, nil)

	r := NewTreeReader(svc)
	sub := ReadScope{RootID: docs, IncludeDescendants: true}
	read := func(scope ReadScope) string {
		t.Helper()
		v, err := r.Tree(context.Background(), scope)
		if err != nil {
			t.Fatalf("Tree(%+v): %v", scope, err)
		}
		return outline(v)
	}

	if got := read(sub); got != "docs(guide)" {
		t.Fatalf("subtree = %s, want docs(guide)", got)
	}
	if got := read(ReadScope{RootID: docs}); got != "docs" {
		t.Errorf("single page = %s, want docs", got)
	}

	mk("later", &guide, nil)
	if err := svc.MoveNode("u", other, docs, tree.VersionUnchecked); err != nil {
		t.Fatalf("MoveNode in: %v", err)
	}
	if got := read(sub); got != "docs(guide(later),other)" {
		t.Errorf("after create and move in = %s, want docs(guide(later),other)", got)
	}
	if err := svc.MoveNode("u", guide, company, tree.VersionUnchecked); err != nil {
		t.Fatalf("MoveNode out: %v", err)
	}
	if got := read(sub); got != "docs(other)" {
		t.Errorf("after move out = %s, want docs(other)", got)
	}

	if err := svc.DeleteNode("u", docs, true, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	for _, scope := range []ReadScope{sub, {RootID: docs}} {
		if _, err := r.Tree(context.Background(), scope); !errors.Is(err, ErrNotFound) {
			t.Errorf("deleted root, scope %+v: err = %v, want ErrNotFound", scope, err)
		}
	}
}

func TestTreeReader_Tree_RealTreeNotLoaded(t *testing.T) {
	r := NewTreeReader(tree.NewTreeService(t.TempDir()))
	if _, err := r.Tree(context.Background(), ReadScope{RootID: "a"}); !errors.Is(err, tree.ErrTreeNotLoaded) {
		t.Errorf("err = %v, want ErrTreeNotLoaded", err)
	}
}

// The shared root itself changes: a page leaves the scope upwards, children
// are sorted, the root moves to the top level, is renamed and changes kind.
// The root stays on top and nothing of its old ancestors shows up.
func TestTreeReader_Tree_RealTreeSharedRootChanges(t *testing.T) {
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
	must := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	top := mk("top", nil, &section)
	docs := mk("docs", &top, &section)
	guide := mk("guide", &docs, nil)
	faq := mk("faq", &docs, nil)

	r := NewTreeReader(svc)
	sub := ReadScope{RootID: docs, IncludeDescendants: true}
	read := func() *TreeNodeView {
		t.Helper()
		v, err := r.Tree(context.Background(), sub)
		if err != nil {
			t.Fatalf("Tree: %v", err)
		}
		return v
	}

	must("move above the root", svc.MoveNode("u", faq, top, tree.VersionUnchecked))
	if got := outline(read()); got != "docs(guide)" {
		t.Errorf("after a move above the root = %s, want docs(guide)", got)
	}
	must("move back", svc.MoveNode("u", faq, docs, tree.VersionUnchecked))
	must("sort", svc.SortPages(docs, []string{faq, guide}))
	if got := outline(read()); got != "docs(faq,guide)" {
		t.Errorf("after SortPages = %s, want docs(faq,guide)", got)
	}
	must("move the root", svc.MoveNode("u", docs, "root", tree.VersionUnchecked))
	if v := read(); v.ID != docs || outline(v) != "docs(faq,guide)" {
		t.Errorf("after moving the root = %s, want docs(faq,guide)", outline(v))
	}
	must("rename", svc.UpdateNode(tree.UpdateNodeInput{UserID: "u", ID: docs, Title: "Handbook", Slug: "handbook", ExpectedVersion: tree.VersionUnchecked}))
	if v := read(); v.Title != "Handbook" || v.Kind != "section" {
		t.Errorf("after rename = %q %q, want Handbook section", v.Title, v.Kind)
	}
	for _, id := range []string{faq, guide} {
		must("delete child", svc.DeleteNode("u", id, false, tree.VersionUnchecked))
	}
	must("convert", svc.ConvertNode("u", docs, tree.NodeKindPage, tree.VersionUnchecked))
	if v := read(); v.Kind != "page" || len(v.Children) != 0 {
		t.Errorf("after convert = %q with %d children, want a page without children", v.Kind, len(v.Children))
	}
	if _, err := r.Tree(context.Background(), ReadScope{RootID: top + "/.."}); !errors.Is(err, ErrNotFound) {
		t.Errorf("path-like root: err = %v, want ErrNotFound", err)
	}
}

// A deep chain and a wide level nest completely and keep their order. The
// source is a fake, so no files are written.
func TestTreeReader_Tree_DeepAndWideTrees(t *testing.T) {
	const depth, width = 1000, 200
	src := &fakeTree{entries: []tree.RouteEntry{{ID: "top", ParentID: "root", Title: "top", Kind: tree.NodeKindSection}}}
	parent := "top"
	for i := 0; i < depth; i++ {
		id := fmt.Sprintf("d%d", i)
		src.entries = append(src.entries, tree.RouteEntry{ID: id, ParentID: parent, Title: id, Kind: tree.NodeKindSection})
		parent = id
	}
	for i := 0; i < width; i++ {
		id := fmt.Sprintf("w%04d", i)
		src.entries = append(src.entries, tree.RouteEntry{ID: id, ParentID: "top", Title: id, Kind: tree.NodeKindPage})
	}

	v, err := NewTreeReader(src).Tree(context.Background(), ReadScope{RootID: "top", IncludeDescendants: true})
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if len(v.Children) != 1+width {
		t.Fatalf("top has %d children, want %d", len(v.Children), 1+width)
	}
	for i, c := range v.Children[1:] {
		if c.Title != fmt.Sprintf("w%04d", i) {
			t.Fatalf("child %d = %s, order broken", i, c.Title)
		}
	}
	levels := 0
	for n := v.Children[0]; ; n = n.Children[0] {
		levels++
		if len(n.Children) == 0 {
			break
		}
	}
	if levels != depth {
		t.Errorf("chain depth = %d, want %d", levels, depth)
	}
}
