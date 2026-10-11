package backup

import (
	"bytes"
	"crypto/rand"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
)

func packWindow(t *testing.T, r *Repository) uint {
	t.Helper()
	cfg, err := r.repo.Config()
	if err != nil {
		t.Fatalf("read repo config: %v", err)
	}
	return cfg.Pack.Window
}

func newLocalConfig(t *testing.T) (Config, string) {
	t.Helper()
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	writeFileHelper(t, rootDir, "page.md", "# Page\n")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll assets: %v", err)
	}
	return Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}, tmpDir
}

func TestInit_NewRepo_DisablesPackDeltaCompression(t *testing.T) {
	cfg, _ := newLocalConfig(t)
	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if w := packWindow(t, repo); w != 0 {
		t.Errorf("pack.window = %d, want 0", w)
	}
}

// Repos created by earlier LeafWiki versions carry go-git's default window
// (10); opening them must switch delta compression off too.
func TestInit_ExistingRepoWithDefaultWindow_DisablesPackDeltaCompression(t *testing.T) {
	cfg, tmpDir := newLocalConfig(t)
	if _, err := gogit.PlainInit(tmpDir, false); err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if w := packWindow(t, repo); w != 0 {
		t.Errorf("pack.window = %d, want 0", w)
	}
	// Persisted, not just set on the in-memory handle.
	reopened, err := gogit.PlainOpen(tmpDir)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	rc, err := reopened.Config()
	if err != nil {
		t.Fatalf("Config: %v", err)
	}
	if rc.Pack.Window != 0 {
		t.Errorf("persisted pack.window = %d, want 0", rc.Pack.Window)
	}
}

func TestInit_AdoptedRemoteHistory_DisablesPackDeltaCompression(t *testing.T) {
	bareDir := initBareRemote(t)
	pushFirstCommitToRemote(t, bareDir, "main", "root/remote.md", "# Remote\n")

	cfg, _ := newLocalConfig(t)
	cfg.RemoteURL = "file://" + bareDir
	cfg.SSHKey = testSSHKeyPEM
	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if w := packWindow(t, repo); w != 0 {
		t.Errorf("pack.window = %d, want 0", w)
	}
}

// countDeltaObjectsInRemotePacks scans every packfile the remote received and
// counts the entries stored as deltas.
func countDeltaObjectsInRemotePacks(t *testing.T, bareDir string) int {
	t.Helper()
	packs, err := filepath.Glob(filepath.Join(bareDir, "objects", "pack", "*.pack"))
	if err != nil {
		t.Fatalf("glob packs: %v", err)
	}
	if len(packs) == 0 {
		t.Fatal("remote has no packfiles")
	}
	deltas := 0
	for _, p := range packs {
		f, err := os.Open(p)
		if err != nil {
			t.Fatalf("open pack: %v", err)
		}
		s := packfile.NewScanner(f)
		_, n, err := s.Header()
		if err != nil {
			_ = f.Close()
			t.Fatalf("pack header: %v", err)
		}
		for i := uint32(0); i < n; i++ {
			h, err := s.NextObjectHeader()
			if err != nil {
				_ = f.Close()
				t.Fatalf("pack object header: %v", err)
			}
			if h.Type == plumbing.OFSDeltaObject || h.Type == plumbing.REFDeltaObject {
				deltas++
			}
		}
		_ = f.Close()
	}
	return deltas
}

// Regression guard for go-git's delta search: with the default pack window it
// diffs every new blob against its neighbours byte by byte, which on
// image-heavy wikis makes the push run into the network timeout. Two nearly
// identical assets in one push would be stored as a delta if the search ran.
func TestRunBackup_PushedPack_ContainsNoDeltas(t *testing.T) {
	bareDir := initBareRemote(t)
	// file:// pushes run the real git-receive-pack, which explodes small packs
	// into loose objects; keep them as received so the encoding is inspectable.
	if out, err := exec.Command("git", "-C", bareDir, "config", "receive.unpackLimit", "1").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}
	repo, rootDir := newRepoWithRemote(t, bareDir)
	assetsDir := filepath.Join(filepath.Dir(rootDir), "assets")

	base := make([]byte, 64<<10)
	if _, err := rand.Read(base); err != nil {
		t.Fatalf("rand: %v", err)
	}
	variant := bytes.Clone(base)
	variant[len(variant)/2] ^= 0xff
	if err := os.WriteFile(filepath.Join(assetsDir, "a.bin"), base, 0o644); err != nil {
		t.Fatalf("write a.bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "b.bin"), variant, 0o644); err != nil {
		t.Fatalf("write b.bin: %v", err)
	}

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	if n := countDeltaObjectsInRemotePacks(t, bareDir); n != 0 {
		t.Errorf("remote packs contain %d delta objects, want 0", n)
	}
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// go-git's repack already removes the loose objects it packed; gc must not
// warn once per object when it finds them gone.
func TestGC_AfterRepack_LogsNoWarnings(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	writeFileHelper(t, rootDir, "second.md", "second\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	logs := captureLogs(t)
	if !repo.gc() {
		t.Fatal("gc failed")
	}
	if strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("gc logged warnings:\n%s", logs.String())
	}
	if n := countLooseObjects(t, repo); n != 0 {
		t.Errorf("expected no loose objects after gc, got %d", n)
	}
}

// Loose objects no ref reaches are not packed by the repack; gc still prunes them.
func TestGC_UnreachableLooseObject_IsPruned(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, _ := newRepoWithRemote(t, bareDir)

	orphan, err := repo.writeBlob([]byte("orphan blob nobody references\n"))
	if err != nil {
		t.Fatalf("writeBlob: %v", err)
	}
	if !repo.gc() {
		t.Fatal("gc failed")
	}
	_, err = repo.repo.Storer.EncodedObject(plumbing.AnyObject, orphan)
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("unreachable object still present after gc (err=%v)", err)
	}
}
