package wiki

import (
	"strings"
	"testing"
)

func TestSearchRootDir_WindowsPath(t *testing.T) {
	got := strings.ReplaceAll(searchRootDir(`C:\wiki\data`), `\`, `/`)
	want := `C:/wiki/data/root`
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
