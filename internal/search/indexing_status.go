package search

import (
	"sync"
	"time"
)

type IndexingStatus struct {
	mu         sync.RWMutex
	Active     bool      // Indicates if indexing is currently active
	Indexed    int       // Number of pages indexed
	Failed     int       // Number of pages that failed to index
	FinishedAt time.Time // Timestamp when indexing finished
}

func NewIndexingStatus() *IndexingStatus {
	return &IndexingStatus{
		Active:  false,
		Indexed: 0,
		Failed:  0,
	}
}

func (s *IndexingStatus) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = true
	s.Indexed = 0
	s.Failed = 0
	s.FinishedAt = time.Time{} // Reset finished time
}

func (s *IndexingStatus) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = false
	s.FinishedAt = time.Now()
}

func (s *IndexingStatus) Success() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Indexed++
}

func (s *IndexingStatus) Fail() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Failed++
}

func (s *IndexingStatus) Snapshot() *IndexingStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &IndexingStatus{
		Active:     s.Active,
		Indexed:    s.Indexed,
		Failed:     s.Failed,
		FinishedAt: s.FinishedAt,
	}
}
