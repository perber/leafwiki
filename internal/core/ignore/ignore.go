// Package ignore provides gitignore-style pattern matching for LeafWiki's
// .leafwikiignore file. A file at the root of the wiki's data directory
// controls which files and directories are excluded from the page tree,
// search index, tags, links, backlinks, and asset management.
package ignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	gitignore "github.com/sabhiram/go-gitignore"
)

const IgnoreFilename = ".leafwikiignore"

// IgnoreFile holds compiled gitignore patterns loaded from a .leafwikiignore file.
type IgnoreFile struct {
	matcher      *gitignore.GitIgnore
	patternCount int
}

// LoadFromDir reads .leafwikiignore from dir. Returns nil, nil if the file
// doesn't exist. Returns an error if the file exists but cannot be read or
// contains invalid syntax.
func LoadFromDir(dir string) (*IgnoreFile, error) {
	path := filepath.Join(dir, IgnoreFilename)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", IgnoreFilename, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory, expected a file", IgnoreFilename)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", IgnoreFilename, err)
	}

	lines := strings.Split(string(raw), "\n")
	count := countPatterns(lines)
	matcher := gitignore.CompileIgnoreLines(lines...)

	return &IgnoreFile{matcher: matcher, patternCount: count}, nil
}

// countPatterns counts non-comment, non-blank lines that are valid patterns.
// This mirrors the library's internal logic in getPatternFromLine.
func countPatterns(lines []string) int {
	count := 0
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		count++
	}
	return count
}

// Cache provides lazy, cached ignore file resolution for any directory
// under a given root. Thread-safe.
type Cache struct {
	root string
	mu   sync.RWMutex
	data map[string]*IgnoreFile
}

// NewCache creates a cache rooted at rootDir (typically storageDir/root).
func NewCache(rootDir string) *Cache {
	return &Cache{
		root: rootDir,
		data: make(map[string]*IgnoreFile),
	}
}

// Get returns the compiled ignore rules for dir, accumulating patterns
// from every .leafwikiignore between root and dir. Returns nil if no
// .leafwikiignore exists in any ancestor.
func (c *Cache) Get(dir string) *IgnoreFile {
	c.mu.RLock()
	if cached, ok := c.data[dir]; ok {
		c.mu.RUnlock()
		return cached
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if cached, ok := c.data[dir]; ok {
		return cached
	}

	// Collect ancestor directories root-first
	var ancestors []string
	for d := dir; ; d = filepath.Dir(d) {
		ancestors = append(ancestors, d)
		if d == c.root {
			break
		}
		// Safety: if we somehow go above root (e.g. dir not under root),
		// stop to avoid infinite loop.
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
	}

	// Reverse so we process root → target
	var allLines []string
	for i := len(ancestors) - 1; i >= 0; i-- {
		ignorePath := filepath.Join(ancestors[i], IgnoreFilename)
		if data, err := os.ReadFile(ignorePath); err == nil {
			allLines = append(allLines, strings.Split(string(data), "\n")...)
		}
	}

	if len(allLines) == 0 {
		c.data[dir] = nil
		return nil
	}

	compiled := CompileLines(allLines)
	c.data[dir] = compiled
	return compiled
}

// CompileLines compiles the given lines into an IgnoreFile.
// Used by Cache to combine patterns from multiple .leafwikiignore files.
func CompileLines(lines []string) *IgnoreFile {
	return &IgnoreFile{
		matcher:      gitignore.CompileIgnoreLines(lines...),
		patternCount: countPatterns(lines),
	}
}

// PatternCount returns the number of non-comment, non-blank patterns loaded.
func (f *IgnoreFile) PatternCount() int {
	if f == nil {
		return 0
	}
	return f.patternCount
}

// Matches checks whether the given relative path matches any pattern.
// path must be relative to the wiki root, using forward slashes.
// isDir must be true for directories.
func (f *IgnoreFile) Matches(path string, isDir bool) bool {
	if f == nil || f.matcher == nil {
		return false
	}
	// Append trailing slash for directory matching per gitignore semantics.
	if isDir && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return f.matcher.MatchesPath(path)
}
