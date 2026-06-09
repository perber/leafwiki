package backup

import (
	"log/slog"
	"sync"
	"time"
)

// minInterval is the smallest allowed non-zero periodic interval.
const minInterval = 1 * time.Minute

// Scheduler runs periodic git backups.
// When created with interval == 0 it operates in manual-only mode: no
// automatic ticker fires, but TriggerNow() and the initial startup run still work.
type Scheduler struct {
	repo      *Repository
	ticker    *time.Ticker // nil in manual-only mode
	manual    chan struct{}
	done      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewScheduler creates and starts the background goroutine.
// The interval is taken from repo.cfg.Interval; 0 = manual-only mode.
func NewScheduler(repo *Repository) *Scheduler {
	interval := repo.cfg.Interval

	if interval < 0 {
		slog.Warn("backup scheduler interval is negative, switching to manual-only mode", "requested", interval)
		interval = 0
		repo.cfg.Interval = 0
	}

	s := &Scheduler{
		repo:   repo,
		manual: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}

	if interval > 0 {
		if interval < minInterval {
			slog.Warn("backup scheduler interval too small, using minimum", "requested", interval, "using", minInterval)
			interval = minInterval
			repo.cfg.Interval = minInterval
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
					slog.Error("backup scheduler recovered from panic, will retry on next tick", "panic", r)
				}
			}()
			select {
			case <-tickerC:
				if err := s.repo.RunBackup(); err != nil {
					slog.Error("backup failed", "error", err)
				}
			case <-s.manual:
				if err := s.repo.RunBackup(); err != nil {
					slog.Error("backup failed", "error", err)
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

// TriggerNow signals the scheduler to run a backup immediately,
// regardless of the interval. Non-blocking.
func (s *Scheduler) TriggerNow() {
	select {
	case s.manual <- struct{}{}:
	default:
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
