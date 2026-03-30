package shared

import (
	"bytes"
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

	if err := WriteStreamAtomic(target, bytes.NewBufferString("hello stream"), 1024); err != nil {
		t.Fatalf("WriteStreamAtomic err: %v", err)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}
	if string(raw) != "hello stream" {
		t.Fatalf("content = %q", string(raw))
	}
}
