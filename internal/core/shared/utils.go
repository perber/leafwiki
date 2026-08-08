package shared

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/teris-io/shortid"
)

// GenerateUniqueID generates a unique ID for a tree entry
func GenerateUniqueID() (string, error) {
	id, err := shortid.Generate()
	if err != nil {
		return "", err
	}

	return id, nil
}

var charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

const errFailedToCloseTempFile = "failed to close temp file"

var (
	ErrFileTooLarge              = errors.New("file too large")
	ErrCumulativeSizeTooLarge    = errors.New("cumulative size too large")
	ErrDecompressionRatioTooHigh = errors.New("decompression ratio too high")
)

// ExtractionLimits bundles the size and decompression-ratio caps enforced
// while extracting entries from a zip archive (see CopyWithBudget).
type ExtractionLimits struct {
	MaxEntryBytes   int64 // hard per-file cap -> ErrFileTooLarge
	MaxTotalBytes   int64 // hard cumulative cap across the whole archive -> ErrCumulativeSizeTooLarge
	MaxRatio        int64 // decompressed:compressed ceiling -> ErrDecompressionRatioTooHigh
	RatioFloorBytes int64 // ratio check only applies once decompressed output exceeds this
}

// DefaultZipExtractionLimits is what production applies to zip extraction
// from a source that crosses a trust boundary (an uploaded import/restore
// zip): 50 MiB/file, 1 GiB/archive, 100:1 ratio past a 1 MiB floor.
var DefaultZipExtractionLimits = ExtractionLimits{
	MaxEntryBytes:   50 * 1024 * 1024,
	MaxTotalBytes:   1024 * 1024 * 1024,
	MaxRatio:        100,
	RatioFloorBytes: 1024 * 1024,
}

// unrestrictedExtractionBytes is deliberately far below the int64 range's
// true ceiling (math.MaxInt64) so that limit+1 in CopyWithBudget, and a
// budget's remaining-bytes bookkeeping across an entire archive, can never
// overflow — while still being larger than any real wiki's data could ever
// plausibly reach (a few exabytes of headroom).
const unrestrictedExtractionBytes = int64(1) << 62

// UnrestrictedExtractionLimits disables all three caps. Use it for a zip
// source that doesn't cross a new trust boundary — a snapshot the server
// itself already created via its own snapshot machinery, or a file path an
// operator with direct filesystem access chose (the offline CLI restore
// path): DefaultZipExtractionLimits would only risk rejecting a legitimately
// large restore there, not stop an attack, since nothing about that data is
// attacker-controlled. RatioFloorBytes is set equal to the other fields so
// ratioLimitedWriter's `written > minFloor` check short-circuits before ever
// evaluating the ratio math, regardless of MaxRatio's value.
var UnrestrictedExtractionLimits = ExtractionLimits{
	MaxEntryBytes:   unrestrictedExtractionBytes,
	MaxTotalBytes:   unrestrictedExtractionBytes,
	MaxRatio:        unrestrictedExtractionBytes,
	RatioFloorBytes: unrestrictedExtractionBytes,
}

// SizeBudget tracks a cumulative byte budget shared across a sequence of
// CopyWithBudget calls (e.g. every file extracted from one archive). Not
// safe for concurrent use — each extraction is processed by a single
// goroutine, so this is sufficient.
type SizeBudget struct {
	remaining int64
}

func NewSizeBudget(max int64) *SizeBudget {
	return &SizeBudget{remaining: max}
}

// ratioLimitedWriter wraps a Writer and aborts once bytes written exceed
// compressedSize * maxRatio, but only past minFloor bytes — small files
// naturally have noisy/high ratios without being a bomb signature.
type ratioLimitedWriter struct {
	dst            io.Writer
	compressedSize int64
	maxRatio       int64
	minFloor       int64
	written        int64
}

func (w *ratioLimitedWriter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	w.written += int64(n)
	if err != nil {
		return n, err
	}
	// written/maxRatio > compressedSize is equivalent to
	// written > compressedSize*maxRatio (up to rounding within maxRatio,
	// negligible next to the megabyte-scale floor/caps involved) but avoids
	// overflowing int64 when compressedSize is large — compressedSize comes
	// from attacker-controlled zip metadata, so it cannot be trusted not to
	// be huge.
	if w.compressedSize > 0 && w.maxRatio > 0 && w.written > w.minFloor && w.written/w.maxRatio > w.compressedSize {
		return n, fmt.Errorf("%w: %d bytes decompressed from %d compressed bytes (exceeds %d:1)",
			ErrDecompressionRatioTooHigh, w.written, w.compressedSize, w.maxRatio)
	}
	return n, nil
}

// CopyWithBudget copies from src to dst, enforcing three independent caps in
// one pass: limits.MaxEntryBytes for this single copy, budget's shared
// remaining cumulative total across the whole archive, and a
// decompression-ratio ceiling relative to declaredCompressedSize (once past
// limits.RatioFloorBytes) that catches an actual zip-bomb-shaped entry almost
// immediately — long before either byte cap could trip on its own. Pass
// declaredCompressedSize == 0 to skip the ratio check (e.g. entries where
// it isn't known). io.Copy stops as soon as ratioLimitedWriter.Write returns
// an error, so a ratio violation aborts mid-stream rather than after the
// fact.
func CopyWithBudget(dst io.Writer, src io.Reader, declaredCompressedSize uint64, limits ExtractionLimits, budget *SizeBudget) error {
	limit := limits.MaxEntryBytes
	limitedByBudget := false
	if budget.remaining < limit {
		limit = budget.remaining
		limitedByBudget = true
	}
	if limit < 0 {
		limit = 0
	}

	// declaredCompressedSize is read straight from the zip's central
	// directory — attacker-controlled and independent of the entry's real
	// compressed byte count. It can exceed math.MaxInt64, which would make
	// the naive int64 conversion negative and silently disable the ratio
	// check below (compressedSize > 0 becomes false). Clamp an implausible
	// value down to something deliberately small instead, so an attacker
	// inflating this field to dodge detection makes the ratio check *more*
	// aggressive, not disabled.
	compressedSize := int64(declaredCompressedSize)
	if compressedSize < 0 {
		compressedSize = 1
	}

	rw := &ratioLimitedWriter{
		dst:            dst,
		compressedSize: compressedSize,
		maxRatio:       limits.MaxRatio,
		minFloor:       limits.RatioFloorBytes,
	}

	n, err := io.Copy(rw, io.LimitReader(src, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		if limitedByBudget {
			return fmt.Errorf("%w: %d bytes would exceed remaining budget of %d bytes", ErrCumulativeSizeTooLarge, n, budget.remaining)
		}
		return fmt.Errorf("%w: %d bytes (max %d)", ErrFileTooLarge, n, limits.MaxEntryBytes)
	}
	budget.remaining -= n
	return nil
}

func GenerateRandomPassword(length int) (string, error) {
	password := make([]byte, length)
	for i := range password {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password[i] = charset[n.Int64()]
	}
	return string(password), nil
}

func atomicReplace(src, dst string) error {
	// On Windows, os.Rename fails if dst already exists.
	// On Unix, Rename is atomic and replaces dst.
	if runtime.GOOS == "windows" {
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove existing file: %w", err)
		}
	}
	return os.Rename(src, dst)
}

func atomicWriteDir(filename string) string {
	normalized := strings.ReplaceAll(filename, `\`, `/`)
	return filepath.Dir(filepath.FromSlash(normalized))
}

// WriteFileAtomic writes data to filename atomically by writing to a temp file
// in the same directory and then renaming it over the target.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := atomicWriteDir(filename)

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmpFile.Name()
	// Ensure the temp file is removed in case of an error
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if perm != 0 {
		if err := tmpFile.Chmod(perm); err != nil {
			chmodErr := fmt.Errorf("chmod temp file: %w", err)
			if closeErr := tmpFile.Close(); closeErr != nil {
				slog.Default().Error(errFailedToCloseTempFile, "operation", "chmod", "error", closeErr)
			}
			return chmodErr
		}
	}

	if _, err := tmpFile.Write(data); err != nil {
		writeErr := fmt.Errorf("write temp file: %w", err)
		if closeErr := tmpFile.Close(); closeErr != nil {
			slog.Default().Error(errFailedToCloseTempFile, "operation", "write", "error", closeErr)
		}
		return writeErr
	}

	if err := tmpFile.Sync(); err != nil {
		syncErr := fmt.Errorf("sync temp file: %w", err)
		if closeErr := tmpFile.Close(); closeErr != nil {
			slog.Default().Error(errFailedToCloseTempFile, "operation", "sync", "error", closeErr)
		}
		return syncErr
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := atomicReplace(tmpName, filename); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// CopyWithLimit copies from src to dst but returns an error if more than max bytes are copied.
func CopyWithLimit(dst io.Writer, src io.Reader, max int64) error {
	n, err := io.Copy(dst, io.LimitReader(src, max+1))
	if err != nil {
		return err
	}
	if n > max {
		return fmt.Errorf("%w: %d bytes (max %d)", ErrFileTooLarge, n, max)
	}
	return nil
}

func WriteStreamAtomic(targetPath string, src io.Reader, maxBytes int64, perm os.FileMode) error {
	dir := atomicWriteDir(targetPath)

	out, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := out.Name()

	// Ensure cleanup on failure
	ok := false
	defer func() {
		if out == nil {
			if !ok {
				_ = os.Remove(tmp)
			}
			return
		}

		if err := out.Close(); err != nil {
			slog.Default().Error("Failed to close temp file", "file", tmp, "error", err)
			return
		}
		if !ok {
			_ = os.Remove(tmp)
		}
	}()

	if err := CopyWithLimit(out, src, maxBytes); err != nil {
		return err
	}

	// Best-effort durability
	if err := out.Sync(); err != nil {
		return err
	}

	// Chmod after writing so partial content is never world-readable.
	if perm != 0 {
		if err := out.Chmod(perm); err != nil {
			return fmt.Errorf("chmod temp file: %w", err)
		}
	}

	if err := out.Close(); err != nil {
		return err
	}
	out = nil

	if err := atomicReplace(tmp, targetPath); err != nil {
		return err
	}

	ok = true
	return nil
}

// LogClose calls closer and logs any error via slog. Intended for use in
// deferred cleanup where the error cannot be propagated, e.g.:
//
//	defer shared.LogClose(rows.Close, "could not close rows")
func LogClose(closer func() error, msg string) {
	if err := closer(); err != nil {
		slog.Default().Error(msg, "error", err)
	}
}
