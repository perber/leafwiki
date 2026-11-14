package assets

import (
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/shared"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
)

const testAssetMaxBytes int64 = 1024

func TestSaveAndListAsset(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "lonely-page", ID: "a7b3"}
	// Create index.md page
	pagePath := filepath.Join(tmp, "lonely-page")
	if err := os.MkdirAll(pagePath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pagePath, "index.md"), []byte("# Lonely Page"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	service := NewAssetService(tmp, tree.NewSlugService())

	file, name, err := test_utils.CreateMultipartFile("my-image.png", []byte("hello image"))
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			t.Fatalf("Close() error: %v", err)
		}
	}()

	url, err := service.SaveAssetForPage(page, file, name, testAssetMaxBytes)
	if err != nil {
		t.Fatalf("SaveAsset failed: %v", err)
	}

	if url == "" {
		t.Fatalf("expected public URL, got empty string")
	}

	files, err := service.ListAssetsForPage(page)
	if err != nil {
		t.Fatalf("ListAssets failed: %v", err)
	}

	if len(files) != 1 || files[0] != "/assets/a7b3/my-image.png" {
		t.Errorf("unexpected asset list: %v", files)
	}
}

func TestDeletePageAndEnsureAllAssetsAreDeleted(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "lonely-page", ID: "a7b3"}
	// Create index.md page
	pagePath := filepath.Join(tmp, "lonely-page")
	if err := os.MkdirAll(pagePath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pagePath, "index.md"), []byte("# Lonely Page"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	service := NewAssetService(tmp, tree.NewSlugService())

	file, name, err := test_utils.CreateMultipartFile("my-image.png", []byte("hello image"))
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			t.Fatalf("Close() error: %v", err)
		}
	}()

	_, err = service.SaveAssetForPage(page, file, name, testAssetMaxBytes)
	if err != nil {
		t.Fatalf("SaveAsset failed: %v", err)
	}

	err = service.DeleteAllAssetsForPage(page)
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	files, err := service.ListAssetsForPage(page)
	if err != nil {
		t.Fatalf("ListAssets failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected no assets, got %d", len(files))
	}
}

func TestSlugCollision(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "collision-page"}
	service := NewAssetService(tmp, tree.NewSlugService())

	for i := 0; i < 3; i++ {
		file, name, err := test_utils.CreateMultipartFile("logo.png", []byte("image"))
		if err != nil {
			t.Fatalf("test_utils.CreateMultipartFile failed: %v", err)
		}
		defer func(f multipart.File) {
			if err := f.Close(); err != nil {
				t.Fatalf("Close() error: %v", err)
			}
		}(file)

		_, err = service.SaveAssetForPage(page, file, name, testAssetMaxBytes)
		if err != nil {
			t.Fatalf("upload %d failed: %v", i, err)
		}
	}

	files, err := service.ListAssetsForPage(page)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

func TestAssetRename(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "rename-page", ID: "c3d4"}
	// Create index.md page
	pagePath := filepath.Join(tmp, "rename-page")
	if err := os.MkdirAll(pagePath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pagePath, "index.md"), []byte("# Rename Page"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	service := NewAssetService(tmp, tree.NewSlugService())

	file, name, err := test_utils.CreateMultipartFile("old-name.png", []byte("old image"))
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			t.Fatalf("Close() error: %v", err)
		}
	}()

	if _, err := service.SaveAssetForPage(page, file, name, testAssetMaxBytes); err != nil {
		t.Fatalf("SaveAsset failed: %v", err)
	}

	newName := "new-name.png"
	newUrl, err := service.RenameAsset(page, name, newName)
	if err != nil {
		t.Fatalf("RenameAsset failed: %v", err)
	}

	if newUrl != "" && strings.Contains(newUrl, newName) == false {
		t.Errorf("expected new URL to contain new name %s, got %s", newName, newUrl)
	}

	files, err := service.ListAssetsForPage(page)
	if err != nil {
		t.Fatalf("ListAssets failed: %v", err)
	}

	expectedURL := "/assets/c3d4/new-name.png"
	if len(files) != 1 || files[0] != expectedURL {
		t.Errorf("unexpected asset list after rename: %v", files)
	}
}

func TestDeleteMissingAssetReturnsNotFound(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "delete-page", ID: "delete-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create asset directory: %v", err)
	}

	err := service.DeleteAsset(page, "missing.png")
	if err == nil {
		t.Fatal("expected delete to fail for missing asset")
	}

	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected localized error, got %T", err)
	}

	if localized.Code != "asset_not_found" {
		t.Fatalf("expected asset_not_found, got %s", localized.Code)
	}
}

func TestAssetDiskPaths_WindowsPath(t *testing.T) {
	if got, want := strings.ReplaceAll(assetPageDiskPath(`C:\wiki\data\assets`, "a7b3"), `\`, `/`), `C:/wiki/data/assets/a7b3`; got != want {
		t.Fatalf("asset page path = %q, want %q", got, want)
	}
	if got, want := strings.ReplaceAll(assetFileDiskPath(`C:\wiki\data\assets\a7b3`, "my-image.png"), `\`, `/`), `C:/wiki/data/assets/a7b3/my-image.png`; got != want {
		t.Fatalf("asset file path = %q, want %q", got, want)
	}
}

func TestValidateFilename(t *testing.T) {
	good := []string{"my-image.png", "file.jpg", "a", "foo-bar.webp"}
	for _, name := range good {
		if err := validateFilename(name); err != nil {
			t.Errorf("validateFilename(%q) unexpected error: %v", name, err)
		}
	}

	bad := []string{
		"",
		".",
		"..",
		"../etc/passwd",
		"../../users.db",
		"foo/bar.png",
		`foo\bar.png`,
	}
	for _, name := range bad {
		if err := validateFilename(name); err == nil {
			t.Errorf("validateFilename(%q) should have returned an error", name)
		}
	}
}

func TestDeleteAsset_PathTraversal(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "test-page", ID: "traversal-delete"}
	service := NewAssetService(tmp, tree.NewSlugService())

	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create asset directory: %v", err)
	}

	traversalNames := []string{
		"../../users.db",
		"../other-page/secret.png",
		"..",
		".",
		"",
		"foo/bar.png",
		`foo\bar.png`,
	}
	for _, name := range traversalNames {
		err := service.DeleteAsset(page, name)
		if err == nil {
			t.Errorf("DeleteAsset(%q) should have returned an error", name)
			continue
		}
		localized, ok := sharederrors.AsLocalizedError(err)
		if !ok {
			t.Errorf("DeleteAsset(%q): expected localized error, got %T: %v", name, err, err)
			continue
		}
		if localized.Code != "asset_invalid_name" {
			t.Errorf("DeleteAsset(%q): expected asset_invalid_name, got %s", name, localized.Code)
		}
	}
}

func TestRenameAsset_OldFilenamePathTraversal(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "test-page", ID: "traversal-rename"}
	service := NewAssetService(tmp, tree.NewSlugService())

	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create asset directory: %v", err)
	}

	traversalNames := []string{
		"../../users.db",
		"../other-page/secret.png",
		"..",
		".",
		"",
		"foo/bar.png",
		`foo\bar.png`,
	}
	for _, name := range traversalNames {
		_, err := service.RenameAsset(page, name, "new-name.png")
		if err == nil {
			t.Errorf("RenameAsset(oldFilename=%q) should have returned an error", name)
			continue
		}
		localized, ok := sharederrors.AsLocalizedError(err)
		if !ok {
			t.Errorf("RenameAsset(oldFilename=%q): expected localized error, got %T: %v", name, err, err)
			continue
		}
		if localized.Code != "asset_invalid_name" {
			t.Errorf("RenameAsset(oldFilename=%q): expected asset_invalid_name, got %s", name, localized.Code)
		}
	}
}

func TestAssetPublicPath_UsesForwardSlashes(t *testing.T) {
	service := NewAssetService(t.TempDir(), tree.NewSlugService())
	page := &tree.PageNode{ID: "a7b3"}

	if got, want := service.buildPublicPath(page, "my-image.png"), "/assets/a7b3/my-image.png"; got != want {
		t.Fatalf("public path = %q, want %q", got, want)
	}
}

func TestSaveAssetForPage_TooLarge_DoesNotLeavePartialFile(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "limit-page", ID: "limit-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	file, name, err := test_utils.CreateMultipartFile("too-large.bin", []byte(strings.Repeat("a", 32)))
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}
	}()

	_, err = service.SaveAssetForPage(page, file, name, 8)
	if !errors.Is(err, shared.ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}

	assetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	entries, readErr := os.ReadDir(assetDir)
	if readErr != nil {
		t.Fatalf("failed to read asset directory: %v", readErr)
	}

	if len(entries) != 0 {
		t.Fatalf("expected no files after failed upload, got %d", len(entries))
	}
}

func TestOpenAsset_ReadsTheFile(t *testing.T) {
	tmp := t.TempDir()
	service := NewAssetService(tmp, tree.NewSlugService())
	pageDir := filepath.Join(tmp, "assets", "a7b3")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "my image.png"), []byte("hello image"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	f, err := service.OpenAsset("a7b3", "my image.png")
	if err != nil {
		t.Fatalf("OpenAsset: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	body, err := io.ReadAll(f)
	if err != nil || string(body) != "hello image" {
		t.Fatalf("content = %q, %v, want hello image", body, err)
	}
}

// Anything that is not a regular file directly in the page's asset folder is
// ErrAssetNotFound, before or without touching the disk.
func TestOpenAsset_NotFound(t *testing.T) {
	tmp := t.TempDir()
	service := NewAssetService(tmp, tree.NewSlugService())
	pageDir := filepath.Join(tmp, "assets", "a7b3")
	if err := os.MkdirAll(filepath.Join(pageDir, "folder"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for name, content := range map[string]string{
		filepath.Join(tmp, "assets", "a7b3", "ok.png"):   "ok",
		filepath.Join(tmp, "assets", "other.png"):        "outside the page folder",
		filepath.Join(tmp, "secret.txt"):                 "outside the assets folder",
		filepath.Join(tmp, "assets", "b9c1", "peer.png"): "another page's asset",
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	tests := []struct{ name, pageID, filename string }{
		{"unknown file", "a7b3", "missing.png"},
		{"unknown page folder", "zzzz", "ok.png"},
		{"a directory", "a7b3", "folder"},
		{"empty name", "a7b3", ""},
		{"empty page id", "", "other.png"},
		{"dot dot name", "a7b3", ".."},
		{"traversal in name", "a7b3", "../other.png"},
		{"backslash traversal in name", "a7b3", `..\other.png`},
		{"traversal to another page", "a7b3", "../b9c1/peer.png"},
		{"dot dot page id", "..", "secret.txt"},
		{"traversal in page id", "../..", "secret.txt"},
		{"page id with a separator", "a7b3/folder", "ok.png"},
		{"dot page id", ".", "other.png"},
		{"absolute name", "a7b3", filepath.Join(tmp, "secret.txt")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := service.OpenAsset(tc.pageID, tc.filename)
			if f != nil {
				_ = f.Close()
			}
			if !errors.Is(err, ErrAssetNotFound) {
				t.Fatalf("OpenAsset(%q, %q) err = %v, want ErrAssetNotFound", tc.pageID, tc.filename, err)
			}
		})
	}
}

// A symlink inside the page's folder must not lead out of it.
func TestOpenAsset_DoesNotFollowSymlinksOut(t *testing.T) {
	tmp := t.TempDir()
	service := NewAssetService(tmp, tree.NewSlugService())
	pageDir := filepath.Join(tmp, "assets", "a7b3")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	secret := filepath.Join(tmp, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(pageDir, "link.png")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	f, err := service.OpenAsset("a7b3", "link.png")
	if f != nil {
		_ = f.Close()
	}
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("err = %v, want ErrAssetNotFound", err)
	}
}

// Names Windows would alias to another file or folder are rejected before the
// disk is touched, on every platform.
func TestValidateServedName_RejectsWindowsAliases(t *testing.T) {
	for _, name := range []string{"b9c1.", "b9c1 ", "a.png:secret", "a.png::$DATA", "C:x.png", "a\x00b", "a\tb", "a\x7fb", "..", "a/b"} {
		if err := validateServedName(name); err == nil {
			t.Errorf("validateServedName(%q) = nil, want an error", name)
		}
	}
	for _, name := range []string{"a7b3", "my image.png", "report.v2.pdf", "Bild-ä.png", "x_y-z"} {
		if err := validateServedName(name); err != nil {
			t.Errorf("validateServedName(%q) = %v, want nil", name, err)
		}
	}
}

// Names that Windows normalises or resolves specially (trailing dots and
// spaces, alternate data streams, 8.3 short names, device names, NUL) and names
// that are too long. Whatever such a name does on the platform, it is either
// ErrAssetNotFound or a file of the requested page's own folder; it never
// yields a file of another page or one outside the page's folder.
func TestOpenAsset_WindowsAliasesStayInThePageFolder(t *testing.T) {
	tmp := t.TempDir()
	service := NewAssetService(tmp, tree.NewSlugService())
	for name, content := range map[string]string{
		filepath.Join("a7b3", "salaries-2024.png"): "own",
		filepath.Join("b9c1", "salaries-2024.png"): "peer",
		filepath.Join("b9c1", "peer.png"):          "peer",
		"outside.png":                              "outside",
	} {
		path := filepath.Join(tmp, "assets", name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	tests := []struct{ name, pageID, filename, owner string }{
		{"trailing dot", "a7b3", "salaries-2024.png.", "own"},
		{"trailing space", "a7b3", "salaries-2024.png ", "own"},
		{"alternate data stream", "a7b3", "salaries-2024.png:secret", "own"},
		{"default data stream", "a7b3", "salaries-2024.png::$DATA", "own"},
		{"short 8.3 name", "a7b3", "SALARI~1.PNG", "own"},
		{"short 8.3 name of the peer file", "a7b3", "PEER~1.PNG", "own"},
		{"device name", "a7b3", "CON", "own"},
		{"device name with extension", "a7b3", "aux.png", "own"},
		{"nul device", "a7b3", "NUL", "own"},
		{"nul byte", "a7b3", "salaries-2024.png\x00.txt", "own"},
		{"drive-relative name", "a7b3", "C:peer.png", "own"},
		{"unicode name", "a7b3", "gehälter-日本.png", "own"},
		{"too long name", "a7b3", strings.Repeat("a", 300) + ".png", "own"},
		{"another page's file name", "a7b3", "peer.png", "own"},
		{"page id with a trailing dot", "b9c1.", "peer.png", "peer"},
		{"page id with a trailing space", "b9c1 ", "peer.png", "peer"},
		{"page id with a stream", "a7b3::$INDEX_ALLOCATION", "salaries-2024.png", "own"},
		{"page id that is a drive", "C:", "outside.png", "own"},
		{"page id that is a device", "NUL", "peer.png", "own"},
		{"page id with a nul byte", "a7b3\x00", "salaries-2024.png", "own"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := service.OpenAsset(tc.pageID, tc.filename)
			if err != nil {
				if !errors.Is(err, ErrAssetNotFound) {
					t.Fatalf("OpenAsset(%q, %q) err = %v, want ErrAssetNotFound or a file of the page", tc.pageID, tc.filename, err)
				}
				return
			}
			defer func() { _ = f.Close() }()
			body, readErr := io.ReadAll(f)
			if readErr != nil {
				t.Fatalf("ReadAll: %v", readErr)
			}
			if string(body) != tc.owner {
				t.Fatalf("OpenAsset(%q, %q) served %q, want only a file of the requested page (%q)", tc.pageID, tc.filename, body, tc.owner)
			}
		})
	}
}
