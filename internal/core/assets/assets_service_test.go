package assets

import (
	"errors"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/shared"
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

func TestAssetDiskPaths_WindowsPath(t *testing.T) {
	if got, want := strings.ReplaceAll(assetPageDiskPath(`C:\wiki\data\assets`, "a7b3"), `\`, `/`), `C:/wiki/data/assets/a7b3`; got != want {
		t.Fatalf("asset page path = %q, want %q", got, want)
	}
	if got, want := strings.ReplaceAll(assetFileDiskPath(`C:\wiki\data\assets\a7b3`, "my-image.png"), `\`, `/`), `C:/wiki/data/assets/a7b3/my-image.png`; got != want {
		t.Fatalf("asset file path = %q, want %q", got, want)
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
