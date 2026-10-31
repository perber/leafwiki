package pages_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki/pages"
)

func TestGetFirstPagePathUseCase_WithPages_ReturnsFirstRootChild(t *testing.T) {
	deps := newTestDeps(t)
	create := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.orchestrator(), slog.Default(), nil)
	for _, in := range []pages.CreatePageInput{
		{UserID: "user1", Title: "Docs", Slug: "docs", Kind: sectionKind()},
		{UserID: "user1", Title: "About", Slug: "about", Kind: pageKind()},
	} {
		if _, err := create.Execute(context.Background(), in); err != nil {
			t.Fatalf("CreatePage(%s) failed: %v", in.Slug, err)
		}
	}

	out, err := pages.NewGetFirstPagePathUseCase(deps.tree).Execute(context.Background(), pages.GetFirstPagePathInput{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out.RoutePath != "docs" {
		t.Errorf("RoutePath = %q, want %q", out.RoutePath, "docs")
	}
}

func TestGetFirstPagePathUseCase_EmptyTree_ReturnsPageNotFound(t *testing.T) {
	deps := newTestDeps(t)

	out, err := pages.NewGetFirstPagePathUseCase(deps.tree).Execute(context.Background(), pages.GetFirstPagePathInput{})
	if !errors.Is(err, tree.ErrPageNotFound) {
		t.Fatalf("err = %v, want tree.ErrPageNotFound", err)
	}
	if out != nil {
		t.Errorf("out = %+v, want nil", out)
	}
}
