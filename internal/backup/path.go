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

// resolveLivePath joins rel, a slash-separated path from a git tree, onto
// liveDir and refuses a result that is not strictly inside liveDir. Tree entry
// names are validated when the tree is read; this checks every write and
// delete again, so the pull keeps its writes inside the content directory even
// if a path slips past that validation.
func resolveLivePath(liveDir, rel string) (string, error) {
	p := filepath.Join(liveDir, filepath.FromSlash(rel))
	inside, err := filepath.Rel(liveDir, p)
	if err != nil || inside == "." || inside == ".." || filepath.IsAbs(inside) ||
		strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q resolves outside %s", rel, liveDir)
	}
	return p, nil
}
