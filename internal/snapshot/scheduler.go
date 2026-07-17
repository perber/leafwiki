package snapshot

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// minSnapshotInterval is the smallest allowed non-zero periodic interval.
const minSnapshotInterval = 1 * time.Minute

// Scheduler runs periodic snapshots.
// When created with interval == 0 it operates in manual-only mode: no
// automatic ticker fires, but TriggerNow() and the initial startup run still work.
type Scheduler struct {
	manager   *Manager
	ticker    *time.Ticker // nil in manual-only mode
	manual    chan struct{}
	done      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewScheduler creates and starts the background goroutine.
// The interval is taken from manager.cfg.Interval; 0 = manual-only mode.
func NewScheduler(manager *Manager) *Scheduler {
	interval := manager.cfg.Interval

	if interval < 0 {
		slog.Warn("snapshot scheduler interval is negative, switching to manual-only mode", "requested", interval)
		interval = 0
		manager.cfg.Interval = 0
	}

	s := &Scheduler{
		manager: manager,
		manual:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}

	if interval > 0 {
		if interval < minSnapshotInterval {
			slog.Warn("snapshot scheduler interval too small, using minimum", "requested", interval, "using", minSnapshotInterval)
			interval = minSnapshotInterval
			manager.cfg.Interval = minSnapshotInterval
		}
		s.ticker = time.NewTicker(interval)
	}

	s.manual <- struct{}{} // pre-seed: fires an immediate run on startup

	s.wg.Add(1)
	go s.run()
	return s
}

func (s *Scheduler) run() {
	defer s.wg.Done()

	// A nil channel blocks forever, so tickerC is never selected in manual-only mode.
	var tickerC <-chan time.Time
	if s.ticker != nil {
		tickerC = s.ticker.C
	}

	for {
		var done bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("snapshot scheduler recovered from panic, will retry on next tick", "panic", r)
				}
			}()
			select {
			case <-tickerC:
				if err := s.manager.RunOnce(context.Background()); err != nil {
					slog.Error("snapshot failed", "error", err)
				}
			case <-s.manual:
				if err := s.manager.RunOnce(context.Background()); err != nil {
					slog.Error("snapshot failed", "error", err)
				}
			case <-s.done:
				done = true
			}
		}()
		if done {
			return
		}
	}
}

// TriggerNow signals the scheduler to run a snapshot immediately,
// regardless of the interval. Non-blocking. Returns false without blocking
// if a manual trigger is already pending (or a run is in progress and the
// buffered signal from a prior trigger hasn't been picked up yet), so the
// caller can tell the difference between "queued" and "dropped" instead of
// assuming every call succeeds.
func (s *Scheduler) TriggerNow() bool {
	select {
	case s.manual <- struct{}{}:
		return true
	default:
		return false
	}
}

// Stop shuts down the goroutine cleanly.
func (s *Scheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.closeOnce.Do(func() {
		close(s.done)
	})
	s.wg.Wait()
}
