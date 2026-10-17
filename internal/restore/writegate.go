package restore

import (
	"sync"
	"sync/atomic"
	"time"
)

// WriteGate blocks mutating HTTP requests while a restore is swapping live
// data out from under the running server. It has no HTTP dependency itself —
// internal/http/middleware/maintenance wraps it as gin middleware.
type WriteGate struct {
	mu      sync.Mutex
	engaged bool
	// atomic.Int64 rather than a raw int64: it is guaranteed 64-bit aligned,
	// so this doesn't panic on 32-bit platforms (linux/386).
	inflight atomic.Int64
}

func NewWriteGate() *WriteGate {
	return &WriteGate{}
}

// Engage blocks new mutating requests (via TryEnter) starting now.
func (g *WriteGate) Engage() {
	g.mu.Lock()
	g.engaged = true
	g.mu.Unlock()
}

// Disengage allows mutating requests again.
func (g *WriteGate) Disengage() {
	g.mu.Lock()
	g.engaged = false
	g.mu.Unlock()
}

// Engaged reports whether the gate currently blocks new mutating requests.
func (g *WriteGate) Engaged() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.engaged
}

// TryEnter admits one in-flight mutating request, returning ok=false if the
// gate is engaged. On success, the caller must call leave exactly once when
// the request finishes.
func (g *WriteGate) TryEnter() (leave func(), ok bool) {
	g.mu.Lock()
	if g.engaged {
		g.mu.Unlock()
		return nil, false
	}
	g.inflight.Add(1)
	g.mu.Unlock()
	return func() { g.inflight.Add(-1) }, true
}

// WaitForDrain blocks until every request admitted by TryEnter before Engage
// was called has finished, or timeout elapses (returns false in that case,
// not treated as fatal by callers — see Manager.runLocked). Intended to be
// called right after Engage(), so no new requests can be admitted while this
// waits for requests that started just before Engage() to finish.
func (g *WriteGate) WaitForDrain(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if g.inflight.Load() == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}
