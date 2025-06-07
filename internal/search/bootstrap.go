package search

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
)

// BuildAndRunIndexer initializes the indexer with the given tree service and SQLite index,
func BuildAndRunIndexer(treeService *tree.TreeService, sqliteIndex *SQLiteIndex, dataDir string, workers int) error {
	indexer := NewIndexer(dataDir, workers, func(file string, content []byte) error {
		rel, err := filepath.Rel(dataDir, file)
		if err != nil {
			return err
		}
		routePath := strings.TrimSuffix(rel, filepath.Ext(rel))
		routePath = filepath.ToSlash(routePath)

		page, err := treeService.FindPageByRoutePath(treeService.GetTree().Children, routePath)
		if err != nil {
			// the page is on the filesystem but not in the tree, skip it
			log.Printf("[indexer] skipping file not in tree: %s", rel)
			return nil
		}

		return sqliteIndex.IndexPage(rel, page.ID, page.Title, string(content))
	})

	return indexer.Start()
}
