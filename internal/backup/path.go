package backup

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const gitBackupDirName = ".git-backup"

// validateBackupPath checks a relative monorepo subdirectory for git backup.
// Empty path is valid (legacy in-place $DATA/.git behavior).
func validateBackupPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	slashPath := filepath.ToSlash(path)
	if strings.Contains(slashPath, "..") {
		return fmt.Errorf("git backup path must not contain \"..\" (got %q)", path)
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if cleaned == "." || cleaned == "" {
		return fmt.Errorf("git backup path must not be empty or \".\"")
	}
	if filepath.IsAbs(path) || strings.HasPrefix(cleaned, "/") {
		return fmt.Errorf("git backup path must be relative (got %q)", path)
	}
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("git backup path must not contain empty or \"..\" segments (got %q)", path)
		}
	}
	return nil
}

func normalizeBackupPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func resolveRepoDir(cfg Config) string {
	dataDir := filepath.Dir(filepath.Clean(cfg.RootDir))
	if normalizeBackupPath(cfg.Path) == "" {
		return dataDir
	}
	return filepath.Join(dataDir, gitBackupDirName)
}

// prepareContentDirs returns the root/ and assets/ directories to stage into git.
// When Path is set, live wiki content is mirrored under
// $DATA/.git-backup/<path>/{root,assets} first so monorepo remotes stay intact.
func (r *Repository) prepareContentDirs() (rootDir, assetsDir string, err error) {
	path := normalizeBackupPath(r.cfg.Path)
	if path == "" {
		return r.cfg.RootDir, r.cfg.AssetsDir, nil
	}
	rootDir = filepath.Join(r.repoDir, filepath.FromSlash(path), "root")
	assetsDir = filepath.Join(r.repoDir, filepath.FromSlash(path), "assets")
	if err := mirrorDir(r.cfg.RootDir, rootDir); err != nil {
		return "", "", fmt.Errorf("mirror root into git backup path: %w", err)
	}
	if err := mirrorDir(r.cfg.AssetsDir, assetsDir); err != nil {
		return "", "", fmt.Errorf("mirror assets into git backup path: %w", err)
	}
	return rootDir, assetsDir, nil
}

// mirrorDir replaces dest with a full copy of src. Missing src creates an empty dest.
func mirrorDir(src, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported in git backup mirror: %s", path)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
