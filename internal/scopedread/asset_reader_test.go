package scopedread

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	coreassets "github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/tree"
)

// fakeAssets serves files from a temp folder per page ID and records which
// assets were opened, so tests can prove the scope check comes first.
type fakeAssets struct {
	dir    string
	opened []string
	err    error
}

func newFakeAssets(t *testing.T) *fakeAssets {
	t.Helper()
	f := &fakeAssets{dir: t.TempDir()}
	for _, id := range []string{"docs", "guide", "other"} {
		if err := os.MkdirAll(filepath.Join(f.dir, id), 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(f.dir, id, "pic.png"), []byte("png of "+id), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return f
}

func (f *fakeAssets) OpenAsset(pageID, filename string) (*os.File, error) {
	f.opened = append(f.opened, pageID+"/"+filename)
	if f.err != nil {
		return nil, f.err
	}
	file, err := os.Open(filepath.Join(f.dir, pageID, filename))
	if err != nil {
		return nil, coreassets.ErrAssetNotFound
	}
	return file, nil
}

func readAsset(t *testing.T, a *AssetFile) string {
	t.Helper()
	defer func() { _ = a.Content.Close() }()
	body, err := io.ReadAll(a.Content)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(body)
}

func TestAssetReader_Asset_EnforcesScope(t *testing.T) {
	single := ReadScope{RootID: "docs"}
	subtree := ReadScope{RootID: "docs", IncludeDescendants: true}

	tests := []struct {
		name     string
		scope    ReadScope
		pageID   string
		file     string
		want     string
		wantErr  error
		wantOpen bool
	}{
		{name: "single page: the root's asset", scope: single, pageID: "docs", file: "pic.png", want: "png of docs", wantOpen: true},
		{name: "single page: a child's asset", scope: single, pageID: "guide", file: "pic.png", wantErr: ErrNotFound},
		{name: "subtree: a child's asset", scope: subtree, pageID: "guide", file: "pic.png", want: "png of guide", wantOpen: true},
		{name: "subtree: a sibling's asset", scope: subtree, pageID: "other", file: "pic.png", wantErr: ErrNotFound},
		{name: "subtree: an unknown page", scope: subtree, pageID: "missing", file: "pic.png", wantErr: ErrNotFound},
		{name: "deleted root, single page", scope: ReadScope{RootID: "gone"}, pageID: "gone", file: "pic.png", wantErr: ErrNotFound},
		{name: "in scope, missing file", scope: subtree, pageID: "guide", file: "nope.png", wantErr: ErrNotFound, wantOpen: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := newFakeAssets(t)
			a, err := NewAssetReader(newFakeSource(), files).Asset(context.Background(), tc.scope, tc.pageID, tc.file)
			if (len(files.opened) > 0) != tc.wantOpen {
				t.Errorf("opened %v, want an open: %v", files.opened, tc.wantOpen)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || a != nil {
					t.Fatalf("Asset = %v, %v, want nil, %v", a, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Asset: %v", err)
			}
			if a.Name != tc.file || a.ModTime.IsZero() {
				t.Errorf("name %q, mod time %v, want %q and a time", a.Name, a.ModTime, tc.file)
			}
			if got := readAsset(t, a); got != tc.want {
				t.Errorf("content = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAssetReader_Asset_Errors(t *testing.T) {
	denied := errors.New("open asset: permission denied")
	tests := []struct {
		name    string
		scope   ReadScope
		openErr error
		wantErr error
	}{
		{name: "invalid scope", scope: ReadScope{RootID: "root"}, wantErr: ErrInvalidScope},
		{name: "operational error passes through", scope: ReadScope{RootID: "docs"}, openErr: denied, wantErr: denied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := newFakeAssets(t)
			files.err = tc.openErr
			a, err := NewAssetReader(newFakeSource(), files).Asset(context.Background(), tc.scope, "docs", "pic.png")
			if !errors.Is(err, tc.wantErr) || a != nil {
				t.Fatalf("Asset = %v, %v, want nil, %v", a, err, tc.wantErr)
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	files := newFakeAssets(t)
	if _, err := NewAssetReader(newFakeSource(), files).Asset(ctx, ReadScope{RootID: "docs"}, "docs", "pic.png"); !errors.Is(err, context.Canceled) || len(files.opened) != 0 {
		t.Errorf("cancelled: err = %v, opened %v, want context.Canceled and no open", err, files.opened)
	}
}

// Against a real tree and asset service: a page moved out takes its assets
// out of the scope, and a deleted page's leftover files are not served.
func TestAssetReader_Asset_RealTreeLifecycle(t *testing.T) {
	dir := t.TempDir()
	svc := tree.NewTreeService(dir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	assets := coreassets.NewAssetService(dir, tree.NewSlugService())
	section := tree.NodeKindSection
	mk := func(title string, parent *string, kind *tree.NodeKind) string {
		id, err := svc.CreateNode("u", parent, title, title, kind)
		if err != nil {
			t.Fatalf("CreateNode(%s) failed: %v", title, err)
		}
		if err := os.MkdirAll(filepath.Join(assets.GetAssetsDir(), *id), 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(assets.GetAssetsDir(), *id, "pic.png"), []byte(title), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		return *id
	}
	docs := mk("docs", nil, &section)
	guide := mk("guide", &docs, nil)

	r := NewAssetReader(svc, assets)
	sub := ReadScope{RootID: docs, IncludeDescendants: true}
	a, err := r.Asset(context.Background(), sub, guide, "pic.png")
	if err != nil {
		t.Fatalf("child's asset: %v", err)
	}
	if got := readAsset(t, a); got != "guide" {
		t.Errorf("content = %q, want guide", got)
	}

	if err := svc.MoveNode("u", guide, "root", tree.VersionUnchecked); err != nil {
		t.Fatalf("MoveNode: %v", err)
	}
	if _, err := r.Asset(context.Background(), sub, guide, "pic.png"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after move out: err = %v, want ErrNotFound", err)
	}

	// The tree forgets the page; its asset file is left on disk here.
	if err := svc.DeleteNode("u", docs, true, tree.VersionUnchecked); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if _, err := r.Asset(context.Background(), ReadScope{RootID: docs}, docs, "pic.png"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted page with files left: err = %v, want ErrNotFound", err)
	}
}
