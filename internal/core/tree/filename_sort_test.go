package tree

import (
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type namedSortEntry string

func (e namedSortEntry) Name() string               { return string(e) }
func (e namedSortEntry) IsDir() bool                { return false }
func (e namedSortEntry) Type() fs.FileMode          { return 0 }
func (e namedSortEntry) Info() (fs.FileInfo, error) { return nil, os.ErrInvalid }
func TestSortReconstructionEntries_MixedCaseAndUnicode_PreservesOrder(t *testing.T) {
	entries := []os.DirEntry{namedSortEntry("z.md"), namedSortEntry("ä.md"), namedSortEntry("b.md"), namedSortEntry("A.md"), namedSortEntry("B.md"), namedSortEntry("a.md"), namedSortEntry("Ä.md")}
	sortReconstructionEntries(entries)
	got := []string{}
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := []string{"A.md", "a.md", "B.md", "b.md", "z.md", "Ä.md", "ä.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
func BenchmarkSortReconstructionEntries(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			original := make([]os.DirEntry, count)
			for i := range original {
				original[i] = namedSortEntry(fmt.Sprintf("Page-%05d.md", count-i))
			}
			entries := make([]os.DirEntry, count)
			b.ReportAllocs()
			for b.Loop() {
				copy(entries, original)
				sortReconstructionEntries(entries)
			}
		})
	}
}
func TestSortReconstructionEntries_MatchesLegacyComparator(t *testing.T) {
	// Pairwise order checks guard Unicode folding and the bytewise tie-breaker.
	names := []string{"a", "A", "İ", "i", "Ä", "ä", "ß", "ss", "Z"}
	for _, a := range names {
		for _, z := range names {
			entries := []os.DirEntry{namedSortEntry(a), namedSortEntry(z)}
			sortReconstructionEntries(entries)
			wantFirst := a
			if strings.ToLower(z) < strings.ToLower(a) || (strings.ToLower(z) == strings.ToLower(a) && z < a) {
				wantFirst = z
			}
			if entries[0].Name() != wantFirst {
				t.Fatalf("%q,%q: got %q want %q", a, z, entries[0].Name(), wantFirst)
			}
		}
	}
}

func BenchmarkSortReconstructionReadDirOrder(b *testing.B) {
	for _, mixed := range []bool{false, true} {
		name := "Lowercase"
		if mixed {
			name = "MixedCase"
		}
		b.Run(name, func(b *testing.B) {
			original := make([]os.DirEntry, 10000)
			for i := range original {
				prefix := "page"
				if mixed && i%2 == 0 {
					prefix = "Page"
				}
				original[i] = namedSortEntry(fmt.Sprintf("%s-%05d.md", prefix, i))
			}
			sort.Slice(original, func(i, j int) bool { return original[i].Name() < original[j].Name() })
			entries := make([]os.DirEntry, len(original))
			b.ReportAllocs()
			for b.Loop() {
				copy(entries, original)
				sortReconstructionEntries(entries)
			}
		})
	}
}
