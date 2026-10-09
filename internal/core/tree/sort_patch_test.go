package tree

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkTreeSortPages(b *testing.B) {
	for _, count := range []int{100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			dir := b.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "root"), 0755); err != nil {
				b.Fatal(err)
			}
			svc := NewTreeService(dir)
			if err := svc.LoadTree(); err != nil {
				b.Fatal(err)
			}
			order := make([]string, count)
			for i := 0; i < count; i++ {
				id := fmt.Sprintf("page-%d", i)
				svc.tree.Children = append(svc.tree.Children, &PageNode{ID: id, Position: i, Parent: svc.tree})
				order[count-i-1] = id
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := svc.SortPages("root", order); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
func TestTreeService_SortPages_RepeatedReorder_PreservesPositions(t *testing.T) {
	svc, _ := newLoadedService(t)
	a, err := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatal(err)
	}
	z, err := svc.CreateNode("system", nil, "Z", "z", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatal(err)
	}
	for _, order := range [][]string{{*z, *a}, {*a, *z}, {*a, *z}} {
		if err := svc.SortPages("root", order); err != nil {
			t.Fatal(err)
		}
		for i, n := range svc.GetTree().Children {
			if n.ID != order[i] || n.Position != i {
				t.Fatalf("child %d = %+v", i, n)
			}
		}
	}
}
