package shared

import (
	"os"
	"path/filepath"
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
	got := strings.ReplaceAll(atomicWriteDir(`C:\wiki\data\root\page.md`), `\`, `/`)
	want := `C:/wiki/data/root`
	if got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}
