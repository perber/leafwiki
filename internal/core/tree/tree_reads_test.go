package tree

import (
	"errors"
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
