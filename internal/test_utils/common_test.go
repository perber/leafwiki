package test_utils

import (
	"runtime"
	"testing"
)

func TestFileURL(t *testing.T) {
	// filepath semantics differ per OS, so each OS gets its own native path.
	tests := map[string]struct {
		goos string
		dir  string
		want string
	}{
		"unix absolute path": {goos: "linux", dir: "/tmp/repo", want: "file:///tmp/repo"},
		"windows drive path": {goos: "windows", dir: `D:\wiki\repo`, want: "file:///D:/wiki/repo"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if (tt.goos == "windows") != (runtime.GOOS == "windows") {
				t.Skipf("case is for %s paths", tt.goos)
			}

			if got := FileURL(tt.dir); got != tt.want {
				t.Errorf("FileURL(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}
