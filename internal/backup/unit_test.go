package backup

import (
	"sync"
	"testing"
	"time"
)

// --- Status ---

func TestStatus_SetSuccessClearsAllFields(t *testing.T) {
	s := &Status{}
	s.SetNeedsIntervention(ConflictKindDiverged, "conflict details")
	s.SetSuccess(time.Now())

	snap := s.Snapshot()
	if snap.LastError != "" {
		t.Errorf("expected empty LastError, got %q", snap.LastError)
	}
	if snap.NeedsIntervention {
		t.Error("expected NeedsIntervention to be cleared by SetSuccess")
	}
	if snap.ConflictDetails != "" {
		t.Errorf("expected empty ConflictDetails, got %q", snap.ConflictDetails)
	}
	if snap.LastBackupAt == nil {
		t.Error("expected LastBackupAt to be set")
	}
}

func TestStatus_SetNeedsInterventionSetsAllFields(t *testing.T) {
	s := &Status{}
	s.SetNeedsIntervention(ConflictKindFileConflict, "some conflict")

	snap := s.Snapshot()
	if !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true")
	}
	if snap.ConflictDetails != "some conflict" {
		t.Errorf("expected ConflictDetails %q, got %q", "some conflict", snap.ConflictDetails)
	}
	if snap.LastError != "some conflict" {
		t.Errorf("expected LastError %q, got %q", "some conflict", snap.LastError)
	}
}

func TestStatus_ConflictKind(t *testing.T) {
	resets := []struct {
		name  string
		reset func(*Status)
	}{
		{"SetSuccess", func(s *Status) { s.SetSuccess(time.Now()) }},
		{"SetError", func(s *Status) { s.SetError("network down") }},
		{"ClearIntervention", func(s *Status) { s.ClearIntervention() }},
	}
	for _, kind := range []ConflictKind{ConflictKindDiverged, ConflictKindFileConflict} {
		t.Run(string(kind)+"/set", func(t *testing.T) {
			s := &Status{}
			s.SetNeedsIntervention(kind, "details")
			if got := s.Snapshot().ConflictKind; got != kind {
				t.Errorf("ConflictKind = %q, want %q", got, kind)
			}
		})
		for _, r := range resets {
			t.Run(string(kind)+"/"+r.name+" resets", func(t *testing.T) {
				s := &Status{}
				s.SetNeedsIntervention(kind, "details")
				r.reset(s)
				if got := s.Snapshot().ConflictKind; got != "" {
					t.Errorf("ConflictKind = %q after %s, want empty", got, r.name)
				}
			})
		}
	}
}

func TestStatus_SnapshotZeroTimeReturnsNilPointer(t *testing.T) {
	s := &Status{}
	snap := s.Snapshot()
	if snap.LastBackupAt != nil {
		t.Error("expected nil LastBackupAt for zero-value Status")
	}
}

func TestStatus_ConcurrentAccess(t *testing.T) {
	s := &Status{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.SetSuccess(time.Now())
		}()
		go func() {
			defer wg.Done()
			_ = s.Snapshot()
		}()
	}
	wg.Wait()
}
