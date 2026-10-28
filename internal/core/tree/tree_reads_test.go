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
