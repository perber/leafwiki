package test_utils

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
)

// CreateMultipartFile simulates a real file upload using multipart encoding
func CreateMultipartFile(filename string, content []byte) (multipart.File, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", err
	}
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

func WriteFile(t *testing.T, base, rel, content string) string {
	t.Helper()
	abs := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return abs
}
