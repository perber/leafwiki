package search

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/perber/wiki/internal/core/tree"
)

type Watcher struct {
	DataDir     string
	TreeService *tree.TreeService
	Index       *SQLiteIndex
	Status      *IndexingStatus
	watcher     *fsnotify.Watcher
}

func NewWatcher(dataDir string, treeService *tree.TreeService, index *SQLiteIndex, status *IndexingStatus) (*Watcher, error) {
	watcher := &Watcher{
		DataDir:     dataDir,
		TreeService: treeService,
		Index:       index,
		Status:      status,
		watcher:     nil,
	}

	return watcher, nil
}

func (w *Watcher) Start() error {
	var err error
	if w.watcher, err = fsnotify.NewWatcher(); err != nil {
		return err
	}

	err = filepath.Walk(w.DataDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[watcher] walk error: %v", err)
			return nil
		}
		if info.IsDir() {
			if err := w.watcher.Add(p); err != nil {
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
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}

				// Normalize path
				eventPath := filepath.ToSlash(event.Name)

				info, statErr := os.Stat(eventPath)
				isDir := statErr == nil && info.IsDir()

				// New Directory or Moved
				if (event.Op&(fsnotify.Create|fsnotify.Rename) != 0) && isDir {
					// Watch recursive
					log.Printf("[watcher] watching new dir: %s", eventPath)
					if err := filepath.Walk(eventPath, func(p string, i os.FileInfo, _ error) error {
						if i.IsDir() {
							if err := w.watcher.Add(p); err != nil {
								log.Printf("[watcher] add error: %v", err)
								return nil // continue walking
							}
						} else if filepath.Ext(p) == ".md" {
							reindexFile(p, w.DataDir, w.TreeService, w.Index, w.Status)
						}
						return nil
					}); err != nil {
						log.Printf("[watcher] walk error: %v", err)
					}
					continue
				}

				if filepath.Ext(eventPath) != ".md" {
					continue
				}

				switch {
				case event.Op&(fsnotify.Create|fsnotify.Write) != 0:
					reindexFile(eventPath, w.DataDir, w.TreeService, w.Index, w.Status)

				case event.Op&fsnotify.Remove != 0:
					relPath, err := filepath.Rel(w.DataDir, eventPath)
					if err == nil {
						log.Printf("[watcher] file removed: %s", relPath)
						cnt, err := w.Index.RemovePageByFilePath(relPath)
						if err != nil {
							log.Printf("[watcher] remove error: %v", err)
						} else {
							log.Printf("[watcher] removed %d pages for: %s", cnt, relPath)
						}
					}

				case event.Op&fsnotify.Rename != 0 && !isDir:
					relPath, err := filepath.Rel(w.DataDir, eventPath)
					if err == nil {
						log.Printf("[watcher] file renamed/removed: %s", relPath)
						cnt, err := w.Index.RemovePageByFilePath(relPath)
						if err != nil {
							log.Printf("[watcher] remove error: %v", err)
						} else {
							log.Printf("[watcher] removed %d pages for: %s", cnt, relPath)
						}
					}
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[watcher] error: %v", err)
			}
		}
	}()

	log.Println("[watcher] started watching:", w.DataDir)
	return nil
}

func (w *Watcher) Stop() error {
	if w.watcher != nil {
		return w.watcher.Close()
	}
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
