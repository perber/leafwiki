package wikiresync

import "sync"

type Phase string

const (
	PhaseTree   Phase = "tree"
	PhaseLinks  Phase = "links"
	PhaseTags   Phase = "tags"
	PhaseSearch Phase = "search"
)

// JobStatus is the snapshot returned by Status().
type JobStatus struct {
	Running bool   `json:"running"`
	Phase   string `json:"phase,omitempty"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

// ResyncJob tracks the lifecycle of a single background resync operation.
// It is safe for concurrent use.
type ResyncJob struct {
	mu      sync.RWMutex
	running bool
	phase   Phase
	done    bool
	err     error
}

func NewResyncJob() *ResyncJob {
	return &ResyncJob{}
}

// Start marks the job as running and resets all state from a previous run.
// Returns false if the job is already running (caller should not start a new goroutine).
func (j *ResyncJob) Start() bool {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.running {
		return false
	}
	j.running = true
	j.done = false
	j.phase = ""
	j.err = nil
	return true
}

// SetPhase updates the current phase and is safe to call from the worker goroutine.
func (j *ResyncJob) SetPhase(p Phase) {
	j.mu.Lock()
	j.phase = p
	j.mu.Unlock()
}

// Finish marks the job as completed (with or without an error).
func (j *ResyncJob) Finish(err error) {
	j.mu.Lock()
	j.running = false
	j.done = true
	j.err = err
	j.mu.Unlock()
}

// Status returns a point-in-time snapshot of the job state.
func (j *ResyncJob) Status() JobStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()

	s := JobStatus{
		Running: j.running,
		Phase:   string(j.phase),
		Done:    j.done,
	}
	if j.err != nil {
		s.Error = j.err.Error()
	}
	return s
}
