package wiki

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Workspace struct {
	ID      string
	DataDir string
	RootDir string
}

var reservedDataDirEntries = []string{
	"users.db",
	"sessions.db",
	"search.db",
	"links.db",
	"tags.db",
	"properties.db",
	"schema.json",
	"tree.json",
	"assets",
	".leafwiki",
	".importer",
	"branding",
	"branding.json",
}

func DefaultWorkspace(dataDir string) Workspace {
	return NormalizeWorkspace(Workspace{ID: "default", DataDir: dataDir})
}

func NormalizeWorkspace(workspace Workspace) Workspace {
	id := strings.TrimSpace(workspace.ID)
	if id == "" {
		id = "default"
	}
	dataDir := cleanWorkspacePath(workspace.DataDir)
	rootDir := cleanWorkspacePath(workspace.RootDir)
	if rootDir == "" && dataDir != "" {
		rootDir = filepath.Join(dataDir, "root")
	}
	return Workspace{
		ID:      id,
		DataDir: dataDir,
		RootDir: rootDir,
	}
}

func cleanWorkspacePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func ValidateWorkspace(workspace Workspace) error {
	workspace = NormalizeWorkspace(workspace)
	if strings.TrimSpace(workspace.DataDir) == "" {
		return fmt.Errorf("data dir must not be empty")
	}
	if strings.TrimSpace(workspace.RootDir) == "" {
		return fmt.Errorf("root dir must not be empty")
	}
	cleanData, err := resolveWorkspacePath(workspace.DataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}
	cleanRoot, err := resolveWorkspacePath(workspace.RootDir)
	if err != nil {
		return fmt.Errorf("resolve root dir: %w", err)
	}
	if cleanData == cleanRoot {
		return fmt.Errorf("root dir must be different from data dir")
	}
	if pathContains(cleanRoot, cleanData) {
		return fmt.Errorf("root dir must not contain data dir")
	}
	for _, entry := range reservedDataDirEntries {
		statePath := filepath.Join(cleanData, entry)
		if cleanRoot == statePath || pathContains(statePath, cleanRoot) {
			return fmt.Errorf("root dir must not be inside data dir app state: %s", statePath)
		}
	}
	return nil
}

func resolveWorkspacePath(path string) (string, error) {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	current := absPath
	var suffix []string
	for {
		parent := filepath.Dir(current)
		if parent == current {
			return absPath, nil
		}
		suffix = append([]string{filepath.Base(current)}, suffix...)
		current = parent
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			parts := append([]string{resolved}, suffix...)
			return filepath.Clean(filepath.Join(parts...)), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
}

func pathContains(parent string, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
