package wikiresync_test

import (
	"sync"
	"testing"

	wikiresync "github.com/perber/wiki/internal/wiki/resync"
)

func TestResyncJob_NewJobIsIdle(t *testing.T) {
	job := wikiresync.NewResyncJob()
	s := job.Status()

	if s.Running {
		t.Error("new job should not be running")
	}
	if s.Done {
		t.Error("new job should not be done")
	}
	if s.Phase != "" {
		t.Errorf("new job should have no phase, got %q", s.Phase)
	}
	if s.Error != "" {
		t.Errorf("new job should have no error, got %q", s.Error)
	}
}

func TestResyncJob_StartSetsRunning(t *testing.T) {
	job := wikiresync.NewResyncJob()

	if !job.Start() {
		t.Fatal("Start() should return true for a new idle job")
	}

	s := job.Status()
	if !s.Running {
		t.Error("job should be running after Start()")
	}
	if s.Done {
		t.Error("job should not be done after Start()")
	}
}

func TestResyncJob_StartReturnsFalseWhenAlreadyRunning(t *testing.T) {
	job := wikiresync.NewResyncJob()
	job.Start()

	if job.Start() {
		t.Error("Start() should return false when job is already running")
	}
}

func TestResyncJob_SetPhaseUpdatesPhase(t *testing.T) {
	job := wikiresync.NewResyncJob()
	job.Start()
	job.SetPhase(wikiresync.PhaseLinks)

	s := job.Status()
	if s.Phase != string(wikiresync.PhaseLinks) {
		t.Errorf("expected phase %q, got %q", wikiresync.PhaseLinks, s.Phase)
	}
}

func TestResyncJob_FinishSuccess(t *testing.T) {
	job := wikiresync.NewResyncJob()
	job.Start()
	job.SetPhase(wikiresync.PhaseSearch)
	job.Finish(nil)

	s := job.Status()
	if s.Running {
		t.Error("job should not be running after Finish()")
	}
	if !s.Done {
		t.Error("job should be done after Finish()")
	}
	if s.Error != "" {
		t.Errorf("expected no error, got %q", s.Error)
	}
}

func TestResyncJob_FinishWithError(t *testing.T) {
	job := wikiresync.NewResyncJob()
	job.Start()
	job.SetPhase(wikiresync.PhaseTree)

	job.Finish(errTest("tree reconstruction failed"))

	s := job.Status()
	if s.Running {
		t.Error("job should not be running after Finish(err)")
	}
	if !s.Done {
		t.Error("job should be done after Finish(err)")
	}
	if s.Error != "tree reconstruction failed" {
		t.Errorf("expected error message, got %q", s.Error)
	}
}

func TestResyncJob_StartAfterFinishResetsState(t *testing.T) {
	job := wikiresync.NewResyncJob()
	job.Start()
	job.SetPhase(wikiresync.PhaseSearch)
	job.Finish(nil)

	// Should be startable again after finishing
	if !job.Start() {
		t.Fatal("Start() should return true after job has finished")
	}
	s := job.Status()
	if !s.Running {
		t.Error("job should be running after second Start()")
	}
	if s.Done {
		t.Error("job should not be done right after re-Start()")
	}
	if s.Phase != "" {
		t.Errorf("phase should be reset after re-Start(), got %q", s.Phase)
	}
}

func TestResyncJob_ConcurrentStartIsSafe(t *testing.T) {
	job := wikiresync.NewResyncJob()

	var wg sync.WaitGroup
	started := make([]bool, 10)
	for i := range started {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			started[idx] = job.Start()
		}(i)
	}
	wg.Wait()

	count := 0
	for _, ok := range started {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Errorf("exactly one goroutine should win Start(), got %d", count)
	}
}

// errTest is a minimal error for testing.
type errTest string

func (e errTest) Error() string { return string(e) }
