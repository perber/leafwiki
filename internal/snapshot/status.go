package snapshot

import (
	"sync"
	"time"
)

type Status struct {
	mu             sync.RWMutex
	IsRunning      bool
	LastSnapshotAt time.Time
	LastError      string
}

// TryStart atomically marks the status as running, unless it already is.
// Returns false if a snapshot is already in progress.
func (s *Status) TryStart() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.IsRunning {
		return false
	}
	s.IsRunning = true
	return true
}

func (s *Status) SetSuccess(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsRunning = false
	s.LastSnapshotAt = t
	s.LastError = ""
}

func (s *Status) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsRunning = false
	s.LastError = err
}

func (s *Status) Snapshot() StatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var lastSnapshotAt *time.Time
	if !s.LastSnapshotAt.IsZero() {
		t := s.LastSnapshotAt
		lastSnapshotAt = &t
	}
	return StatusSnapshot{
		IsRunning:      s.IsRunning,
		LastSnapshotAt: lastSnapshotAt,
		LastError:      s.LastError,
	}
}

type StatusSnapshot struct {
	IsRunning      bool       `json:"isRunning"`
	LastSnapshotAt *time.Time `json:"lastSnapshotAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
}
