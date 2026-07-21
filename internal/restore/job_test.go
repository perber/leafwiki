package restore

import "testing"

func TestJob_NewJobIsIdle(t *testing.T) {
	job := NewJob()
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
	if s.NeedsIntervention {
		t.Error("new job should not need intervention")
	}
}

func TestJob_StartSetsRunning(t *testing.T) {
	job := NewJob()

	if !job.Start() {
		t.Fatal("Start() should return true for a new idle job")
	}
	s := job.Status()
	if !s.Running {
		t.Error("job should be running after Start()")
	}
}

func TestJob_StartReturnsFalseWhenAlreadyRunning(t *testing.T) {
	job := NewJob()
	job.Start()

	if job.Start() {
		t.Error("Start() should return false when job is already running")
	}
}

func TestJob_SetPhaseUpdatesPhase(t *testing.T) {
	job := NewJob()
	job.Start()
	job.SetPhase(PhaseSwapping)

	if s := job.Status(); s.Phase != string(PhaseSwapping) {
		t.Errorf("expected phase %q, got %q", PhaseSwapping, s.Phase)
	}
}

func TestJob_FinishSuccess(t *testing.T) {
	job := NewJob()
	job.Start()
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
	if s.NeedsIntervention {
		t.Error("Finish(nil) should not set NeedsIntervention")
	}
}

func TestJob_FinishNeedsIntervention(t *testing.T) {
	job := NewJob()
	job.Start()
	job.FinishNeedsIntervention(errTest("rollback also failed"))

	s := job.Status()
	if s.Running {
		t.Error("job should not be running after FinishNeedsIntervention()")
	}
	if !s.Done {
		t.Error("job should be done after FinishNeedsIntervention()")
	}
	if !s.NeedsIntervention {
		t.Error("expected NeedsIntervention = true")
	}
	if s.Error != "rollback also failed" {
		t.Errorf("expected error message, got %q", s.Error)
	}
}

func TestJob_StartAfterFinishNeedsInterventionResetsFlag(t *testing.T) {
	job := NewJob()
	job.Start()
	job.FinishNeedsIntervention(errTest("boom"))

	if !job.Start() {
		t.Fatal("expected Start() to succeed again after a finished job")
	}
	if s := job.Status(); s.NeedsIntervention {
		t.Error("expected NeedsIntervention to reset on a fresh Start()")
	}
}

func TestJob_SetVersionWarning(t *testing.T) {
	job := NewJob()
	job.Start()
	job.SetVersionWarning("snapshot was created by v0.9.0, this server is running v0.10.0")

	if s := job.Status(); s.VersionWarning == "" {
		t.Error("expected version warning to be set")
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
