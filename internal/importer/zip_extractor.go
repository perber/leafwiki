package importer

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type ZipExtractor struct {
	log *slog.Logger
}

func NewZipExtractor() *ZipExtractor {
	return &ZipExtractor{
		log: slog.Default().With("component", "ZipExtractor"),
	}
}

func (x *ZipExtractor) ExtractToTemp(zipPath string) (*ZipWorkspace, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	root, err := os.MkdirTemp("", "import-*")
	if err != nil {
		return nil, fmt.Errorf("mkdtemp: %w", err)
	}

	ws := &ZipWorkspace{Root: root}
	// Helper to clean up and return error
	fail := func(e error) (*ZipWorkspace, error) {
		if err = ws.Cleanup(); err != nil {
			x.log.Error("cleanup failed", "error", err)
		}
		return nil, e
	}

	for _, f := range r.File {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}

		destPath, err := safeJoin(ws.Root, name)
		if err != nil {
			return fail(fmt.Errorf("invalid zip entry %q: %w", f.Name, err))
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fail(fmt.Errorf("mkdir: %w", err))
		}

		// Extract single file in inner scope to ensure deterministic cleanup per iteration
		if err := func() error {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open zip entry: %w", err)
			}
			defer rc.Close()

			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			defer func() {
				if err := out.Close(); err != nil {
					x.log.Error("close failed", "error", err)
				}
			}()

			if _, err := io.Copy(out, rc); err != nil {
				return fmt.Errorf("write file: %w", err)
			}

			return nil
		}(); err != nil {
			return fail(err)
		}
	}

	return ws, nil
}

func safeJoin(baseDir, zipEntryName string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(zipEntryName))
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	dest := filepath.Join(baseDir, clean)

	baseClean := filepath.Clean(baseDir) + string(filepath.Separator)
	destClean := filepath.Clean(dest)

	if !strings.HasPrefix(destClean+string(filepath.Separator), baseClean) {
		return "", fmt.Errorf("path traversal detected: %q", zipEntryName)
	}
	return destClean, nil
}
