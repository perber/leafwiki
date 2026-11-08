package tree

import (
	"errors"
	"strings"
	"testing"
)

func TestTreeService_FirstRootChildPath_FollowsDisplayOrder(t *testing.T) {
	tests := []struct {
		name  string
		order func(ids map[string]string) []string
		want  string
	}{
		{name: "section created first is first", want: "docs"},
		{
			name:  "after SortPages",
			order: func(ids map[string]string) []string { return []string{ids["other"], ids["docs"]} },
			want:  "other",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, ids := subtreeFixture(t)
			if tc.order != nil {
				if err := svc.SortPages("root", tc.order(ids)); err != nil {
					t.Fatalf("SortPages failed: %v", err)
				}
			}

			got, err := svc.FirstRootChildPath()
			if err != nil {
				t.Fatalf("FirstRootChildPath failed: %v", err)
			}
			if got != tc.want {
				t.Errorf("FirstRootChildPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTreeService_FirstRootChildPath_WithoutNodes_ReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		svc     func(t *testing.T) *TreeService
		wantErr error
	}{
		{name: "tree not loaded", svc: func(t *testing.T) *TreeService { return NewTreeService(t.TempDir()) }, wantErr: ErrTreeNotLoaded},
		{
			name: "empty tree",
			svc: func(t *testing.T) *TreeService {
				svc, _ := newLoadedService(t)
				return svc
			},
			wantErr: ErrPageNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.svc(t).FirstRootChildPath()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got != "" {
				t.Errorf("path = %q, want empty", got)
			}
		})
	}
}

// Run with -race: the read must hold the tree lock while SortPages and
// CreateNode rewrite the root's children.
func TestTreeService_FirstRootChildPath_ConcurrentWithMutations(t *testing.T) {
	svc, ids := subtreeFixture(t)
	orders := [][]string{{ids["other"], ids["docs"]}, {ids["docs"], ids["other"]}}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			if err := svc.SortPages("root", orders[i%2]); err != nil {
				t.Errorf("SortPages failed: %v", err)
				return
			}
		}
	}()
	for i := 0; i < 200; i++ {
		got, err := svc.FirstRootChildPath()
		if err != nil {
			t.Fatalf("FirstRootChildPath failed: %v", err)
		}
		if got != "docs" && got != "other" {
			t.Fatalf("FirstRootChildPath() = %q, want docs or other", got)
		}
	}
	<-done
}

func TestTreeService_PageRoutes_ListsNodesInNavigationOrder(t *testing.T) {
	svc, ids := subtreeFixture(t)
	if err := svc.SortPages("root", []string{ids["other"], ids["docs"]}); err != nil {
		t.Fatalf("SortPages failed: %v", err)
	}
	all := []string{"other", "docs", "docs/guide", "docs/guide/intro", "docs/faq"}

	tests := []struct {
		name          string
		limit         int
		want          []string
		wantTruncated bool
	}{
		{name: "no limit", limit: 0, want: all},
		{name: "limit equals the node count", limit: len(all), want: all},
		{name: "limit above the node count", limit: 100, want: all},
		{name: "limit below the node count", limit: 3, want: all[:3], wantTruncated: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entries, truncated, err := svc.PageRoutes(tc.limit)
			if err != nil {
				t.Fatalf("PageRoutes failed: %v", err)
			}
			got := make([]string, len(entries))
			for i, e := range entries {
				got[i] = e.RoutePath
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("route paths = %v, want %v", got, tc.want)
			}
			if truncated != tc.wantTruncated {
				t.Errorf("truncated = %v, want %v", truncated, tc.wantTruncated)
			}
		})
	}
}

func TestTreeService_PageRoutes_CopiesNodeFields(t *testing.T) {
	svc, ids := subtreeFixture(t)

	entries, _, err := svc.PageRoutes(0)
	if err != nil {
		t.Fatalf("PageRoutes failed: %v", err)
	}
	byPath := map[string]RouteEntry{}
	for _, e := range entries {
		byPath[e.RoutePath] = e
	}
	docs, intro := byPath["docs"], byPath["docs/guide/intro"]
	if docs.ID != ids["docs"] || docs.Title != "docs" || docs.Kind != NodeKindSection {
		t.Errorf("docs entry = %+v, want id %s, title docs, kind section", docs, ids["docs"])
	}
	if intro.ID != ids["intro"] || intro.Kind != NodeKindPage {
		t.Errorf("intro entry = %+v, want id %s, kind page", intro, ids["intro"])
	}
	node := svc.GetTree().Children[0].Children[0].Children[0]
	if !intro.UpdatedAt.Equal(node.Metadata.UpdatedAt) || intro.UpdatedAt.IsZero() {
		t.Errorf("intro UpdatedAt = %v, want the node's %v", intro.UpdatedAt, node.Metadata.UpdatedAt)
	}
}

func TestTreeService_PageRoutes_WithoutNodes(t *testing.T) {
	t.Run("tree not loaded", func(t *testing.T) {
		entries, truncated, err := NewTreeService(t.TempDir()).PageRoutes(0)
		if !errors.Is(err, ErrTreeNotLoaded) {
			t.Fatalf("err = %v, want ErrTreeNotLoaded", err)
		}
		if entries != nil || truncated {
			t.Errorf("entries = %v, truncated = %v, want nil, false", entries, truncated)
		}
	})
	t.Run("empty tree", func(t *testing.T) {
		svc, _ := newLoadedService(t)
		entries, truncated, err := svc.PageRoutes(0)
		if err != nil {
			t.Fatalf("PageRoutes failed: %v", err)
		}
		if len(entries) != 0 || truncated {
			t.Errorf("entries = %v, truncated = %v, want none", entries, truncated)
		}
	})
}

// Run with -race: the walk must hold the tree lock while SortPages rewrites
// the root's children.
func TestTreeService_PageRoutes_ConcurrentWithMutations(t *testing.T) {
	svc, ids := subtreeFixture(t)
	orders := [][]string{{ids["other"], ids["docs"]}, {ids["docs"], ids["other"]}}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			if err := svc.SortPages("root", orders[i%2]); err != nil {
				t.Errorf("SortPages failed: %v", err)
				return
			}
		}
	}()
	for i := 0; i < 100; i++ {
		entries, _, err := svc.PageRoutes(0)
		if err != nil {
			t.Fatalf("PageRoutes failed: %v", err)
		}
		if len(entries) != 5 {
			t.Fatalf("got %d entries, want 5", len(entries))
		}
	}
	<-done
}
