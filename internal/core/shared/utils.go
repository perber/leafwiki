package shared

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"os"
	"path"

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

// WriteFileAtomic writes data to filename atomically by writing to a temp file
// in the same directory and then renaming it over the target.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := path.Dir(filename)

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
			tmpFile.Close()
			return fmt.Errorf("chmod temp file: %w", err)
		}
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, filename); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// CopyWithLimit copies from src to dst but returns an error if more than max bytes are copied
func CopyWithLimit(dst *os.File, src io.Reader, max int64) error {
	n, err := io.Copy(dst, io.LimitReader(src, max+1))
	if err != nil {
		return err
	}
	if n > max {
		return fmt.Errorf("file too large: %d bytes (max %d)", n, max)
	}
	return nil
}

func WriteStreamAtomic(targetPath string, src multipart.File, maxBytes int64) error {
	tmp := targetPath + ".tmp"

	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	// Ensure cleanup on failure
	ok := false
	defer func() {
		out.Close()
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

	if err := os.Rename(tmp, targetPath); err != nil {
		return err
	}

	ok = true
	return nil
}
