package test_utils

import (
	"runtime"
	"testing"
)

func TestFileURL(t *testing.T) {
	// A drive-letter path only means something to filepath on Windows, so that
	// case is skipped elsewhere. The slash-rooted path works on every OS.
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
			if tt.goos == "windows" && runtime.GOOS != "windows" {
				t.Skip("drive-letter paths are only converted on Windows")
			}

			if got := FileURL(tt.dir); got != tt.want {
				t.Errorf("FileURL(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}
