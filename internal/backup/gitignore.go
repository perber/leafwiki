package backup

import (
	"os"
	"path/filepath"
)

const gitignoreContent = `# LeafWiki runtime files – do not commit
*.db
*.db-journal
*.db-shm
*.db-wal
*.tmp
.tmp-*
.leafwiki/
schema.json
`

// EnsureGitignore writes a .gitignore to repoDir if it does not already exist.
func EnsureGitignore(repoDir string) error {
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	// os.WriteFile already respects the process umask — no manual umask needed
	return os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
}