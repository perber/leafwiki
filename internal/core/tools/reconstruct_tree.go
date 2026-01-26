package tools

import (
	"github.com/perber/wiki/internal/core/tree"
)

func ReconstructTreeFromFS(storageDir string) error {
	treeService := tree.NewTreeService(storageDir)
	return treeService.ReconstructTreeFromFS()
}
