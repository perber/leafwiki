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

// avatarFixture holds both avatar use cases over one AvatarService rooted in a temp dir.
type avatarFixture struct {
	upload *UploadAvatarUseCase
	del    *DeleteAvatarUseCase
	dir    string
}

func newAvatarFixture(t *testing.T) *avatarFixture {
	t.Helper()
	dir := t.TempDir()
	svc, err := coreavatar.NewAvatarService(dir)
	if err != nil {
		t.Fatalf("NewAvatarService: %v", err)
	}
	return &avatarFixture{upload: NewUploadAvatarUseCase(svc), del: NewDeleteAvatarUseCase(svc), dir: dir}
}

// uploadPNG uploads a valid PNG named filename as the avatar of userID.
func (f *avatarFixture) uploadPNG(t *testing.T, userID, filename string) error {
	t.Helper()
	return f.upload.Execute(context.Background(), UploadAvatarInput{
		UserID:   userID,
		File:     newMultipartFile(solidSquarePNG(t)),
		Filename: filename,
	})
}

// avatarPath is where AvatarService stores the PNG avatar of userID.
func (f *avatarFixture) avatarPath(userID string) string {
	return filepath.Join(f.dir, "avatars", userID+".png")
}

func TestUploadAvatarUseCase_Execute_ValidImage_WritesAvatarFile(t *testing.T) {
	f := newAvatarFixture(t)

	if err := f.uploadPNG(t, "user-1", "avatar.png"); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := os.Stat(f.avatarPath("user-1")); err != nil {
		t.Fatalf("expected avatar file to exist: %v", err)
	}
}

func TestUploadAvatarUseCase_Execute_InvalidExtension_ReturnsError(t *testing.T) {
	f := newAvatarFixture(t)

	if err := f.uploadPNG(t, "user-1", "avatar.bmp"); err == nil {
		t.Fatal("expected an error for a disallowed extension")
	}
}

func TestDeleteAvatarUseCase_Execute_RemovesUploadedAvatar(t *testing.T) {
	f := newAvatarFixture(t)

	if err := f.uploadPNG(t, "user-1", "avatar.png"); err != nil {
		t.Fatalf("Execute (upload): %v", err)
	}
	path := f.avatarPath("user-1")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected avatar file to exist before delete: %v", err)
	}

	if err := f.del.Execute(context.Background(), DeleteAvatarInput{UserID: "user-1"}); err != nil {
		t.Fatalf("Execute (delete): %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected avatar file removed, got err=%v", err)
	}
}

func TestDeleteAvatarUseCase_Execute_NothingUploaded_NoOp(t *testing.T) {
	f := newAvatarFixture(t)

	if err := f.del.Execute(context.Background(), DeleteAvatarInput{UserID: "user-with-no-avatar"}); err != nil {
		t.Fatalf("expected no error deleting a nonexistent avatar, got: %v", err)
	}
}
