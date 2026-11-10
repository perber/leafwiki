package shares

import (
	"context"
	"errors"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/scopedread"
	"github.com/perber/wiki/internal/sharing"
)

type fakeResolver struct {
	grant *sharing.Grant
	err   error
	got   string
}

func (f *fakeResolver) Resolve(_ context.Context, token string) (*sharing.Grant, error) {
	f.got = token
	return f.grant, f.err
}

type fakeReader struct {
	err       error
	gotScope  scopedread.ReadScope
	gotPageID string
}

func (f *fakeReader) Page(_ context.Context, scope scopedread.ReadScope, id string) (*scopedread.PageView, error) {
	f.gotScope, f.gotPageID = scope, id
	if f.err != nil {
		return nil, f.err
	}
	return &scopedread.PageView{ID: id, Title: "T", Kind: "page", UpdatedAt: time.Unix(0, 0)}, nil
}

func errorCode(err error) string {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		return loc.Code
	}
	return ""
}

func TestGetSharedPageUseCase_Execute_ReadsThroughTheLinkScope(t *testing.T) {
	grant := &sharing.Grant{ShareID: "share-1", PageID: "customer", IncludeDescendants: true}

	tests := []struct {
		name       string
		pageID     string
		wantPageID string
	}{
		{name: "no page id reads the shared page", pageID: "", wantPageID: "customer"},
		{name: "a page id reads that page inside the scope", pageID: "contract", wantPageID: "contract"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			links := &fakeResolver{grant: grant}
			pages := &fakeReader{}
			uc := NewGetSharedPageUseCase(links, pages)

			out, err := uc.Execute(context.Background(), GetSharedPageInput{Token: "lws_token", PageID: tc.pageID})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if links.got != "lws_token" {
				t.Errorf("resolved token %q, want lws_token", links.got)
			}
			wantScope := scopedread.ReadScope{RootID: "customer", IncludeDescendants: true}
			if pages.gotScope != wantScope {
				t.Errorf("scope = %+v, want %+v", pages.gotScope, wantScope)
			}
			if pages.gotPageID != tc.wantPageID || out.Page.ID != tc.wantPageID {
				t.Errorf("read page %q (out %q), want %q", pages.gotPageID, out.Page.ID, tc.wantPageID)
			}
		})
	}
}

func TestGetSharedPageUseCase_Execute_MapsErrors(t *testing.T) {
	grant := &sharing.Grant{ShareID: "share-1", PageID: "customer"}
	storeDown := sharederrors.NewLocalizedError(sharing.ErrCodeShareLinkStoreUnavailable, "m", "t", nil)
	linkGone := sharederrors.NewLocalizedError(sharing.ErrCodeShareLinkNotFound, "m", "t", sharing.ErrShareLinkNotFound)
	diskError := errors.New("read /data/root/customer.md: permission denied")

	tests := []struct {
		name        string
		resolveErr  error
		readErr     error
		wantCode    string
		wantWrapped error
	}{
		{name: "unknown or expired link", resolveErr: linkGone, wantCode: sharing.ErrCodeShareLinkNotFound},
		{name: "store suspended for a restore", resolveErr: storeDown, wantCode: sharing.ErrCodeShareLinkStoreUnavailable},
		{name: "page out of scope, unknown or deleted", readErr: scopedread.ErrNotFound, wantCode: sharing.ErrCodeShareLinkNotFound},
		{name: "link to an invalid scope", readErr: scopedread.ErrInvalidScope, wantCode: sharing.ErrCodeShareLinkNotFound},
		{name: "operational error is not a missing page", readErr: diskError, wantWrapped: diskError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			links := &fakeResolver{grant: grant, err: tc.resolveErr}
			if tc.resolveErr != nil {
				links.grant = nil
			}
			uc := NewGetSharedPageUseCase(links, &fakeReader{err: tc.readErr})

			out, err := uc.Execute(context.Background(), GetSharedPageInput{Token: "lws_token"})
			if out != nil {
				t.Errorf("out = %+v, want nil on error", out)
			}
			if tc.wantCode != "" && errorCode(err) != tc.wantCode {
				t.Errorf("error code = %q (err %v), want %q", errorCode(err), err, tc.wantCode)
			}
			if tc.wantWrapped != nil && (!errors.Is(err, tc.wantWrapped) || errorCode(err) != "") {
				t.Errorf("err = %v, want the unlocalized cause wrapped", err)
			}
		})
	}
}

type fakeTreeReader struct {
	err      error
	gotScope scopedread.ReadScope
}

func (f *fakeTreeReader) Tree(_ context.Context, scope scopedread.ReadScope) (*scopedread.TreeNodeView, error) {
	f.gotScope = scope
	if f.err != nil {
		return nil, f.err
	}
	return &scopedread.TreeNodeView{ID: scope.RootID, Title: "T", Kind: "section", Children: []*scopedread.TreeNodeView{}}, nil
}

func TestGetSharedTreeUseCase_Execute_ReadsThroughTheLinkScope(t *testing.T) {
	for _, include := range []bool{false, true} {
		links := &fakeResolver{grant: &sharing.Grant{ShareID: "share-1", PageID: "customer", IncludeDescendants: include}}
		trees := &fakeTreeReader{}

		out, err := NewGetSharedTreeUseCase(links, trees).Execute(context.Background(), GetSharedTreeInput{Token: "lws_token"})
		if err != nil {
			t.Fatalf("include %v: Execute: %v", include, err)
		}
		if links.got != "lws_token" {
			t.Errorf("resolved token %q, want lws_token", links.got)
		}
		if want := (scopedread.ReadScope{RootID: "customer", IncludeDescendants: include}); trees.gotScope != want {
			t.Errorf("scope = %+v, want %+v", trees.gotScope, want)
		}
		if out.Tree.ID != "customer" {
			t.Errorf("tree root = %q, want customer", out.Tree.ID)
		}
	}
}

func TestGetSharedTreeUseCase_Execute_MapsErrors(t *testing.T) {
	storeDown := sharederrors.NewLocalizedError(sharing.ErrCodeShareLinkStoreUnavailable, "m", "t", nil)
	diskError := errors.New("tree not loaded")

	tests := []struct {
		name        string
		resolveErr  error
		readErr     error
		wantCode    string
		wantWrapped error
	}{
		{name: "store suspended for a restore", resolveErr: storeDown, wantCode: sharing.ErrCodeShareLinkStoreUnavailable},
		{name: "shared page deleted", readErr: scopedread.ErrNotFound, wantCode: sharing.ErrCodeShareLinkNotFound},
		{name: "link to an invalid scope", readErr: scopedread.ErrInvalidScope, wantCode: sharing.ErrCodeShareLinkNotFound},
		{name: "operational error is not a missing page", readErr: diskError, wantWrapped: diskError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			links := &fakeResolver{grant: &sharing.Grant{ShareID: "share-1", PageID: "customer"}, err: tc.resolveErr}
			if tc.resolveErr != nil {
				links.grant = nil
			}
			out, err := NewGetSharedTreeUseCase(links, &fakeTreeReader{err: tc.readErr}).Execute(context.Background(), GetSharedTreeInput{Token: "lws_token"})
			if out != nil {
				t.Errorf("out = %+v, want nil on error", out)
			}
			if tc.wantCode != "" && errorCode(err) != tc.wantCode {
				t.Errorf("error code = %q (err %v), want %q", errorCode(err), err, tc.wantCode)
			}
			if tc.wantWrapped != nil && (!errors.Is(err, tc.wantWrapped) || errorCode(err) != "") {
				t.Errorf("err = %v, want the unlocalized cause wrapped", err)
			}
		})
	}
}
