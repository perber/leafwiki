package pagesave

import (
	"log/slog"
	"testing"

	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/search"
)

func TestNewLinkIndexSideEffect_DefaultsLogger(t *testing.T) {
	treeService := tree.NewTreeService(t.TempDir())
	store, err := links.NewLinksStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLinksStore failed: %v", err)
	}
	effect := NewLinkIndexSideEffect(links.NewLinkService(t.TempDir(), treeService, store), nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

func TestNewRevisionSideEffect_DefaultsLogger(t *testing.T) {
	treeService := tree.NewTreeService(t.TempDir())
	effect := NewRevisionSideEffect(revision.NewService(t.TempDir(), treeService, nil, revision.ServiceOptions{}), nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

func TestNewSearchIndexSideEffect_DefaultsLogger(t *testing.T) {
	treeService := tree.NewTreeService(t.TempDir())
	index, err := search.NewSQLiteIndex(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteIndex failed: %v", err)
	}
	defer func() {
		if err := index.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	effect := NewSearchIndexSideEffect(index, treeService, nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

func TestNewTagsSideEffect_DefaultsLogger(t *testing.T) {
	effect := NewTagsSideEffect(nil, nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

func TestNewPropertiesSideEffect_DefaultsLogger(t *testing.T) {
	effect := NewPropertiesSideEffect(nil, nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}
