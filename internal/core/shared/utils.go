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

var ErrFileTooLarge = errors.New("file too large")

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
				slog.Default().Error("failed to close temp file", "operation", "chmod", "error", closeErr)
			}
			return chmodErr
		}
	}

	if _, err := tmpFile.Write(data); err != nil {
		writeErr := fmt.Errorf("write temp file: %w", err)
		if closeErr := tmpFile.Close(); closeErr != nil {
			slog.Default().Error("failed to close temp file", "operation", "write", "error", closeErr)
		}
		return writeErr
	}

	if err := tmpFile.Sync(); err != nil {
		syncErr := fmt.Errorf("sync temp file: %w", err)
		if closeErr := tmpFile.Close(); closeErr != nil {
			slog.Default().Error("failed to close temp file", "operation", "sync", "error", closeErr)
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

func WriteStreamAtomic(targetPath string, src io.Reader, maxBytes int64) error {
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
