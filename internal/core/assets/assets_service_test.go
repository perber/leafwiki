package assets

import (
	"bytes"
	"errors"
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
	assetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if _, err := os.Stat(assetDir); err != nil {
		t.Fatalf("expected asset directory before delete, got stat error: %v", err)
	}

	err = service.DeleteAllAssetsForPage(page)
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}
	if _, err := os.Stat(assetDir); !os.IsNotExist(err) {
		t.Fatalf("expected asset directory to be removed, got stat error: %v", err)
	}

	files, err := service.ListAssetsForPage(page)
	if err != nil {
		t.Fatalf("ListAssets failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected no assets, got %d", len(files))
	}
}

func TestDeleteLastAssetRemovesPageAssetDirectory(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "delete-page", ID: "delete-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	file, name, err := test_utils.CreateMultipartFile("only.png", []byte("hello image"))
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}
	}()

	if _, err := service.SaveAssetForPage(page, file, name, testAssetMaxBytes); err != nil {
		t.Fatalf("SaveAsset failed: %v", err)
	}
	assetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if _, err := os.Stat(assetDir); err != nil {
		t.Fatalf("expected asset directory before delete, got stat error: %v", err)
	}

	if err := service.DeleteAsset(page, name); err != nil {
		t.Fatalf("DeleteAsset failed: %v", err)
	}
	if _, err := os.Stat(assetDir); !os.IsNotExist(err) {
		t.Fatalf("expected asset directory to be removed after last delete, got stat error: %v", err)
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

func TestSaveAssetForPageRejectsInvalidNormalizedFilenames(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "upload-page", ID: "upload-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	for _, originalName := range []string{"", ".", ".."} {
		t.Run("name="+originalName, func(t *testing.T) {
			file := newTestMultipartFile([]byte("asset"))
			defer func() {
				if err := file.Close(); err != nil {
					t.Fatalf("Close() error: %v", err)
				}
			}()

			_, err := service.SaveAssetForPage(page, file, originalName, testAssetMaxBytes)
			if originalName == "" {
				assertLocalizedCode(t, err, "asset_missing_name")
			} else {
				assertLocalizedCode(t, err, "asset_invalid_name")
			}
		})
	}

	assetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	entries, err := os.ReadDir(assetDir)
	if err != nil {
		t.Fatalf("failed to read asset directory: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("invalid upload left asset directory entries: %v", names)
	}
}

func TestReadAssetForPageRejectsPathSeparators(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "read-page", ID: "read-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create asset directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pageAssetDir, "note.txt"), []byte("asset"), 0644); err != nil {
		t.Fatalf("failed to write asset: %v", err)
	}

	if _, err := service.ReadAssetForPage(page, "../note.txt"); err == nil {
		t.Fatalf("expected path separator asset read to fail")
	}
	if _, err := service.ReadAssetForPage(page, `..\note.txt`); err == nil {
		t.Fatalf("expected windows path separator asset read to fail")
	}
}

func TestReadAssetForPageRejectsDotNames(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "read-page", ID: "read-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create asset directory: %v", err)
	}

	for _, filename := range []string{".", ".."} {
		if _, err := service.ReadAssetForPage(page, filename); err == nil {
			t.Fatalf("expected dot asset read %q to fail", filename)
		} else {
			assertLocalizedCode(t, err, "asset_invalid_name")
		}
	}
}

func TestDeleteAssetRejectsPathSeparators(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "delete-page", ID: "delete-page-id"}
	other := &tree.PageNode{Slug: "other-page", ID: "other-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	otherAssetDir := filepath.Join(service.GetAssetsDir(), other.ID)
	if err := os.MkdirAll(otherAssetDir, 0755); err != nil {
		t.Fatalf("failed to create other asset directory: %v", err)
	}
	otherAsset := filepath.Join(otherAssetDir, "note.txt")
	if err := os.WriteFile(otherAsset, []byte("other asset"), 0644); err != nil {
		t.Fatalf("failed to write other asset: %v", err)
	}
	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create page asset directory: %v", err)
	}

	if err := service.DeleteAsset(page, "../"+other.ID+"/note.txt"); err == nil {
		t.Fatalf("expected path-bearing delete to fail")
	}
	if _, err := os.Stat(otherAsset); err != nil {
		t.Fatalf("path-bearing delete affected other page asset: %v", err)
	}
}

func TestDeleteAssetRejectsDotNames(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "delete-page", ID: "delete-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create page asset directory: %v", err)
	}

	for _, filename := range []string{".", ".."} {
		err := service.DeleteAsset(page, filename)
		assertLocalizedCode(t, err, "asset_invalid_name")
		if _, statErr := os.Stat(pageAssetDir); statErr != nil {
			t.Fatalf("dot delete affected page asset directory: %v", statErr)
		}
	}
}

func TestRenameAssetRejectsPathSeparators(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "rename-page", ID: "rename-page-id"}
	other := &tree.PageNode{Slug: "other-page", ID: "other-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create page asset directory: %v", err)
	}
	pageAsset := filepath.Join(pageAssetDir, "note.txt")
	if err := os.WriteFile(pageAsset, []byte("page asset"), 0644); err != nil {
		t.Fatalf("failed to write page asset: %v", err)
	}
	otherAssetDir := filepath.Join(service.GetAssetsDir(), other.ID)
	if err := os.MkdirAll(otherAssetDir, 0755); err != nil {
		t.Fatalf("failed to create other asset directory: %v", err)
	}
	otherAsset := filepath.Join(otherAssetDir, "note.txt")
	if err := os.WriteFile(otherAsset, []byte("other asset"), 0644); err != nil {
		t.Fatalf("failed to write other asset: %v", err)
	}

	if _, err := service.RenameAsset(page, "../"+other.ID+"/note.txt", "renamed.txt"); err == nil {
		t.Fatalf("expected path-bearing old filename rename to fail")
	}
	if _, err := os.Stat(otherAsset); err != nil {
		t.Fatalf("path-bearing old filename rename affected other page asset: %v", err)
	}
	if _, err := service.RenameAsset(page, "note.txt", "../"+other.ID+"/renamed.txt"); err == nil {
		t.Fatalf("expected path-bearing new filename rename to fail")
	}
	if _, err := os.Stat(pageAsset); err != nil {
		t.Fatalf("path-bearing new filename rename affected page asset: %v", err)
	}
}

func TestRenameAssetRejectsDotNames(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "rename-page", ID: "rename-page-id"}
	service := NewAssetService(tmp, tree.NewSlugService())

	pageAssetDir := filepath.Join(service.GetAssetsDir(), page.ID)
	if err := os.MkdirAll(pageAssetDir, 0755); err != nil {
		t.Fatalf("failed to create page asset directory: %v", err)
	}
	pageAsset := filepath.Join(pageAssetDir, "note.txt")
	if err := os.WriteFile(pageAsset, []byte("page asset"), 0644); err != nil {
		t.Fatalf("failed to write page asset: %v", err)
	}

	for _, filename := range []string{".", ".."} {
		_, err := service.RenameAsset(page, filename, "renamed.txt")
		assertLocalizedCode(t, err, "asset_invalid_name")
	}
	for _, filename := range []string{".", ".."} {
		_, err := service.RenameAsset(page, "note.txt", filename)
		assertLocalizedCode(t, err, "asset_invalid_name")
	}
	if _, err := os.Stat(pageAsset); err != nil {
		t.Fatalf("dot rename affected page asset: %v", err)
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

	err := service.DeleteAsset(page, "")
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("DeleteAsset(\"\"): expected localized error, got %T: %v", err, err)
	}
	if localized.Code != "asset_missing_name" {
		t.Fatalf("DeleteAsset(\"\"): expected asset_missing_name, got %s", localized.Code)
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

	_, err := service.RenameAsset(page, "", "new-name.png")
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("RenameAsset(oldFilename=\"\"): expected localized error, got %T: %v", err, err)
	}
	if localized.Code != "asset_missing_name" {
		t.Fatalf("RenameAsset(oldFilename=\"\"): expected asset_missing_name, got %s", localized.Code)
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

func assertLocalizedCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %s error, got nil", want)
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected localized error %s, got %T: %v", want, err, err)
	}
	if localized.Code != want {
		t.Fatalf("error code = %s, want %s", localized.Code, want)
	}
}

type testMultipartFile struct {
	*bytes.Reader
}

func newTestMultipartFile(content []byte) multipart.File {
	return &testMultipartFile{Reader: bytes.NewReader(content)}
}

func (f *testMultipartFile) Close() error {
	return nil
}
