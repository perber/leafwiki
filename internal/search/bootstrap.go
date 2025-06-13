package search

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
)

// BuildAndRunIndexer initializes the indexer with the given tree service and SQLite index,
func BuildAndRunIndexer(treeService *tree.TreeService, sqliteIndex *SQLiteIndex, dataDir string, workers int, status *IndexingStatus) error {
	status.Start()
	indexer := NewIndexer(dataDir, workers, func(file string, content []byte) error {
		rel, err := filepath.Rel(dataDir, file)
		if err != nil {
			status.Fail()
			return err
		}
		routePath := strings.TrimSuffix(rel, filepath.Ext(rel))
		routePath = filepath.ToSlash(routePath)

		page, err := treeService.FindPageByRoutePath(treeService.GetTree().Children, routePath)
		if err != nil {
			// the page is on the filesystem but not in the tree, skip it
			log.Printf("[indexer] skipping file not in tree: %s", rel)
			status.Fail()
			return nil
		}

		if err := sqliteIndex.IndexPage(rel, page.ID, page.Title, string(content)); err != nil {
			log.Printf("[indexer] error indexing page %s: %v", rel, err)
			status.Fail()
			return err
		}

		status.Success()
		return nil
	})

	err := indexer.Start()
	status.Finish()
	return err
}
