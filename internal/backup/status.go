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
	var lastBackupAt *time.Time
	if !s.LastBackupAt.IsZero() {
		t := s.LastBackupAt
		lastBackupAt = &t
	}
	return StatusSnapshot{
		LastBackupAt: lastBackupAt,
		LastError:    s.LastError,
	}
}

type StatusSnapshot struct {
	LastBackupAt *time.Time `json:"lastBackupAt,omitempty"`
	LastError    string     `json:"lastError,omitempty"`
}
