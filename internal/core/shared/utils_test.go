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

func TestWriteFileAtomic_ReplacesExistingFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "page.md")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile err: %v", err)
	}

	if err := WriteFileAtomic(target, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFileAtomic err: %v", err)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}
	if string(raw) != "new" {
		t.Fatalf("content = %q, want %q", string(raw), "new")
	}
}

// The target is an empty directory, so the rename must fail. Removing the
// target before renaming (as a Windows workaround once did) deleted the empty
// directory first, which made the write succeed and replaced a directory with
// a file (#1698).
func TestWriteFileAtomic_TargetIsEmptyDirectory_FailsAndKeepsDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "page.md")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir err: %v", err)
	}

	if err := WriteFileAtomic(target, []byte("x"), 0o644); err == nil {
		t.Fatal("expected an error when the target is a directory")
	}

	assertDirectoryWithoutTempFiles(t, parent, target)
}

func TestWriteStreamAtomic_TargetIsEmptyDirectory_FailsAndKeepsDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "asset.bin")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir err: %v", err)
	}

	if err := WriteStreamAtomic(target, strings.NewReader("x"), 1<<20, 0o644); err == nil {
		t.Fatal("expected an error when the target is a directory")
	}

	assertDirectoryWithoutTempFiles(t, parent, target)
}

// Whether replacing a file that a reader holds open succeeds depends on the
// OS, but the target must never end up missing: either the old or the new
// content has to be readable afterwards.
func TestWriteFileAtomic_TargetOpenForReading_NeverLeavesTargetMissing(t *testing.T) {
	target := filepath.Join(t.TempDir(), "asset.bin")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile err: %v", err)
	}
	reader, err := os.Open(target)
	if err != nil {
		t.Fatalf("Open err: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	err = WriteFileAtomic(target, []byte("new"), 0o644)
	t.Logf("replace while open: err=%v", err)

	raw, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("target missing after replace while open: %v", readErr)
	}
	if got := string(raw); got != "old" && got != "new" {
		t.Fatalf("content = %q, want %q or %q", got, "old", "new")
	}
}

func assertDirectoryWithoutTempFiles(t *testing.T, parent, target string) {
	t.Helper()

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat err: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is no longer a directory", target)
	}
	leftovers, err := filepath.Glob(filepath.Join(parent, ".tmp-*"))
	if err != nil {
		t.Fatalf("Glob err: %v", err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
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
