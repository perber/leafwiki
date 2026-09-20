package backup

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// normalizeBackupPath validates and cleans a user-supplied git backup prefix.
// It returns "" for an empty path (content at the repository top level) and a
// slash-separated relative path otherwise, e.g. "docs/wiki".
//
// Rejected: absolute paths and any ".." segment. Both "/" and "\" are accepted
// as separators so a Windows-style "docs\wiki" behaves the same as "docs/wiki"
// (git tree paths are always slash-separated).
func normalizeBackupPath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", nil
	}
	p = strings.ReplaceAll(filepath.ToSlash(p), "\\", "/")
	if strings.HasPrefix(p, "/") || filepath.IsAbs(p) {
		return "", fmt.Errorf("git backup path must be relative, got %q", raw)
	}
	if slices.Contains(strings.Split(p, "/"), "..") {
		return "", fmt.Errorf("git backup path must not contain %q segments, got %q", "..", raw)
	}
	cleaned := path.Clean(p)
	if cleaned == "." || cleaned == "/" || cleaned == "" {
		return "", fmt.Errorf("git backup path must not be empty or %q, got %q", ".", raw)
	}
	return cleaned, nil
}
