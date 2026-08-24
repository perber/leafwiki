package avatar

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	coreavatar "github.com/perber/wiki/internal/avatar"
)

type fakeMultipartFile struct {
	*bytes.Reader
}

func (f fakeMultipartFile) Close() error { return nil }

func newMultipartFile(data []byte) multipart.File {
	return fakeMultipartFile{bytes.NewReader(data)}
}

func solidSquarePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{50, 60, 70, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func setupAvatarUseCases(t *testing.T) (*UploadAvatarUseCase, *DeleteAvatarUseCase, string) {
	t.Helper()
	dir := t.TempDir()
	svc, err := coreavatar.NewAvatarService(dir)
	if err != nil {
		t.Fatalf("NewAvatarService: %v", err)
	}
	return NewUploadAvatarUseCase(svc), NewDeleteAvatarUseCase(svc), dir
}

func TestUploadAvatarUseCase_Execute_ValidImage_WritesAvatarFile(t *testing.T) {
	upload, _, dir := setupAvatarUseCases(t)

	if err := upload.Execute(context.Background(), "user-1", newMultipartFile(solidSquarePNG(t)), "avatar.png"); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "avatars", "user-1.png")); err != nil {
		t.Fatalf("expected avatar file to exist: %v", err)
	}
}

func TestUploadAvatarUseCase_Execute_InvalidExtension_ReturnsError(t *testing.T) {
	upload, _, _ := setupAvatarUseCases(t)

	if err := upload.Execute(context.Background(), "user-1", newMultipartFile(solidSquarePNG(t)), "avatar.bmp"); err == nil {
		t.Fatal("expected an error for a disallowed extension")
	}
}

func TestDeleteAvatarUseCase_Execute_RemovesUploadedAvatar(t *testing.T) {
	upload, del, dir := setupAvatarUseCases(t)

	if err := upload.Execute(context.Background(), "user-1", newMultipartFile(solidSquarePNG(t)), "avatar.png"); err != nil {
		t.Fatalf("Execute (upload): %v", err)
	}
	path := filepath.Join(dir, "avatars", "user-1.png")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected avatar file to exist before delete: %v", err)
	}

	if err := del.Execute(context.Background(), "user-1"); err != nil {
		t.Fatalf("Execute (delete): %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected avatar file removed, got err=%v", err)
	}
}

func TestDeleteAvatarUseCase_Execute_NothingUploaded_NoOp(t *testing.T) {
	_, del, _ := setupAvatarUseCases(t)

	if err := del.Execute(context.Background(), "user-with-no-avatar"); err != nil {
		t.Fatalf("expected no error deleting a nonexistent avatar, got: %v", err)
	}
}
