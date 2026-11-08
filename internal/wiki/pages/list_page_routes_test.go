package pages_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki/pages"
)

func TestListPageRoutesUseCase_ListsPagesAndSectionsInNavigationOrder(t *testing.T) {
	deps := newTestDeps(t)
	create := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.orchestrator(), slog.Default(), nil)
	docs, err := create.Execute(context.Background(), pages.CreatePageInput{UserID: "user1", Title: "Docs", Slug: "docs", Kind: sectionKind()})
	if err != nil {
		t.Fatalf("CreatePage(docs) failed: %v", err)
	}
	for _, in := range []pages.CreatePageInput{
		{UserID: "user1", Title: "Setup", Slug: "setup", Kind: pageKind(), ParentID: &docs.Page.ID},
		{UserID: "user1", Title: "About", Slug: "about", Kind: pageKind()},
	} {
		if _, err := create.Execute(context.Background(), in); err != nil {
			t.Fatalf("CreatePage(%s) failed: %v", in.Slug, err)
		}
	}

	tests := []struct {
		name          string
		limit         int
		want          string
		wantTruncated bool
	}{
		{name: "no limit", want: "docs,docs/setup,about"},
		{name: "limit cuts the list", limit: 2, want: "docs,docs/setup", wantTruncated: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := pages.NewListPageRoutesUseCase(deps.tree).Execute(context.Background(), pages.ListPageRoutesInput{Limit: tc.limit})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			got := make([]string, len(out.Routes))
			for i, r := range out.Routes {
				got[i] = r.RoutePath
			}
			if strings.Join(got, ",") != tc.want {
				t.Errorf("routes = %v, want %s", got, tc.want)
			}
			if out.Truncated != tc.wantTruncated {
				t.Errorf("Truncated = %v, want %v", out.Truncated, tc.wantTruncated)
			}
		})
	}
}

func TestListPageRoutesUseCase_TreeNotLoaded_ReturnsError(t *testing.T) {
	out, err := pages.NewListPageRoutesUseCase(tree.NewTreeService(t.TempDir())).Execute(context.Background(), pages.ListPageRoutesInput{})
	if !errors.Is(err, tree.ErrTreeNotLoaded) {
		t.Fatalf("err = %v, want tree.ErrTreeNotLoaded", err)
	}
	if out != nil {
		t.Errorf("out = %+v, want nil", out)
	}
}
