package restore

import "sync"

type Phase string

const (
	PhaseValidating           Phase = "validating"
	PhaseSwapping             Phase = "swapping"
	PhaseReopeningAuth        Phase = "reopening_auth"
	PhaseInvalidatingSessions Phase = "invalidating_sessions"
	PhaseReloadingBranding    Phase = "reloading_branding"
)

// JobStatus is the snapshot returned by Status().
type JobStatus struct {
	Running           bool   `json:"running"`
	Phase             string `json:"phase,omitempty"`
	Done              bool   `json:"done"`
	Error             string `json:"error,omitempty"`
	VersionWarning    string `json:"versionWarning,omitempty"`
	NeedsIntervention bool   `json:"needsIntervention,omitempty"`
}

// Job tracks the lifecycle of a single background restore operation. Mirrors
// wikiresync.ResyncJob's shape (Start/SetPhase/Finish/Status), plus a
// NeedsIntervention flag set only when rollback itself fails after a
// mid-sequence phase failure.
type Job struct {
	mu                sync.RWMutex
	running           bool
	phase             Phase
	done              bool
	err               error
	versionWarning    string
	needsIntervention bool
}

func NewJob() *Job {
	return &Job{}
}

// Start marks the job as running and resets all state from a previous run.
// Returns false if the job is already running (caller should not start a new goroutine).
func (j *Job) Start() bool {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.running {
		return false
	}
	j.running = true
	j.done = false
	j.phase = ""
	j.err = nil
	j.versionWarning = ""
	j.needsIntervention = false
	return true
}

// SetPhase updates the current phase and is safe to call from the worker goroutine.
func (j *Job) SetPhase(p Phase) {
	j.mu.Lock()
	j.phase = p
	j.mu.Unlock()
}

// SetVersionWarning records a non-fatal snapshot/binary version mismatch.
func (j *Job) SetVersionWarning(msg string) {
	j.mu.Lock()
	j.versionWarning = msg
	j.mu.Unlock()
}

// Finish marks the job as completed (with or without an error) and clears
// running/NeedsIntervention state from the outcome.
func (j *Job) Finish(err error) {
	j.mu.Lock()
	j.running = false
	j.done = true
	j.err = err
	j.mu.Unlock()
}

// FinishNeedsIntervention marks the job done with an error and flags that
// automatic rollback failed too — the instance may be left in a partially
// restored state, and the admin's supported way out is self-restart.
func (j *Job) FinishNeedsIntervention(err error) {
	j.mu.Lock()
	j.running = false
	j.done = true
	j.err = err
	j.needsIntervention = true
	j.mu.Unlock()
}

// Status returns a point-in-time snapshot of the job state.
func (j *Job) Status() JobStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()

	s := JobStatus{
		Running:           j.running,
		Phase:             string(j.phase),
		Done:              j.done,
		VersionWarning:    j.versionWarning,
		NeedsIntervention: j.needsIntervention,
	}
	if j.err != nil {
		s.Error = j.err.Error()
	}
	return s
}
