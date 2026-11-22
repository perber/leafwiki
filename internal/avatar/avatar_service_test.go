package avatar

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/webp"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// fakeMultipartFile adapts a bytes.Reader to multipart.File (io.Reader +
// io.ReaderAt + io.Seeker + io.Closer), which is all UploadAvatar needs.
type fakeMultipartFile struct {
	*bytes.Reader
}

func (f fakeMultipartFile) Close() error { return nil }

func newMultipartFile(data []byte) multipart.File {
	return fakeMultipartFile{bytes.NewReader(data)}
}

func newTestAvatarService(t *testing.T) (*AvatarService, string) {
	t.Helper()
	dir := t.TempDir()
	svc, err := NewAvatarService(dir)
	if err != nil {
		t.Fatalf("NewAvatarService() error: %v", err)
	}
	return svc, dir
}

// quadrantImage builds a rectWidth x rectHeight RGBA image split into four
// solid-colored quadrants, used to verify that cropToSquare anchors at the
// image's center rather than a corner: only a center-anchored crop keeps all
// four colors visible near the crop's own corners in predictable positions.
func quadrantImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	colors := [4]color.RGBA{
		{255, 0, 0, 255},   // top-left: red
		{0, 255, 0, 255},   // top-right: green
		{0, 0, 255, 255},   // bottom-left: blue
		{255, 255, 0, 255}, // bottom-right: yellow
	}
	midX, midY := width/2, height/2
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var c color.RGBA
			switch {
			case x < midX && y < midY:
				c = colors[0]
			case x >= midX && y < midY:
				c = colors[1]
			case x < midX && y >= midY:
				c = colors[2]
			default:
				c = colors[3]
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode() error: %v", err)
	}
	return buf.Bytes()
}

func encodeGIF(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("gif.Encode() error: %v", err)
	}
	return buf.Bytes()
}

// solidSquarePNG builds a small solid-color square PNG for the simple
// format-acceptance tests (quadrantImage is reserved for the crop-centering
// test, which needs a non-square source).
func solidSquarePNG(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.RGBA{10, 20, 30, 255})
		}
	}
	return encodePNG(t, img)
}

func TestAvatarService_UploadAvatar_ValidFormats_ProducesTargetSizePNG(t *testing.T) {
	square := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			square.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}

	tests := []struct {
		name string
		data []byte
		ext  string
	}{
		{"jpeg", encodeJPEG(t, square), ".jpg"},
		{"png", encodePNG(t, square), ".png"},
		{"gif", encodeGIF(t, square), ".gif"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dir := newTestAvatarService(t)
			if err := svc.UploadAvatar("user-1", newMultipartFile(tt.data), "avatar"+tt.ext); err != nil {
				t.Fatalf("UploadAvatar() error: %v", err)
			}

			path := filepath.Join(dir, "avatars", "user-1.png")
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("expected avatar file at %s: %v", path, err)
			}
			defer func() { _ = f.Close() }()

			cfg, err := png.DecodeConfig(f)
			if err != nil {
				t.Fatalf("stored avatar is not a valid PNG: %v", err)
			}
			if cfg.Width != TargetSize || cfg.Height != TargetSize {
				t.Errorf("expected %dx%d, got %dx%d", TargetSize, TargetSize, cfg.Width, cfg.Height)
			}
		})
	}
}

func TestAvatarService_UploadAvatar_WebpFormat_ProducesTargetSizePNG(t *testing.T) {
	// golang.org/x/image/webp is decode-only (no encoder), so this fixture
	// was generated externally (Pillow, lossless WEBP) as a 100x100 solid
	// color image and pinned here as raw bytes; verified to round-trip
	// through golang.org/x/image/webp.Decode below before use.
	data := []byte{
		0x52, 0x49, 0x46, 0x46, 0x24, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50,
		0x56, 0x50, 0x38, 0x4c, 0x17, 0x00, 0x00, 0x00, 0x2f, 0x63, 0xc0, 0x18,
		0x00, 0x07, 0x50, 0x8a, 0x2a, 0xd4, 0xa3, 0xff, 0x61, 0x00, 0x12, 0xc2,
		0xff, 0xfd, 0x52, 0x44, 0xff, 0x53, 0x89, 0x00,
	}
	if _, err := webp.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("test fixture is not a valid webp: %v", err)
	}

	svc, dir := newTestAvatarService(t)
	if err := svc.UploadAvatar("user-webp", newMultipartFile(data), "avatar.webp"); err != nil {
		t.Fatalf("UploadAvatar() error: %v", err)
	}

	path := filepath.Join(dir, "avatars", "user-webp.png")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("expected avatar file at %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatalf("stored avatar is not a valid PNG: %v", err)
	}
	if cfg.Width != TargetSize || cfg.Height != TargetSize {
		t.Errorf("expected %dx%d, got %dx%d", TargetSize, TargetSize, cfg.Width, cfg.Height)
	}
}

func TestAvatarService_UploadAvatar_RejectedExtension_ReturnsInvalidTypeError(t *testing.T) {
	svc, _ := newTestAvatarService(t)
	data := solidSquarePNG(t, 100)

	err := svc.UploadAvatar("user-1", newMultipartFile(data), "avatar.bmp")
	if err == nil {
		t.Fatal("expected an error for a disallowed extension")
	}
}

func TestAvatarService_UploadAvatar_CorruptBytes_ReturnsDecodeFailedError(t *testing.T) {
	svc, _ := newTestAvatarService(t)
	garbage := []byte("this is not an image, just plain garbage bytes")

	err := svc.UploadAvatar("user-1", newMultipartFile(garbage), "avatar.png")
	if err == nil {
		t.Fatal("expected a decode error for corrupt image bytes")
	}
}

func TestAvatarService_UploadAvatar_OversizedUpload_Rejected(t *testing.T) {
	svc, _ := newTestAvatarService(t)

	// Build a PNG payload whose encoded byte size exceeds MaxUploadSize.
	// Pseudo-random per-pixel noise defeats PNG's filters/deflate, so a
	// moderate resolution is enough to comfortably clear the 5 MiB limit
	// without needing an enormous image.
	size := 1600
	rng := rand.New(rand.NewSource(1))
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	noise := make([]byte, size*size*4)
	if _, err := rng.Read(noise); err != nil {
		t.Fatalf("rng.Read() error: %v", err)
	}
	copy(img.Pix, noise)
	data := encodePNG(t, img)
	if int64(len(data)) <= MaxUploadSize {
		t.Fatalf("test fixture too small to exceed MaxUploadSize: %d bytes", len(data))
	}

	err := svc.UploadAvatar("user-1", newMultipartFile(data), "avatar.png")
	if err == nil {
		t.Fatal("expected an error for an oversized upload")
	}
}

func TestAvatarService_UploadAvatar_NonSquareInput_CropsCenteredNotCorner(t *testing.T) {
	svc, dir := newTestAvatarService(t)

	// A wide rectangle: the center-square crop should span x in [100, 300)
	// (i.e. the middle 200px of a 400px-wide, 200px-tall image), keeping all
	// four color quadrants visible in the output — a corner-anchored crop
	// would instead show only two of the four colors.
	src := quadrantImage(400, 200)
	data := encodePNG(t, src)

	if err := svc.UploadAvatar("user-1", newMultipartFile(data), "avatar.png"); err != nil {
		t.Fatalf("UploadAvatar() error: %v", err)
	}

	path := filepath.Join(dir, "avatars", "user-1.png")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("expected avatar file at %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	out, err := png.Decode(f)
	if err != nil {
		t.Fatalf("stored avatar is not a valid PNG: %v", err)
	}
	if out.Bounds().Dx() != TargetSize || out.Bounds().Dy() != TargetSize {
		t.Fatalf("expected %dx%d output, got %dx%d", TargetSize, TargetSize, out.Bounds().Dx(), out.Bounds().Dy())
	}

	// Sample near each corner of the resized output (inset slightly to avoid
	// resampling blur exactly at the crop boundary) and confirm all four
	// original quadrant colors are represented — proof the crop was centered
	// (source x in [100,300), y in [0,200)) rather than anchored at a
	// corner (which would only ever show 2 of the 4 colors).
	inset := TargetSize / 8
	corners := map[string][2]int{
		"top-left":     {inset, inset},
		"top-right":    {TargetSize - 1 - inset, inset},
		"bottom-left":  {inset, TargetSize - 1 - inset},
		"bottom-right": {TargetSize - 1 - inset, TargetSize - 1 - inset},
	}
	seen := map[string]bool{}
	closestColorName := func(c color.Color) string {
		r, g, b, _ := c.RGBA()
		r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
		switch {
		case r8 > 150 && g8 < 100 && b8 < 100:
			return "red"
		case r8 < 100 && g8 > 150 && b8 < 100:
			return "green"
		case r8 < 100 && g8 < 100 && b8 > 150:
			return "blue"
		case r8 > 150 && g8 > 150 && b8 < 100:
			return "yellow"
		default:
			return "unknown"
		}
	}
	for corner, xy := range corners {
		name := closestColorName(out.At(xy[0], xy[1]))
		seen[name] = true
		t.Logf("corner %s -> color %s", corner, name)
	}
	if len(seen) < 4 {
		t.Errorf("expected all 4 quadrant colors visible in a centered crop, saw: %v", seen)
	}
}

func TestAvatarService_DeleteAvatar_IdempotentAndNoOpWhenNothingUploaded(t *testing.T) {
	svc, dir := newTestAvatarService(t)

	// Deleting when nothing was ever uploaded must not error.
	if err := svc.DeleteAvatar("no-such-user"); err != nil {
		t.Fatalf("DeleteAvatar() on nonexistent user error: %v", err)
	}

	data := solidSquarePNG(t, 100)
	if err := svc.UploadAvatar("user-1", newMultipartFile(data), "avatar.png"); err != nil {
		t.Fatalf("UploadAvatar() error: %v", err)
	}
	path := filepath.Join(dir, "avatars", "user-1.png")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected avatar file to exist before delete: %v", err)
	}

	if err := svc.DeleteAvatar("user-1"); err != nil {
		t.Fatalf("DeleteAvatar() error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected avatar file removed, got err=%v", err)
	}

	// Deleting again is a no-op, not an error.
	if err := svc.DeleteAvatar("user-1"); err != nil {
		t.Fatalf("DeleteAvatar() second call error: %v", err)
	}
}

func TestAvatarService_AvatarPath_ReportsExistence(t *testing.T) {
	svc, _ := newTestAvatarService(t)

	if _, found := svc.AvatarPath("user-1"); found {
		t.Error("expected AvatarPath to report not-found before any upload")
	}

	data := solidSquarePNG(t, 100)
	if err := svc.UploadAvatar("user-1", newMultipartFile(data), "avatar.png"); err != nil {
		t.Fatalf("UploadAvatar() error: %v", err)
	}

	path, found := svc.AvatarPath("user-1")
	if !found {
		t.Fatal("expected AvatarPath to report found after upload")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("AvatarPath returned a path that does not exist: %v", err)
	}
}

func TestAvatarService_UploadAvatar_UnsafeUserID_Rejected(t *testing.T) {
	svc, _ := newTestAvatarService(t)
	data := solidSquarePNG(t, 100)

	for _, id := range []string{"../escape", "a/b", "a\\b", "/abs"} {
		if err := svc.UploadAvatar(id, newMultipartFile(data), "avatar.png"); err == nil {
			t.Errorf("expected UploadAvatar to reject unsafe user id %q", id)
		}
	}
}

var _ io.ReaderAt = fakeMultipartFile{} // sanity: satisfies multipart.File

// withDeclaredSize rewrites the width and height an encoded image declares in
// its header while leaving the (small) pixel data untouched — the shape of a
// decompression bomb: a few bytes on the wire that claim a huge canvas
// (GHSA-p295-c79q-qjg7).
func withDeclaredSize(t *testing.T, format string, data []byte, width, height int) []byte {
	t.Helper()
	out := bytes.Clone(data)
	switch format {
	case "png":
		// IHDR is the first chunk: 8-byte signature, 4-byte length, "IHDR",
		// then width and height (big-endian), and a CRC over type + data.
		binary.BigEndian.PutUint32(out[16:20], uint32(width))
		binary.BigEndian.PutUint32(out[20:24], uint32(height))
		binary.BigEndian.PutUint32(out[29:33], crc32.ChecksumIEEE(out[12:29]))
	case "gif":
		// Logical screen width and height follow the 6-byte "GIF89a" header.
		binary.LittleEndian.PutUint16(out[6:8], uint16(width))
		binary.LittleEndian.PutUint16(out[8:10], uint16(height))
	case "jpeg":
		// SOF0 (FF C0): 2-byte length, 1-byte precision, height, width.
		i := bytes.Index(out, []byte{0xFF, 0xC0})
		if i < 0 {
			t.Fatal("no SOF0 marker in JPEG fixture")
		}
		binary.BigEndian.PutUint16(out[i+5:i+7], uint16(height))
		binary.BigEndian.PutUint16(out[i+7:i+9], uint16(width))
	default:
		t.Fatalf("unsupported fixture format %q", format)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != width || cfg.Height != height {
		t.Fatalf("patched %s fixture declares %dx%d (err %v), want %dx%d", format, cfg.Width, cfg.Height, err, width, height)
	}
	return out
}

func TestAvatarService_UploadAvatar_HugeDeclaredDimensions_RejectedBeforeDecode(t *testing.T) {
	small := image.NewRGBA(image.Rect(0, 0, 16, 16))
	cases := []struct {
		format, filename string
		data             []byte
	}{
		{"png", "avatar.png", encodePNG(t, small)},
		{"gif", "avatar.gif", encodeGIF(t, small)},
		{"jpeg", "avatar.jpg", encodeJPEG(t, small)},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			svc, dir := newTestAvatarService(t)
			bomb := withDeclaredSize(t, tc.format, tc.data, 5000, 5000)

			err := svc.UploadAvatar("user-1", newMultipartFile(bomb), tc.filename)

			localized, ok := sharederrors.AsLocalizedError(err)
			if !ok {
				t.Fatalf("expected a LocalizedError, got %v", err)
			}
			if localized.Code != "avatar_invalid_dimensions" || localized.Message != "Avatar image dimensions too large" {
				t.Fatalf("got code %q message %q, want the dimension-limit rejection", localized.Code, localized.Message)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "avatars", "user-1.png")); !os.IsNotExist(statErr) {
				t.Fatalf("no avatar must be stored for a rejected upload, stat err: %v", statErr)
			}
		})
	}
}

func TestCheckDimensions(t *testing.T) {
	cases := []struct {
		name          string
		width, height int
		wantOK        bool
	}{
		{"typical phone photo", 4032, 3024, true},
		{"square at the pixel cap", 4096, 4096, true},
		{"one row over the pixel cap", 4096, 4097, false},
		{"long strip within the pixel cap", 16384, 1024, true},
		{"side over the per-side cap", MaxImageSide + 1, 1, false},
		// 65535² overflows a 32-bit int (armv7 Raspberry Pi builds).
		{"largest GIF/JPEG canvas", 65535, 65535, false},
		{"zero width", 0, 100, false},
		{"negative height", 100, -1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkDimensions(tc.width, tc.height) == nil; got != tc.wantOK {
				t.Fatalf("checkDimensions(%d, %d) ok = %v, want %v", tc.width, tc.height, got, tc.wantOK)
			}
		})
	}
}

// Even within MaxImagePixels one decode can take a few hundred MiB (a
// progressive CMYK JPEG), so uploads decode one at a time instead of
// multiplying that by the number of parallel requests.
func TestAvatarService_UploadAvatar_DecodesOneAtATime(t *testing.T) {
	svc, _ := newTestAvatarService(t)
	data := solidSquarePNG(t, 32)

	svc.decodeSlot <- struct{}{} // another upload is decoding
	done := make(chan error, 1)
	go func() { done <- svc.UploadAvatar("user-1", newMultipartFile(data), "avatar.png") }()

	select {
	case err := <-done:
		t.Fatalf("upload finished (err %v) while another decode held the slot", err)
	case <-time.After(100 * time.Millisecond):
	}

	<-svc.decodeSlot
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("UploadAvatar() error after the slot was freed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("upload did not finish after the slot was freed")
	}
}
