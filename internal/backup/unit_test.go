package backup

import (
	"encoding/json"
	"fmt"
	"strings"
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

func TestFileConflictMessage_CapsListedPaths(t *testing.T) {
	paths := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("root/p%02d.md", i)
		}
		return out
	}
	tests := []struct {
		name     string
		paths    []string
		wantMore string // "" means no "and N more" suffix
		lastShow string
		hidden   string
	}{
		{"one path", paths(1), "", "root/p00.md", ""},
		{"exactly the cap", paths(10), "", "root/p09.md", ""},
		{"one over the cap", paths(11), " and 1 more", "root/p09.md", "root/p10.md"},
		{"many over the cap", paths(25), " and 15 more", "root/p09.md", "root/p24.md"},
		{"unicode and spaces", []string{"root/Über uns/ä ö.md"}, "", "root/Über uns/ä ö.md", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := fileConflictMessage(tt.paths)
			if !strings.Contains(msg, tt.lastShow) {
				t.Errorf("message %q does not name %q", msg, tt.lastShow)
			}
			if tt.hidden != "" && strings.Contains(msg, tt.hidden) {
				t.Errorf("message %q names %q beyond the cap", msg, tt.hidden)
			}
			if tt.wantMore == "" {
				if strings.Contains(msg, " more") {
					t.Errorf("message %q has an unexpected 'more' suffix", msg)
				}
			} else if !strings.Contains(msg, tt.lastShow+tt.wantMore+";") {
				t.Errorf("message %q, want %q right after the last listed path", msg, tt.wantMore)
			}
		})
	}
}

func TestStatusSnapshot_ConflictKindJSON(t *testing.T) {
	tests := []struct {
		name string
		kind ConflictKind
		want string // "" means the field must be omitted
	}{
		{"diverged", ConflictKindDiverged, `"conflictKind":"diverged"`},
		{"fileConflict", ConflictKindFileConflict, `"conflictKind":"fileConflict"`},
		{"cleared", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(StatusSnapshot{NeedsIntervention: tt.kind != "", ConflictKind: tt.kind})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			got := string(b)
			if tt.want == "" && strings.Contains(got, "conflictKind") {
				t.Errorf("JSON %s, want conflictKind omitted", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("JSON %s, want it to contain %s", got, tt.want)
			}
		})
	}
}
