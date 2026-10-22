package scopedread

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/tree"
)

// fakeSource is a hand-built tree: parents maps a page ID to its parent ID
// ("" for top level). It records which pages had their content read, so tests
// can prove the scope check happens before any content is touched.
type fakeSource struct {
	parents map[string]string
	read    []string
	pageErr error
}

func newFakeSource() *fakeSource {
	// docs
	// ├── guide
	// │   └── intro
	// └── faq
	// other
	return &fakeSource{parents: map[string]string{
		"docs":  "",
		"guide": "docs",
		"intro": "guide",
		"faq":   "docs",
		"other": "",
	}}
}

func (f *fakeSource) IsInSubtree(id, rootID string) (bool, error) {
	if _, ok := f.parents[id]; !ok {
		return false, tree.ErrPageNotFound
	}
	if _, ok := f.parents[rootID]; !ok {
		return false, tree.ErrPageNotFound
	}
	for n := id; n != ""; n = f.parents[n] {
		if n == rootID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeSource) GetPage(id string) (*tree.Page, error) {
	f.read = append(f.read, id)
	if f.pageErr != nil {
		return nil, f.pageErr
	}
	if _, ok := f.parents[id]; !ok {
		return nil, tree.ErrPageNotFound
	}
	return &tree.Page{
		PageNode: &tree.PageNode{
			ID:    id,
			Title: "Title " + id,
			Slug:  "slug-" + id,
			Kind:  tree.NodeKindPage,
			Metadata: tree.PageMetadata{
				CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
				CreatorID:    "creator-user",
				LastAuthorID: "author-user",
			},
		},
		Content:    "body of " + id,
		RawContent: "---\ntags: [secret]\n---\nbody of " + id,
	}, nil
}

func TestPageReader_Page_EnforcesScope(t *testing.T) {
	single := ReadScope{RootID: "docs"}
	subtree := ReadScope{RootID: "docs", IncludeDescendants: true}

	tests := []struct {
		name    string
		scope   ReadScope
		id      string
		wantErr error
	}{
		{name: "single page: the root", scope: single, id: "docs"},
		{name: "single page: a child is out of scope", scope: single, id: "guide", wantErr: ErrNotFound},
		{name: "single page: another page is out of scope", scope: single, id: "other", wantErr: ErrNotFound},
		{name: "subtree: the root", scope: subtree, id: "docs"},
		{name: "subtree: a child", scope: subtree, id: "guide"},
		{name: "subtree: a grandchild", scope: subtree, id: "intro"},
		{name: "subtree: a sibling of the root", scope: subtree, id: "other", wantErr: ErrNotFound},
		{name: "subtree: the parent of the root", scope: ReadScope{RootID: "guide", IncludeDescendants: true}, id: "docs", wantErr: ErrNotFound},
		{name: "subtree: an unknown page", scope: subtree, id: "missing", wantErr: ErrNotFound},
		{name: "empty page id", scope: subtree, id: "", wantErr: ErrNotFound},
		{name: "deleted root, single page", scope: ReadScope{RootID: "gone"}, id: "gone", wantErr: ErrNotFound},
		{name: "deleted root, subtree", scope: ReadScope{RootID: "gone", IncludeDescendants: true}, id: "guide", wantErr: ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := newFakeSource()
			view, err := NewPageReader(src).Page(context.Background(), tc.scope, tc.id)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				if view != nil {
					t.Errorf("view = %+v, want nil on error", view)
				}
				return
			}
			if err != nil {
				t.Fatalf("Page failed: %v", err)
			}
			if view.ID != tc.id {
				t.Errorf("view.ID = %q, want %q", view.ID, tc.id)
			}
		})
	}
}

func TestPageReader_Page_ChecksScopeBeforeReadingContent(t *testing.T) {
	tests := []struct {
		name  string
		scope ReadScope
		id    string
	}{
		{name: "single page, other id", scope: ReadScope{RootID: "docs"}, id: "guide"},
		{name: "subtree, sibling", scope: ReadScope{RootID: "docs", IncludeDescendants: true}, id: "other"},
		{name: "invalid scope", scope: ReadScope{}, id: "docs"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := newFakeSource()
			_, _ = NewPageReader(src).Page(context.Background(), tc.scope, tc.id)
			if len(src.read) != 0 {
				t.Errorf("content was read for %v before the scope check rejected it", src.read)
			}
		})
	}
}

func TestPageReader_Page_RejectsInvalidScope(t *testing.T) {
	tests := []struct {
		name  string
		scope ReadScope
	}{
		{name: "empty root", scope: ReadScope{}},
		{name: "blank root", scope: ReadScope{RootID: "  "}},
		{name: "padded root", scope: ReadScope{RootID: " docs", IncludeDescendants: true}},
		{name: "padded whole-wiki root", scope: ReadScope{RootID: "root "}},
		{name: "whole-wiki root", scope: ReadScope{RootID: "root", IncludeDescendants: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPageReader(newFakeSource()).Page(context.Background(), tc.scope, "docs")
			if !errors.Is(err, ErrInvalidScope) {
				t.Fatalf("err = %v, want ErrInvalidScope", err)
			}
		})
	}
}

func TestPageReader_Page_PassesThroughOtherErrors(t *testing.T) {
	src := newFakeSource()
	src.pageErr = tree.ErrTreeNotLoaded

	_, err := NewPageReader(src).Page(context.Background(), ReadScope{RootID: "docs"}, "docs")
	if !errors.Is(err, tree.ErrTreeNotLoaded) {
		t.Fatalf("err = %v, want ErrTreeNotLoaded (operational errors must not look like not-found)", err)
	}
}

func TestPageReader_Page_ReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := newFakeSource()

	_, err := NewPageReader(src).Page(ctx, ReadScope{RootID: "docs"}, "docs")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(src.read) != 0 {
		t.Error("content was read after the context was cancelled")
	}
}

func TestPageReader_Page_ProjectsOnlyAllowListedFields(t *testing.T) {
	view, err := NewPageReader(newFakeSource()).Page(context.Background(), ReadScope{RootID: "docs"}, "docs")
	if err != nil {
		t.Fatalf("Page failed: %v", err)
	}

	want := PageView{
		ID:        "docs",
		Title:     "Title docs",
		Kind:      "page",
		Content:   "body of docs",
		UpdatedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	if *view != want {
		t.Errorf("view = %+v, want %+v", *view, want)
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, leak := range []string{"creator-user", "author-user", "slug-docs", "secret", "tags"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("shared view leaks %q: %s", leak, raw)
		}
	}
}

// TestPageView_FieldsAreAllowListed fails when a field is added to PageView.
// A new field is visible to anyone holding a share link, so adding one is a
// deliberate decision (ADR-0015): update this list in the same change and say
// why the field is safe to expose.
func TestPageView_FieldsAreAllowListed(t *testing.T) {
	allowed := []string{"content", "id", "kind", "title", "updatedAt"}

	var got []string
	typ := reflect.TypeOf(PageView{})
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		got = append(got, name)
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, allowed) {
		t.Errorf("PageView JSON fields = %v, want exactly %v", got, allowed)
	}
}

func TestPageReader_Page_ReadsRealTreeWithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	svc := tree.NewTreeService(dir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	id, err := svc.CreateNode("author-user", nil, "Customer", "customer", nil)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	content := "Visible body"
	if err := svc.UpdateNode(tree.UpdateNodeInput{
		UserID:          "author-user",
		ID:              *id,
		Title:           "Customer",
		Slug:            "customer",
		Content:         &content,
		ExpectedVersion: tree.VersionUnchecked,
		Tags:            []string{"internal-tag"},
		Properties:      map[string]string{"owner": "internal-owner"},
	}); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}

	view, err := NewPageReader(svc).Page(context.Background(), ReadScope{RootID: *id}, *id)
	if err != nil {
		t.Fatalf("Page failed: %v", err)
	}
	if !strings.Contains(view.Content, "Visible body") {
		t.Errorf("view.Content = %q, want the page body", view.Content)
	}
	raw, _ := json.Marshal(view)
	for _, leak := range []string{"internal-tag", "internal-owner", "author-user"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("shared view leaks %q: %s", leak, raw)
		}
	}
}
