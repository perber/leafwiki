package backup

import (
	"sync"
	"time"
)

// ConflictKind tells which kind of NeedsIntervention state the backup is in;
// the settings UI offers Force Push only for ConflictKindDiverged.
type ConflictKind string

const (
	// ConflictKindDiverged: local and remote history have no fast-forward
	// relation. Force Push resolves it by overwriting the remote.
	ConflictKindDiverged ConflictKind = "diverged"
	// ConflictKindFileConflict: a remote change hits a file that was also
	// changed locally. The local branch is behind the remote, so Force Push is
	// refused (ErrForcePushRemoteAhead).
	ConflictKindFileConflict ConflictKind = "fileConflict"
)

type Status struct {
	mu                sync.RWMutex
	LastBackupAt      time.Time
	LastError         string
	NeedsIntervention bool
	ConflictDetails   string
	ConflictKind      ConflictKind
}

func (s *Status) SetSuccess(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastBackupAt = t
	s.LastError = ""
	s.NeedsIntervention = false
	s.ConflictDetails = ""
	s.ConflictKind = ""
}

func (s *Status) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
	s.NeedsIntervention = false
	s.ConflictDetails = ""
	s.ConflictKind = ""
}

func (s *Status) SetNeedsIntervention(kind ConflictKind, details string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NeedsIntervention = true
	s.ConflictKind = kind
	s.ConflictDetails = details
	s.LastError = details
}

// ClearIntervention clears any stale error/conflict state, without touching
// LastBackupAt (unlike SetSuccess — a pull is not a backup, so it must not be
// reported as one).
func (s *Status) ClearIntervention() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = ""
	s.NeedsIntervention = false
	s.ConflictDetails = ""
	s.ConflictKind = ""
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
		ConflictKind:      s.ConflictKind,
	}
}

type StatusSnapshot struct {
	LastBackupAt      *time.Time   `json:"lastBackupAt,omitempty"`
	LastError         string       `json:"lastError,omitempty"`
	NeedsIntervention bool         `json:"needsIntervention,omitempty"`
	ConflictDetails   string       `json:"conflictDetails,omitempty"`
	ConflictKind      ConflictKind `json:"conflictKind,omitempty"`
}
