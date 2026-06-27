package backup

import (
	"sync"
	"time"
)

type Status struct {
	mu                sync.RWMutex
	LastBackupAt      time.Time
	LastError         string
	NeedsIntervention bool
	ConflictDetails   string
}

func (s *Status) SetSuccess(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastBackupAt = t
	s.LastError = ""
	s.NeedsIntervention = false
	s.ConflictDetails = ""
}

func (s *Status) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
	s.NeedsIntervention = false
	s.ConflictDetails = ""
}

func (s *Status) SetNeedsIntervention(details string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NeedsIntervention = true
	s.ConflictDetails = details
	s.LastError = details
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
		LastBackupAt:      lastBackupAt,
		LastError:         s.LastError,
		NeedsIntervention: s.NeedsIntervention,
		ConflictDetails:   s.ConflictDetails,
	}
}

type StatusSnapshot struct {
	LastBackupAt      *time.Time `json:"lastBackupAt,omitempty"`
	LastError         string     `json:"lastError,omitempty"`
	NeedsIntervention bool       `json:"needsIntervention,omitempty"`
	ConflictDetails   string     `json:"conflictDetails,omitempty"`
}
