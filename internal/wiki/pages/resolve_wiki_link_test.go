package pages_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/perber/wiki/internal/wiki/pages"
)

func TestResolveWikiLinkUseCase_ResolvesLikeStoredLinks(t *testing.T) {
	deps := newTestDeps(t)
	create := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.orchestrator(), slog.Default(), nil)
	docs, err := create.Execute(context.Background(), pages.CreatePageInput{UserID: "user1", Title: "Docs", Slug: "docs", Kind: sectionKind()})
	if err != nil {
		t.Fatalf("CreatePage(docs) failed: %v", err)
	}
	for _, in := range []pages.CreatePageInput{
		{UserID: "user1", Title: "Setup", Slug: "setup", Kind: pageKind(), ParentID: &docs.Page.ID},
		{UserID: "user1", Title: "Twin", Slug: "twin-a", Kind: pageKind()},
		{UserID: "user1", Title: "Twin", Slug: "twin-b", Kind: pageKind()},
	} {
		if _, err := create.Execute(context.Background(), in); err != nil {
			t.Fatalf("CreatePage(%s) failed: %v", in.Slug, err)
		}
	}

	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "title", target: "setup", want: "/docs/setup"},
		{name: "path hint", target: "docs/setup", want: "/docs/setup"},
		{name: "ambiguous title", target: "Twin"},
		{name: "unknown", target: "Missing"},
	}
	uc := pages.NewResolveWikiLinkUseCase(deps.tree)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := uc.Execute(context.Background(), pages.ResolveWikiLinkInput{Target: tc.target})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			got := ""
			if out != nil {
				got = out.RoutePath
			}
			if got != tc.want {
				t.Errorf("route = %q, want %q", got, tc.want)
			}
		})
	}
}
