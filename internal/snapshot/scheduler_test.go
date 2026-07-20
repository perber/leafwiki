package snapshot

import (
	"testing"
	"time"
)

func waitForSnapshotSuccess(t *testing.T, m *Manager, timeout time.Duration) time.Time {
	t.Helper()
	deadline := time.After(timeout)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			last := m.Status().LastSnapshotAt
			if last != nil && !last.IsZero() {
				return *last
			}
		case <-deadline:
			t.Fatal("timeout waiting for snapshot")
		}
	}
}

func TestScheduler_TriggerNow(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Interval = 10 * time.Minute
	m := NewManager(cfg)

	scheduler := NewScheduler(m)
	defer scheduler.Stop()

	initial := waitForSnapshotSuccess(t, m, 2*time.Second)

	scheduler.TriggerNow()

	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if last := m.Status().LastSnapshotAt; last != nil && !last.IsZero() && !last.Equal(initial) {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for TriggerNow")
		}
	}
}

func TestScheduler_Stop(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Interval = 10 * time.Minute
	m := NewManager(cfg)

	scheduler := NewScheduler(m)

	// Stop should block until goroutine finishes, and be safe to call twice.
	scheduler.Stop()
	scheduler.Stop()
}

func TestScheduler_NegativeInterval_ManualOnly(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Interval = -5 * time.Minute
	m := NewManager(cfg)

	scheduler := NewScheduler(m)
	defer scheduler.Stop()

	if m.cfg.Interval != 0 {
		t.Errorf("expected cfg.Interval to be clamped to 0, got %v", m.cfg.Interval)
	}
	if scheduler.ticker != nil {
		t.Error("expected no ticker in manual-only mode")
	}
}

func TestScheduler_SubMinuteInterval_ClampedToMinimum(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Interval = 10 * time.Second
	m := NewManager(cfg)

	scheduler := NewScheduler(m)
	defer scheduler.Stop()

	if m.cfg.Interval != minSnapshotInterval {
		t.Errorf("expected cfg.Interval to be clamped to %v, got %v", minSnapshotInterval, m.cfg.Interval)
	}
	if scheduler.ticker == nil {
		t.Error("expected ticker to be running at minimum interval")
	}
}

func TestScheduler_RunsOnStart(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Interval = 600 * time.Minute
	m := NewManager(cfg)

	scheduler := NewScheduler(m)
	defer scheduler.Stop()

	waitForSnapshotSuccess(t, m, 2*time.Second)
}

func TestScheduler_ManualOnlyStillRunsOnStart(t *testing.T) {
	cfg := newTestConfig(t) // Interval defaults to 0 (manual-only)
	m := NewManager(cfg)

	scheduler := NewScheduler(m)
	defer scheduler.Stop()

	if scheduler.ticker != nil {
		t.Error("expected no ticker in manual-only mode")
	}
	waitForSnapshotSuccess(t, m, 2*time.Second)
}
