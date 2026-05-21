package backup

import (
	"time"

	"golang.org/x/exp/slog"
)

// Minimum interval to prevent time.NewTicker(0) panic
const minInterval = 1 * time.Minute

// Scheduler runs periodic git backups.
type Scheduler struct {
	repo   *Repository
	ticker *time.Ticker
	manual chan struct{}
	done   chan struct{}
}

// NewScheduler creates and starts the background goroutine.
func NewScheduler(repo *Repository, interval time.Duration) *Scheduler {
	if interval < minInterval {
		slog.Default().Warn("backup scheduler interval too small, using minimum", "requested", interval, "using", minInterval)
		interval = minInterval
	}
	s := &Scheduler{
		repo:   repo,
		ticker: time.NewTicker(interval),
		manual: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}

	go s.run()
	return s
}

func (s *Scheduler) run() {
	// Run immediately on start
	s.repo.RunBackup()

	for {
		select {
		case <-s.ticker.C:
			s.repo.RunBackup()
		case <-s.manual:
			s.repo.RunBackup()
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
}