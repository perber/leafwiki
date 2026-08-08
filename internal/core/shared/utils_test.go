package shared

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileAtomic_WritesToTargetFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "page.md")

	if err := WriteFileAtomic(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFileAtomic err: %v", err)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}
	if string(raw) != "hello" {
		t.Fatalf("content = %q", string(raw))
	}
}

func TestAtomicWriteDir_WindowsPath(t *testing.T) {
	testCases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "markdown page",
			path: `C:\wiki\data\root\page.md`,
			want: `C:/wiki/data/root`,
		},
		{
			name: "asset file",
			path: `C:\wiki\data\assets\a7b3\image.png`,
			want: `C:/wiki/data/assets/a7b3`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.ReplaceAll(atomicWriteDir(tc.path), `\`, `/`)
			if got != tc.want {
				t.Fatalf("dir = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriteStreamAtomic_WritesToTargetFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "asset.bin")

	if err := WriteStreamAtomic(target, bytes.NewBufferString("hello stream"), 1024, 0o644); err != nil {
		t.Fatalf("WriteStreamAtomic err: %v", err)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}
	if string(raw) != "hello stream" {
		t.Fatalf("content = %q", string(raw))
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat err: %v", err)
	}
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0o644 {
			t.Fatalf("permissions = %04o, want 0644", got)
		}
	}
}

// Regression tests for the zip-bomb unbounded-decompression DoS (CWE-409)
// fix's CopyWithBudget primitive: which cap trips must be attributed
// correctly, and an attacker-controlled declared compressed size that
// overflows int64 must not silently disable the decompression-ratio check.

func TestCopyWithBudget_ReportsErrFileTooLarge_WhenEntryCapIsBinding(t *testing.T) {
	limits := ExtractionLimits{MaxEntryBytes: 10, MaxTotalBytes: 1000, MaxRatio: 1 << 30, RatioFloorBytes: 1 << 30}
	budget := NewSizeBudget(limits.MaxTotalBytes)

	err := CopyWithBudget(&bytes.Buffer{}, strings.NewReader(strings.Repeat("A", 20)), 0, limits, budget)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got: %v", err)
	}
}

func TestCopyWithBudget_ReportsErrCumulativeSizeTooLarge_WhenBudgetCapIsBinding(t *testing.T) {
	limits := ExtractionLimits{MaxEntryBytes: 1000, MaxTotalBytes: 1000, MaxRatio: 1 << 30, RatioFloorBytes: 1 << 30}
	budget := NewSizeBudget(limits.MaxTotalBytes)
	budget.remaining = 10 // simulate most of the archive's budget already spent

	err := CopyWithBudget(&bytes.Buffer{}, strings.NewReader(strings.Repeat("A", 20)), 0, limits, budget)
	if !errors.Is(err, ErrCumulativeSizeTooLarge) {
		t.Fatalf("expected ErrCumulativeSizeTooLarge, got: %v", err)
	}
}

func TestCopyWithBudget_ClampsOverflowingDeclaredCompressedSize_KeepsRatioCheckFailClosed(t *testing.T) {
	limits := ExtractionLimits{MaxEntryBytes: 1 << 30, MaxTotalBytes: 1 << 30, MaxRatio: 3, RatioFloorBytes: 50}
	budget := NewSizeBudget(limits.MaxTotalBytes)

	// A declared compressed size near uint64 max overflows a naive int64
	// conversion to a negative number. If that silently disabled the ratio
	// check (compressedSize > 0 becomes false), this copy would succeed
	// despite decompressing well past the floor at a ratio no real entry of
	// this declared size could produce.
	declaredCompressedSize := uint64(math.MaxUint64)
	err := CopyWithBudget(&bytes.Buffer{}, strings.NewReader(strings.Repeat("A", 5000)), declaredCompressedSize, limits, budget)
	if !errors.Is(err, ErrDecompressionRatioTooHigh) {
		t.Fatalf("expected the ratio check to stay active (fail-closed) for an overflowing declared compressed size, got: %v", err)
	}
}
