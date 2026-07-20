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

type IgnoreFile struct {
	matcher      *gitignore.GitIgnore
	patternCount int
}

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

type Cache struct {
	root string
	mu   sync.RWMutex
	data map[string]*IgnoreFile
}

func NewCache(rootDir string) *Cache {
	return &Cache{
		root: rootDir,
		data: make(map[string]*IgnoreFile),
	}
}

func (c *Cache) Get(dir string) *IgnoreFile {
	c.mu.RLock()
	if cached, ok := c.data[dir]; ok {
		c.mu.RUnlock()
		return cached
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok := c.data[dir]; ok {
		return cached
	}

	var ancestors []string
	for d := dir; ; d = filepath.Dir(d) {
		ancestors = append(ancestors, d)
		if d == c.root {
			break
		}

		parent := filepath.Dir(d)
		if parent == d {
			break
		}
	}

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

func CompileLines(lines []string) *IgnoreFile {
	return &IgnoreFile{
		matcher:      gitignore.CompileIgnoreLines(lines...),
		patternCount: countPatterns(lines),
	}
}

func (f *IgnoreFile) PatternCount() int {
	if f == nil {
		return 0
	}
	return f.patternCount
}

func (f *IgnoreFile) Matches(path string, isDir bool) bool {
	if f == nil || f.matcher == nil {
		return false
	}
	if isDir && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return f.matcher.MatchesPath(path)
}
