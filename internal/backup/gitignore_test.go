package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureGitignore_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := EnsureGitignore(tmpDir)
	if err != nil {
		t.Fatalf("EnsureGitignore failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".gitignore")
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	if string(content) != gitignoreContent {
		t.Errorf("expected content %q, got %q", gitignoreContent, string(content))
	}
}

func TestEnsureGitignore_DoesNotOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	existingContent := "existing content\n"
	err := os.WriteFile(gitignorePath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("failed to write existing .gitignore: %v", err)
	}

	err = EnsureGitignore(tmpDir)
	if err != nil {
		t.Fatalf("EnsureGitignore failed: %v", err)
	}

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	if string(content) != existingContent {
		t.Errorf("expected content %q, got %q", existingContent, string(content))
	}
}