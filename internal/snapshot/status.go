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
	LastPruneError string
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

// SetSuccess marks the run as finished successfully. pruneErr is the error
// message from retention pruning (empty if pruning succeeded or was
// skipped); the caller is expected to run pruning before calling this, while
// IsRunning is still true, so a concurrent run cannot start mid-prune.
func (s *Status) SetSuccess(t time.Time, pruneErr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsRunning = false
	s.LastSnapshotAt = t
	s.LastError = ""
	s.LastPruneError = pruneErr
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
		LastPruneError: s.LastPruneError,
	}
}

type StatusSnapshot struct {
	IsRunning      bool       `json:"isRunning"`
	LastSnapshotAt *time.Time `json:"lastSnapshotAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	LastPruneError string     `json:"lastPruneError,omitempty"`
}
