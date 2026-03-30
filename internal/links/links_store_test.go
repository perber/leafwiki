package links

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

func TestLinksStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewLinksStore(tmp)
	if err != nil {
		t.Fatalf("NewLinksStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(tmp, "links.db")); err != nil {
		t.Fatalf("expected links.db in storage dir, got err: %v", err)
	}
}

func TestLinksDatabasePath_WindowsPath(t *testing.T) {
	got := strings.ReplaceAll(linksDatabasePath(`C:\wiki\data`, "links.db"), `\`, `/`)
	want := `C:/wiki/data/links.db`
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
