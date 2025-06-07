package search

import (
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Indexer struct {
	DataDir   string
	Workers   int
	IndexFunc func(file string, content []byte) error
}

func NewIndexer(dataDir string, workers int, fn func(string, []byte) error) *Indexer {
	return &Indexer{
		DataDir:   dataDir,
		Workers:   workers,
		IndexFunc: fn,
	}
}

func (i *Indexer) Start() error {
	files := make(chan string, 100)
	var wg sync.WaitGroup

	// Start worker goroutines
	for w := 0; w < i.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range files {
				content, err := os.ReadFile(file)
				if err != nil {
					log.Printf("[indexer] error reading file %s: %v", file, err)
					continue
				}

				// Call the indexing function
				if err := i.IndexFunc(file, content); err != nil {
					log.Printf("[indexer] error indexing file %s: %v", file, err)
				}
			}
		}()
	}

	// Walk through the data directory and send files to the channel
	err := filepath.Walk(i.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[indexer] error walking path %s: %v", path, err)
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".md" {
			files <- path
		}

		return nil
	})

	close(files)
	wg.Wait()

	return err
}
