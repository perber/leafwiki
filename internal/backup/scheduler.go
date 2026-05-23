package backup

import (
	"log/slog"
	"sync"
	"time"
)

// Minimum interval to prevent time.NewTicker(0) panic
const minInterval = 1 * time.Minute

// Scheduler runs periodic git backups.
type Scheduler struct {
	repo   *Repository
	ticker *time.Ticker
	manual chan struct{}
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewScheduler creates and starts the background goroutine.
func NewScheduler(repo *Repository, cfg *Config) *Scheduler {
	interval := cfg.Duration()
	if interval < minInterval {
		slog.Warn("backup scheduler interval too small, using minimum", "requested", interval, "using", minInterval)
		interval = minInterval
	}
	s := &Scheduler{
		repo:   repo,
		ticker: time.NewTicker(interval),
		manual: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}

	s.wg.Add(1)
	go s.run()
	return s
}

func (s *Scheduler) run() {
	defer s.wg.Done()

	// Recover from panics to avoid killing the scheduler
	defer func() {
		if r := recover(); r != nil {
			slog.Error("backup scheduler recovered from panic", "panic", r)
		}
	}()

	// Run immediately on start
	if err := s.repo.RunBackup(); err != nil {
		slog.Error("backup failed", "error", err)
	}

	for {
		select {
		case <-s.ticker.C:
			if err := s.repo.RunBackup(); err != nil {
				slog.Error("backup failed", "error", err)
			}
		case <-s.manual:
			if err := s.repo.RunBackup(); err != nil {
				slog.Error("backup failed", "error", err)
			}
		case <-s.done:
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
	s.ticker.Stop()
	select {
	case <-s.done:
		// Already closed
	default:
		close(s.done)
	}
	s.wg.Wait()
}