package backup

import (
	"os"
	"path/filepath"
	"syscall"
)

const gitignoreContent = `# LeafWiki runtime files – do not commit
*.db
*.db-journal
*.db-shm
*.db-wal
*.tmp
.tmp-*
`

// EnsureGitignore writes a .gitignore to repoDir if it does not already exist.
func EnsureGitignore(repoDir string) error {
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		// File exists, do not overwrite
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	// Respect system umask
	oldmask := syscall.Umask(0)
	defer syscall.Umask(oldmask)
	return os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644&^os.FileMode(oldmask))
}