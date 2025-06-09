package search

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/perber/wiki/internal/core/tree"
)

func StartWatcher(dataDir string, treeService *tree.TreeService, index *SQLiteIndex, status *IndexingStatus) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Initial: Alle Verzeichnisse überwachen
	err = filepath.Walk(dataDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[watcher] walk error: %v", err)
			return nil
		}
		if info.IsDir() {
			if err := watcher.Add(p); err != nil {
				log.Printf("[watcher] add error: %v", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Normalize path
				eventPath := filepath.ToSlash(event.Name)

				info, statErr := os.Stat(eventPath)
				isDir := statErr == nil && info.IsDir()

				// Neues Verzeichnis oder verschobenes?
				if (event.Op&(fsnotify.Create|fsnotify.Rename) != 0) && isDir {
					// Rekursiv beobachten
					log.Printf("[watcher] watching new dir: %s", eventPath)
					if err := filepath.Walk(eventPath, func(p string, i os.FileInfo, _ error) error {
						if i.IsDir() {
							if err := watcher.Add(p); err != nil {
								log.Printf("[watcher] add error: %v", err)
								return nil // continue walking
							}
						} else if filepath.Ext(p) == ".md" {
							reindexFile(p, dataDir, treeService, index, status)
						}
						return nil
					}); err != nil {
						log.Printf("[watcher] walk error: %v", err)
					}
					continue
				}

				// Dateiänderungen
				if filepath.Ext(eventPath) != ".md" {
					continue
				}

				switch {
				case event.Op&(fsnotify.Create|fsnotify.Write) != 0:
					reindexFile(eventPath, dataDir, treeService, index, status)

				case event.Op&fsnotify.Remove != 0:
					relPath, err := filepath.Rel(dataDir, eventPath)
					if err == nil {
						log.Printf("[watcher] file removed: %s", relPath)
						cnt, err := index.RemovePageByFilePath(relPath)
						if err != nil {
							log.Printf("[watcher] remove error: %v", err)
						} else {
							log.Printf("[watcher] removed %d pages for: %s", cnt, relPath)
						}
					}

				case event.Op&fsnotify.Rename != 0 && !isDir:
					relPath, err := filepath.Rel(dataDir, eventPath)
					if err == nil {
						log.Printf("[watcher] file renamed/removed: %s", relPath)
						cnt, err := index.RemovePageByFilePath(relPath)
						if err != nil {
							log.Printf("[watcher] remove error: %v", err)
						} else {
							log.Printf("[watcher] removed %d pages for: %s", cnt, relPath)
						}
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[watcher] error: %v", err)
			}
		}
	}()

	log.Println("[watcher] started watching:", dataDir)
	return nil
}

func reindexFile(fullPath, dataDir string, treeService *tree.TreeService, index *SQLiteIndex, status *IndexingStatus) {
	rel, err := filepath.Rel(dataDir, fullPath)
	if err != nil {
		log.Printf("[watcher] rel path error: %v", err)
		return
	}

	routePath := strings.TrimSuffix(rel, filepath.Ext(rel))
	routePath = filepath.ToSlash(strings.TrimSuffix(routePath, "/index"))

	page, err := treeService.FindPageByRoutePath(treeService.GetTree().Children, routePath)
	if err != nil {
		log.Printf("[watcher] not in tree: %s", rel)
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		log.Printf("[watcher] read error: %v", err)
		return
	}

	err = index.IndexPage(page.CalculatePath(), rel, page.ID, page.Title, string(content))
	if err != nil {
		status.Fail()
		log.Printf("[watcher] index error: %v", err)
	} else {
		status.Success()
		log.Printf("[watcher] indexed: %s", rel)
	}
}
