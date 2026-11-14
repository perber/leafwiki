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
	"github.com/perber/wiki/internal/links"
)

// fakeSource is a hand-built tree: parents maps a page ID to its parent ID
// ("" for top level). It records which pages had their content read, so tests
// can prove the scope check happens before any content is touched.
type fakeSource struct {
	parents map[string]string
	titles  map[string]string // page ID → title; "Title <id>" when unset
	content map[string]string // page ID → content; "body of <id>" when unset
	read    []string
	pageErr error
	// lookupErr is returned by RouteByPath and RoutesByTitle.
	lookupErr error
}

func (f *fakeSource) title(id string) string {
	if t, ok := f.titles[id]; ok {
		return t
	}
	return "Title " + id
}

// path is the route path without a leading slash; slugs equal IDs.
func (f *fakeSource) path(id string) string {
	p := id
	for n := f.parents[id]; n != ""; n = f.parents[n] {
		p = n + "/" + p
	}
	return p
}

func (f *fakeSource) ResolvePermalinkTarget(id string) (*tree.PermalinkTarget, error) {
	if _, ok := f.parents[id]; !ok {
		return nil, tree.ErrPageNotFound
	}
	return &tree.PermalinkTarget{ID: id, Slug: id, Path: f.path(id)}, nil
}

func (f *fakeSource) RouteByPath(routePath string) (tree.RouteEntry, error) {
	if f.lookupErr != nil {
		return tree.RouteEntry{}, f.lookupErr
	}
	for id := range f.parents {
		if f.path(id) == routePath {
			return tree.RouteEntry{ID: id, Title: f.title(id), RoutePath: routePath}, nil
		}
	}
	return tree.RouteEntry{}, tree.ErrPageNotFound
}

func (f *fakeSource) RoutesByTitle(title string) ([]tree.RouteEntry, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	var out []tree.RouteEntry
	for id := range f.parents {
		if strings.EqualFold(strings.TrimSpace(f.title(id)), strings.TrimSpace(title)) {
			out = append(out, tree.RouteEntry{ID: id, Title: f.title(id), RoutePath: f.path(id)})
		}
	}
	return out, nil
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

func (f *fakeSource) body(id string) string {
	if c, ok := f.content[id]; ok {
		return c
	}
	return "body of " + id
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
		Content:    f.body(id),
		RawContent: "---\ntags: [secret]\n---\n" + f.body(id),
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
		Links:     []LinkView{},
	}
	if !reflect.DeepEqual(*view, want) {
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
	allowed := []string{"content", "id", "kind", "links", "title", "updatedAt"}

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

// TestLinkView_FieldsAreAllowListed guards LinkView like PageView: a link
// carries what the shared view needs to follow it, nothing about the target
// beyond its ID.
func TestLinkView_FieldsAreAllowListed(t *testing.T) {
	allowed := []string{"kind", "pageId", "target"}

	var got []string
	typ := reflect.TypeOf(LinkView{})
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		got = append(got, name)
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, allowed) {
		t.Errorf("LinkView JSON fields = %v, want exactly %v", got, allowed)
	}
}

// Links whose target is inside the scope are listed with the target page's
// ID; links to pages outside the scope, unknown pages and ambiguous titles
// are left out alike, so the list says nothing about pages outside.
func TestPageReader_Page_ListsLinksInsideTheScope(t *testing.T) {
	content := strings.Join([]string{
		"[Guide](guide) and again [Guide](guide)",
		"[Intro](/docs/guide/intro)",
		"[Intro with query](guide/intro?x=1#part)",
		"[Other](/other)",
		"[Up](..)",
		"[Missing](nope)",
		"[Web](https://example.com) [Mail](mailto:a@b.c) [Anchor](#top) [File](/assets/docs/a.png)",
		"[[FAQ]] [[Other Page]] [[docs/faq]] [[Shared Title]] [[Twice]] [[Missing Page]]",
		"`[[Intro Page]]` stays code",
	}, "\n\n")

	tests := []struct {
		name  string
		scope ReadScope
		want  []LinkView
	}{
		{
			name:  "subtree link",
			scope: ReadScope{RootID: "docs", IncludeDescendants: true},
			want: []LinkView{
				{Kind: "href", Target: "guide", PageID: "guide"},
				{Kind: "href", Target: "/docs/guide/intro", PageID: "intro"},
				{Kind: "href", Target: "guide/intro", PageID: "intro"},
				{Kind: "wikilink", Target: "FAQ", PageID: "faq"},
				{Kind: "wikilink", Target: "docs/faq", PageID: "faq"},
				{Kind: "wikilink", Target: "Shared Title", PageID: "guide"},
			},
		},
		{
			name:  "single-page link lists no subpages",
			scope: ReadScope{RootID: "docs"},
			want:  []LinkView{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := newFakeSource()
			src.titles = map[string]string{
				"faq": "FAQ", "other": "Other Page", "intro": "Intro Page",
				// one match inside the scope, one outside: the outside one is ignored
				"guide": "Shared Title",
			}
			src.parents["twin"] = ""
			src.titles["twin"] = "Shared Title"
			// two matches inside the scope: ambiguous, left out
			src.parents["t1"], src.parents["t2"] = "docs", "docs"
			src.titles["t1"], src.titles["t2"] = "Twice", "Twice"
			src.content = map[string]string{"docs": content}

			view, err := NewPageReader(src).Page(context.Background(), tc.scope, "docs")
			if err != nil {
				t.Fatalf("Page: %v", err)
			}
			if !reflect.DeepEqual(view.Links, tc.want) {
				t.Errorf("links = %+v\nwant    %+v", view.Links, tc.want)
			}
		})
	}
}

// A link to the shared page itself is listed for a single-page link too.
func TestPageReader_Page_ListsALinkToItself(t *testing.T) {
	src := newFakeSource()
	src.content = map[string]string{"docs": "[Top](/docs) [[Title docs]]"}

	view, err := NewPageReader(src).Page(context.Background(), ReadScope{RootID: "docs"}, "docs")
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	want := []LinkView{{Kind: "href", Target: "/docs", PageID: "docs"}, {Kind: "wikilink", Target: "Title docs", PageID: "docs"}}
	if !reflect.DeepEqual(view.Links, want) {
		t.Errorf("links = %+v, want %+v", view.Links, want)
	}
}

// Against a real tree: relative links resolve below the page, and a target
// moved out of the scope drops out of the list.
func TestPageReader_Page_RealTreeLinks(t *testing.T) {
	svc := tree.NewTreeService(t.TempDir())
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	section := tree.NodeKindSection
	mk := func(title, slug string, parent *string, kind *tree.NodeKind) string {
		id, err := svc.CreateNode("u", parent, title, slug, kind)
		if err != nil {
			t.Fatalf("CreateNode(%s) failed: %v", title, err)
		}
		return *id
	}
	customer := mk("Customer", "customer", nil, &section)
	contract := mk("Contract", "contract", &customer, nil)
	mk("Internal", "internal", nil, nil)
	content := "[Contract](contract) [Internal](/internal) [[Contract]] [[Internal]]"
	if err := svc.UpdateNode(tree.UpdateNodeInput{UserID: "u", ID: customer, Title: "Customer", Slug: "customer", Content: &content, ExpectedVersion: tree.VersionUnchecked}); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}

	r := NewPageReader(svc)
	sub := ReadScope{RootID: customer, IncludeDescendants: true}
	view, err := r.Page(context.Background(), sub, customer)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	want := []LinkView{{Kind: "href", Target: "contract", PageID: contract}, {Kind: "wikilink", Target: "Contract", PageID: contract}}
	if !reflect.DeepEqual(view.Links, want) {
		t.Fatalf("links = %+v, want %+v", view.Links, want)
	}

	if err := svc.MoveNode("u", contract, "root", tree.VersionUnchecked); err != nil {
		t.Fatalf("MoveNode: %v", err)
	}
	view, err = r.Page(context.Background(), sub, customer)
	if err != nil {
		t.Fatalf("Page after move: %v", err)
	}
	if len(view.Links) != 0 {
		t.Errorf("links after moving the target out = %+v, want none", view.Links)
	}
}

// Link forms against a nested shared page (docs/guide, subtree scope): what
// leads outside the scope or is not a Markdown link is never listed, and
// every spelling of an in-scope target is.
func TestPageReader_Page_LinkEdgeCases(t *testing.T) {
	intro := func(target string) []LinkView {
		return []LinkView{{Kind: "href", Target: target, PageID: "intro"}}
	}
	wiki := func(target string) []LinkView {
		return []LinkView{{Kind: "wikilink", Target: target, PageID: "intro"}}
	}
	none := []LinkView{}

	tests := []struct {
		name    string
		content string
		want    []LinkView
	}{
		{"parent escape to a sibling", "[x](../faq)", none},
		{"parent escape to the section", "[x](..) [y](../)", none},
		{"parent escape to another top level page", "[x](../../other)", none},
		{"escape beyond the root", "[x](../../../../../other)", none},
		{"parent step that returns into the scope", "[x](../guide/intro)", intro("../guide/intro")},
		{"absolute path outside", "[x](/other) [y](/docs/faq) [z](/docs)", none},
		{"absolute path inside", "[x](/docs/guide/intro)", intro("/docs/guide/intro")},
		{"dot segments", "[x](./intro)", intro("./intro")},
		{"trailing slash", "[x](intro/)", intro("intro/")},
		{"inner parent step", "[x](intro/../intro)", intro("intro/../intro")},
		{"percent-encoded slug, decoded as the target", "[x](%69ntro)", intro("intro")},
		{"query only links nowhere", "[x](?q=1)", none},
		{"query and fragment are dropped", "[x](intro?a=1#b)", intro("intro")},
		{"fragment only is not a page link", "[x](#intro)", none},
		{"external and mail", "[x](HTTPS://example.com/intro) [y](mailto:a@b.c) [z](//example.com/intro)", none},
		{"reference definition", "[x][ref]\n\n[ref]: intro", intro("intro")},
		{"image is not a link", "![x](intro)", none},
		{"autolink is not a link", "<intro>", none},
		{"html anchor is not a link", `<a href="intro">x</a>`, none},
		{"code span and fence", "`[x](intro)`\n\n```\n[y](intro) [[Intro Page]]\n```", none},
		{"wikilink with alias", "[[Intro Page|the intro]]", wiki("Intro Page")},
		{"wikilink with padded target", "[[  intro PAGE ]]", none},
		{"wikilink path hint to a page outside", "[[docs/faq]] [[../faq]] [[faq]]", none},
		{"wikilink path hint inside", "[[docs/guide/intro]]", wiki("docs/guide/intro")},
		{"wikilink to the section above", "[[Title docs]] [[docs]]", none},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := newFakeSource()
			src.titles = map[string]string{"intro": "Intro Page", "faq": "FAQ"}
			src.content = map[string]string{"guide": tc.content}

			view, err := NewPageReader(src).Page(context.Background(), ReadScope{RootID: "guide", IncludeDescendants: true}, "guide")
			if err != nil {
				t.Fatalf("Page: %v", err)
			}
			if !reflect.DeepEqual(view.Links, tc.want) {
				t.Errorf("links = %+v, want %+v", view.Links, tc.want)
			}
			for _, l := range view.Links {
				if l.PageID == "faq" || l.PageID == "other" || l.PageID == "docs" {
					t.Errorf("lists %q outside the scope", l.PageID)
				}
			}
		})
	}
}

// On a real tree the list is the link index's resolved targets restricted to
// the scope: same hrefs (case, encoding, unicode slugs, dot segments) and the
// same [[Title]] rules, minus everything outside.
func TestPageReader_Page_LinksMatchTheLinkIndex(t *testing.T) {
	dir := t.TempDir()
	svc := tree.NewTreeService(dir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	section := tree.NodeKindSection
	mk := func(title, slug string, parent *string, kind *tree.NodeKind) string {
		id, err := svc.CreateNode("u", parent, title, slug, kind)
		if err != nil {
			t.Fatalf("CreateNode(%s) failed: %v", title, err)
		}
		return *id
	}
	customer := mk("Customer", "customer", nil, &section)
	contract := mk("Contract", "contract", &customer, nil)
	sub := mk("Sub", "sub", &customer, &section)
	deep := mk("Deep Page", "deep", &sub, nil)
	umlaut := mk("Ärger Seite", "ärger", &customer, nil)
	internal := mk("Internal", "internal", nil, nil)
	mk("Contract", "contract-two", nil, nil) // same title outside

	content := strings.Join([]string{
		"[a](contract) [b](Contract) [c](CONTRACT/) [d](./sub/deep) [e](sub/../contract)",
		"[f](/customer/sub/deep#x) [g](%C3%A4rger) [h](ärger) [i](/Customer/Contract)",
		"[j](../internal) [k](/internal) [l](/) [m](..) [n](?q=1) [o](sub) [p](/customer)",
		"[[Deep Page]] [[deep page]] [[ Deep Page ]] [[Ärger Seite]] [[ÄRGER SEITE|x]] [[customer/sub/deep]] [[Internal]] [[Contract]] [[root]]",
		"![img](contract) <contract> [q][r]\n\n[r]: sub/deep",
	}, "\n")
	if err := svc.UpdateNode(tree.UpdateNodeInput{UserID: "u", ID: customer, Title: "Customer", Slug: "customer", Content: &content, ExpectedVersion: tree.VersionUnchecked}); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}

	store, err := links.NewLinksStore(dir)
	if err != nil {
		t.Fatalf("NewLinksStore: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()
	idx := links.NewLinkService(dir, svc, store)
	if err := idx.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}
	out, err := idx.GetOutgoingLinksForPage(customer)
	if err != nil {
		t.Fatalf("GetOutgoingLinksForPage: %v", err)
	}
	indexed := map[string]bool{}
	for _, o := range out.Outgoings {
		if o.ToPageID != "" {
			indexed[o.ToPageID] = true
		}
	}

	inside := map[string]bool{customer: true, contract: true, sub: true, deep: true, umlaut: true}
	for _, tc := range []struct {
		name  string
		scope ReadScope
		want  []string
	}{
		{"subtree", ReadScope{RootID: customer, IncludeDescendants: true}, []string{
			"href:contract", "href:./sub/deep", "href:sub/../contract", "href:/customer/sub/deep",
			"href:ärger", "href:sub", "href:/customer", "href:sub/deep",
			"wikilink:Deep Page", "wikilink:deep page", "wikilink:Ärger Seite", "wikilink:ÄRGER SEITE",
			"wikilink:customer/sub/deep", "wikilink:Contract",
		}},
		{"nested subtree", ReadScope{RootID: sub, IncludeDescendants: true}, []string{
			"href:/customer/sub/deep", "wikilink:Deep Page", "wikilink:deep page", "wikilink:customer/sub/deep",
		}},
		{"single page", ReadScope{RootID: customer}, []string{"href:/customer"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "nested subtree" {
				// the shared page must be inside its scope: share the sub section
				// with a copy of the content
				if err := svc.UpdateNode(tree.UpdateNodeInput{UserID: "u", ID: sub, Title: "Sub", Slug: "sub", Content: &content, ExpectedVersion: tree.VersionUnchecked}); err != nil {
					t.Fatalf("UpdateNode sub: %v", err)
				}
			}
			page := customer
			if tc.name == "nested subtree" {
				page = sub
			}
			view, err := NewPageReader(svc).Page(context.Background(), tc.scope, page)
			if err != nil {
				t.Fatalf("Page: %v", err)
			}
			got := map[string]bool{}
			for _, l := range view.Links {
				got[l.PageID] = true
				if l.PageID == internal {
					t.Errorf("lists the page outside the scope: %+v", l)
				}
				if ok, err := svc.IsInSubtree(l.PageID, tc.scope.RootID); err != nil || !ok {
					t.Errorf("lists %+v outside the scope (err %v)", l, err)
				}
			}
			var targets []string
			for _, l := range view.Links {
				targets = append(targets, l.Kind+":"+l.Target)
			}
			if !reflect.DeepEqual(targets, tc.want) {
				t.Errorf("targets = %q\nwant      %q", targets, tc.want)
			}
			if tc.name != "subtree" {
				return // the index reflects the customer page only
			}
			for id := range indexed {
				if inside[id] && !got[id] {
					t.Errorf("index resolves %s inside the scope, the list misses it; list = %+v", id, view.Links)
				}
			}
			for id := range got {
				if !indexed[id] {
					t.Errorf("list has %s that the index does not resolve; list = %+v", id, view.Links)
				}
			}
		})
	}
}

// A page with a thousand links stays cheap: repeated targets are resolved
// once, each target is a few locked reads of the in-memory tree.
func TestPageReader_Page_ManyLinks(t *testing.T) {
	src := newFakeSource()
	var b strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&b, "[a%d](guide/intro?n=%d) [[Intro Page]] [x%d](/other/n%d)\n\n", i, i, i, i%100)
	}
	src.titles = map[string]string{"intro": "Intro Page"}
	src.content = map[string]string{"docs": b.String()}

	start := time.Now()
	view, err := NewPageReader(src).Page(context.Background(), ReadScope{RootID: "docs", IncludeDescendants: true}, "docs")
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if len(view.Links) != 2 {
		t.Errorf("links = %+v, want the href and the wikilink once each", view.Links)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("1000 links took %v", d)
	}
}

// The same on the real tree, where each link takes the tree's read lock.
func TestPageReader_Page_ManyLinksRealTree(t *testing.T) {
	svc := tree.NewTreeService(t.TempDir())
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	section := tree.NodeKindSection
	id, err := svc.CreateNode("u", nil, "Customer", "customer", &section)
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	var b strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&b, "[a](/customer?n=%d) [[Customer]] [[Missing %d]] [x](/internal/n%d)\n\n", i, i, i)
	}
	content := b.String()
	if err := svc.UpdateNode(tree.UpdateNodeInput{UserID: "u", ID: *id, Title: "Customer", Slug: "customer", Content: &content, ExpectedVersion: tree.VersionUnchecked}); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}

	start := time.Now()
	view, err := NewPageReader(svc).Page(context.Background(), ReadScope{RootID: *id, IncludeDescendants: true}, *id)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	// 1000 distinct hrefs exceed maxScopedLinkTargets before the wikilinks
	// are reached: only the in-scope href is listed, and the cost is capped.
	if len(view.Links) != 1 || view.Links[0].Target != "/customer" {
		t.Errorf("links = %+v, want only the href", view.Links)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("1000 links took %v", d)
	}
}

// At most maxScopedLinkTargets distinct targets are resolved per read; later
// ones are not listed, like any link that doesn't resolve.
func TestPageReader_Page_CapsResolvedTargets(t *testing.T) {
	var unknown strings.Builder
	for i := 0; i < maxScopedLinkTargets; i++ {
		fmt.Fprintf(&unknown, "[x](nope%d) ", i)
	}
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "an in-scope link before the cap", content: "[g](guide) " + unknown.String(), want: 1},
		{name: "an in-scope link after the cap", content: unknown.String() + "[g](guide) [[Title guide]]", want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := newFakeSource()
			src.content = map[string]string{"docs": tc.content}
			view, err := NewPageReader(src).Page(context.Background(), ReadScope{RootID: "docs", IncludeDescendants: true}, "docs")
			if err != nil {
				t.Fatalf("Page: %v", err)
			}
			if len(view.Links) != tc.want {
				t.Errorf("links = %+v, want %d", view.Links, tc.want)
			}
		})
	}
}

// Percent-encoded and non-ASCII hrefs are listed under their decoded form, the
// key a client gets by decoding its rendered href.
func TestPageReader_Page_DecodesHrefTargets(t *testing.T) {
	src := newFakeSource()
	src.parents["über uns"] = "docs"
	src.content = map[string]string{"docs": "[a](%C3%BCber%20uns) [b](<über uns>) [c](guide%2Fintro)"}

	view, err := NewPageReader(src).Page(context.Background(), ReadScope{RootID: "docs", IncludeDescendants: true}, "docs")
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	want := []LinkView{
		{Kind: "href", Target: "über uns", PageID: "über uns"},
		// like the link index: the decoded "/" is a path separator
		{Kind: "href", Target: "guide/intro", PageID: "intro"},
	}
	if !reflect.DeepEqual(view.Links, want) {
		t.Errorf("links = %+v, want %+v", view.Links, want)
	}
}

// An operational error of the tree fails the read instead of silently
// dropping the links.
func TestPageReader_Page_PassesThroughLinkLookupErrors(t *testing.T) {
	for _, content := range []string{"[g](guide)", "[[Title guide]]"} {
		src := newFakeSource()
		src.content = map[string]string{"docs": content}
		src.lookupErr = tree.ErrTreeNotLoaded
		_, err := NewPageReader(src).Page(context.Background(), ReadScope{RootID: "docs", IncludeDescendants: true}, "docs")
		if !errors.Is(err, tree.ErrTreeNotLoaded) {
			t.Errorf("content %q: err = %v, want ErrTreeNotLoaded", content, err)
		}
	}
}
