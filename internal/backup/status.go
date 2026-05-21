package backup

import (
	"sync"
	"time"
)

type Status struct {
	mu           sync.RWMutex
	LastBackupAt time.Time
	LastError    string
}

func (s *Status) SetSuccess(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastBackupAt = t
	s.LastError = ""
}

func (s *Status) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
}

func (s *Status) Snapshot() StatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return StatusSnapshot{
		LastBackupAt: s.LastBackupAt,
		LastError:    s.LastError,
	}
}

type StatusSnapshot struct {
	LastBackupAt time.Time `json:"lastBackupAt"`
	LastError    string    `json:"lastError"`
}