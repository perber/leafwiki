package assets

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

// createMultipartFile simulates a real file upload using multipart encoding
func createMultipartFile(filename string, content []byte) (multipart.File, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	part.Write(content)
	writer.Close()

	reader := multipart.NewReader(body, writer.Boundary())
	form, err := reader.ReadForm(10 << 20)
	if err != nil {
		return nil, "", err
	}

	files := form.File["file"]
	if len(files) == 0 {
		return nil, "", fmt.Errorf("no file found in form")
	}

	f, err := files[0].Open()
	return f, files[0].Filename, err
}

func TestSaveAndListAsset(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "lonely-page"}
	// Create index.md page
	pagePath := filepath.Join(tmp, "lonely-page")
	os.MkdirAll(pagePath, 0755)
	os.WriteFile(filepath.Join(pagePath, "index.md"), []byte("# Lonely Page"), 0644)
	service := NewAssetService(tmp, tree.NewSlugService())

	file, name, err := createMultipartFile("my-image.png", []byte("hello image"))
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	url, err := service.SaveAssetForPage(page, file, name)
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

	if len(files) != 1 || files[0] != "/assets/lonely-page/assets/my-image.png" {
		t.Errorf("unexpected asset list: %v", files)
	}
}

func TestDeleteAssetAndFold(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "lonely-page"}
	// Create index.md page
	pagePath := filepath.Join(tmp, "lonely-page")
	os.MkdirAll(pagePath, 0755)
	os.WriteFile(filepath.Join(pagePath, "index.md"), []byte("# Lonely Page"), 0644)
	service := NewAssetService(tmp, tree.NewSlugService())

	file, name, err := createMultipartFile("single.png", []byte("content"))
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	_, err = service.SaveAssetForPage(page, file, name)
	if err != nil {
		t.Fatalf("SaveAsset failed: %v", err)
	}

	err = service.DeleteAsset(page, name)
	if err != nil {
		t.Fatalf("DeleteAsset failed: %v", err)
	}

	assetDir := filepath.Join(tmp, "lonely-page", "assets")
	if _, err := os.Stat(assetDir); !os.IsNotExist(err) {
		t.Errorf("expected assets dir to be deleted, but found")
	}

	indexPath := filepath.Join(tmp, "lonely-page", "index.md")
	flatPath := filepath.Join(tmp, "lonely-page.md")

	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		t.Errorf("expected index.md to be gone")
	}
	if _, err := os.Stat(flatPath); err != nil {
		t.Errorf("expected flat file to exist again")
	}
}

func TestSlugCollision(t *testing.T) {
	tmp := t.TempDir()
	page := &tree.PageNode{Slug: "collision-page"}
	service := NewAssetService(tmp, tree.NewSlugService())

	for i := 0; i < 3; i++ {
		file, name, err := createMultipartFile("logo.png", []byte("image"))
		if err != nil {
			t.Fatalf("createMultipartFile failed: %v", err)
		}
		defer file.Close()

		_, err = service.SaveAssetForPage(page, file, name)
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
