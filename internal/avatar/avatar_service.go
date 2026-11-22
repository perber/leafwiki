// Package avatar provides self-service profile-picture storage: a single
// user uploads an image, it is center-cropped to a square, resized to a
// fixed target size, and stored as one deterministic PNG per user
// (avatars/<userID>.png) — no extension-guessing or cleanup machinery is
// needed since every upload lands at the same path regardless of input
// format. Mirrors internal/branding's structure (a service over a plain
// storage directory) but without branding's JSON-config layer, since an
// avatar has no metadata beyond "does a file exist for this user".
package avatar

import (
	"bytes"
	"fmt"
	"image"

	// Blank-imported so image.Decode recognizes these formats' magic bytes.
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // decode-only webp support (blank-imported like the stdlib formats above)

	"github.com/perber/wiki/internal/branding"
	"github.com/perber/wiki/internal/core/shared"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// MaxUploadSize is the largest accepted avatar upload, in bytes.
const MaxUploadSize = 5 << 20 // 5 MiB

// TargetSize is the width and height, in pixels, every stored avatar is
// resized to after being center-cropped to a square.
const TargetSize = 256

// MaxImagePixels caps the decoded size of an uploaded image. MaxUploadSize
// only bounds the compressed bytes: a few KB of PNG/GIF/WebP can declare a
// canvas that image.Decode would allocate gigabytes for (GHSA-p295-c79q-qjg7),
// so the declared dimensions are checked before any pixel is decoded.
// 4096x4096 still covers the 12-megapixel photos phones produce. Memory per
// pixel depends on the format: 8 B for a 16-bit RGBA PNG, but about 24 B for
// a progressive CMYK JPEG (coefficient buffers plus the decoded image), so a
// single decode can still reach ~390 MiB — which is why decodes are also
// serialised (see AvatarService.decodeSlot).
const MaxImagePixels = 4096 * 4096

// MaxImageSide caps either dimension on its own, so a strip that stays under
// MaxImagePixels cannot be arbitrarily long.
const MaxImageSide = 16384

// allowedExts are the accepted input file extensions (lower-case, with a
// leading dot), independent of the always-PNG output format.
var allowedExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

// AllowedExts returns the accepted input file extensions, sorted for a
// deterministic order — surfaced via GET /api/config so the frontend can
// build its file-picker accept list and hint text from the same source of
// truth as the server-side validation, instead of duplicating the list.
func AllowedExts() []string {
	exts := make([]string, 0, len(allowedExts))
	for ext := range allowedExts {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

func allowedExtsAsString() string {
	return strings.Join(AllowedExts(), ", ")
}

// AvatarService stores per-user avatar images on disk under
// <storageDir>/avatars/<userID>.png.
type AvatarService struct {
	avatarsDir string
	// decodeSlot lets one upload decode at a time, so parallel requests
	// cannot multiply the per-decode memory bound (GHSA-p295-c79q-qjg7).
	// Avatar uploads are rare, so the queueing is not noticeable.
	decodeSlot chan struct{}
}

// NewAvatarService creates the avatars directory (if missing) under
// storageDir and returns a service ready to accept uploads.
func NewAvatarService(storageDir string) (*AvatarService, error) {
	avatarsDir := filepath.Join(storageDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create avatars directory: %w", err)
	}
	return &AvatarService{avatarsDir: avatarsDir, decodeSlot: make(chan struct{}, 1)}, nil
}

// UploadAvatar validates, decodes, center-crops to a square, resizes to
// TargetSize x TargetSize, and atomically writes the result as
// avatars/<userID>.png — replacing any previous avatar for this user.
func (s *AvatarService) UploadAvatar(userID string, file multipart.File, filename string) error {
	if branding.ContainsUnsafePath(userID) {
		return sharederrors.NewLocalizedError(
			"avatar_invalid_type",
			"Invalid avatar upload",
			"invalid user id for avatar upload",
			nil,
		)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExts[ext] {
		return sharederrors.NewLocalizedError(
			"avatar_invalid_type",
			"Invalid avatar file type",
			"invalid avatar file type %s (allowed: %s)",
			nil,
			ext,
			allowedExtsAsString(),
		)
	}

	// Defense in depth: the HTTP handler already wraps the request body in
	// http.MaxBytesReader(MaxUploadSize) before this is called (see
	// internal/http.ParseUploadedFile), but UploadAvatar is a public service
	// method that could be invoked outside that path — so enforce the same
	// cap here too, before decoding, rather than trusting the caller.
	var buf bytes.Buffer
	if err := shared.CopyWithLimit(&buf, file, MaxUploadSize); err != nil {
		return sharederrors.NewLocalizedError(
			"avatar_upload_failed",
			"Avatar file too large",
			"avatar file too large",
			err,
		)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return sharederrors.NewLocalizedError(
			"avatar_decode_failed",
			"Failed to decode avatar image",
			"failed to decode avatar image",
			err,
		)
	}
	if err := checkDimensions(cfg.Width, cfg.Height); err != nil {
		return err
	}

	resized, err := s.decodeAndResize(buf.Bytes())
	if err != nil {
		return sharederrors.NewLocalizedError(
			"avatar_decode_failed",
			"Failed to decode avatar image",
			"failed to decode avatar image",
			err,
		)
	}

	var out bytes.Buffer
	if err := png.Encode(&out, resized); err != nil {
		return sharederrors.NewLocalizedError(
			"avatar_upload_failed",
			"Failed to encode avatar image",
			"failed to encode avatar image",
			err,
		)
	}

	targetPath := s.avatarPath(userID)
	if err := shared.WriteStreamAtomic(targetPath, &out, MaxUploadSize, 0o644); err != nil {
		return sharederrors.NewLocalizedError(
			"avatar_upload_failed",
			"Failed to save avatar file",
			"failed to save avatar file",
			err,
		)
	}

	return nil
}

// DeleteAvatar removes userID's stored avatar, if any. Deleting a
// nonexistent avatar (or deleting twice) is a no-op, not an error.
func (s *AvatarService) DeleteAvatar(userID string) error {
	if branding.ContainsUnsafePath(userID) {
		return sharederrors.NewLocalizedError(
			"avatar_delete_failed",
			"Failed to delete avatar",
			"invalid user id for avatar delete",
			nil,
		)
	}

	if err := os.Remove(s.avatarPath(userID)); err != nil && !os.IsNotExist(err) {
		return sharederrors.NewLocalizedError(
			"avatar_delete_failed",
			"Failed to delete avatar",
			"failed to delete avatar",
			err,
		)
	}
	return nil
}

// AvatarPath returns the on-disk path of userID's avatar and whether it
// currently exists.
func (s *AvatarService) AvatarPath(userID string) (string, bool) {
	if branding.ContainsUnsafePath(userID) {
		return "", false
	}
	path := s.avatarPath(userID)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

func (s *AvatarService) avatarPath(userID string) string {
	return filepath.Join(s.avatarsDir, userID+".png")
}

// decodeAndResize decodes data and scales it to TargetSize while holding
// decodeSlot; the large source image is garbage once it returns, so only the
// small result outlives the slot.
func (s *AvatarService) decodeAndResize(data []byte) (image.Image, error) {
	s.decodeSlot <- struct{}{}
	defer func() { <-s.decodeSlot }()

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return resize(cropToSquare(img), TargetSize, TargetSize), nil
}

// checkDimensions rejects images whose declared size is empty or exceeds
// MaxImageSide or MaxImagePixels. The product is computed in int64 because
// int is 32 bits on armv7 builds and 65535x65535 would overflow it.
func checkDimensions(width, height int) error {
	if width > 0 && height > 0 && width <= MaxImageSide && height <= MaxImageSide &&
		int64(width)*int64(height) <= MaxImagePixels {
		return nil
	}
	return sharederrors.NewLocalizedError(
		"avatar_decode_failed",
		"Avatar image dimensions too large",
		"avatar image is %sx%s pixels (allowed: up to %s pixels in total)",
		nil,
		strconv.Itoa(width),
		strconv.Itoa(height),
		strconv.Itoa(MaxImagePixels),
	)
}

// cropToSquare returns the largest centered square region of img, so the
// crop is anchored at the center rather than a corner regardless of whether
// the source is wider or taller than it is tall/wide.
func cropToSquare(img image.Image) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	side := w
	if h < side {
		side = h
	}

	offsetX := bounds.Min.X + (w-side)/2
	offsetY := bounds.Min.Y + (h-side)/2
	cropRect := image.Rect(offsetX, offsetY, offsetX+side, offsetY+side)

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	if si, ok := img.(subImager); ok {
		return si.SubImage(cropRect)
	}

	// Fallback for image.Image implementations without SubImage: draw into a
	// fresh RGBA using the crop rectangle as the source region.
	dst := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(dst, dst.Bounds(), img, cropRect.Min, draw.Src)
	return dst
}

// resize scales src to exactly width x height using CatmullRom (bicubic-like,
// higher quality than nearest-neighbor/bilinear) resampling.
func resize(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
