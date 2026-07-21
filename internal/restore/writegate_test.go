package restore

import (
	"sync"
	"testing"
	"time"
)

func TestWriteGate_NewGateAllowsEntry(t *testing.T) {
	g := NewWriteGate()

	leave, ok := g.TryEnter()
	if !ok {
		t.Fatal("expected TryEnter to succeed on a fresh gate")
	}
	leave()

	if g.Engaged() {
		t.Error("fresh gate should not be engaged")
	}
}

func TestWriteGate_EngageBlocksEntry(t *testing.T) {
	g := NewWriteGate()
	g.Engage()

	if !g.Engaged() {
		t.Error("expected gate to be engaged")
	}
	if _, ok := g.TryEnter(); ok {
		t.Error("expected TryEnter to fail while engaged")
	}
}

func TestWriteGate_DisengageAllowsEntryAgain(t *testing.T) {
	g := NewWriteGate()
	g.Engage()
	g.Disengage()

	if g.Engaged() {
		t.Error("expected gate to be disengaged")
	}
	leave, ok := g.TryEnter()
	if !ok {
		t.Fatal("expected TryEnter to succeed after Disengage")
	}
	leave()
}

func TestWriteGate_WaitForDrain_ReturnsImmediatelyWhenEmpty(t *testing.T) {
	g := NewWriteGate()

	start := time.Now()
	if !g.WaitForDrain(2 * time.Second) {
		t.Fatal("expected WaitForDrain to succeed with no in-flight requests")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("expected WaitForDrain to return promptly, took %v", elapsed)
	}
}

func TestWriteGate_WaitForDrain_WaitsForInFlightRequestsToLeave(t *testing.T) {
	g := NewWriteGate()

	leave, ok := g.TryEnter()
	if !ok {
		t.Fatal("expected TryEnter to succeed")
	}

	done := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		leave()
		close(done)
	}()

	if !g.WaitForDrain(2 * time.Second) {
		t.Fatal("expected WaitForDrain to succeed once the in-flight request left")
	}
	<-done
}

func TestWriteGate_WaitForDrain_TimesOutWhenRequestNeverLeaves(t *testing.T) {
	g := NewWriteGate()

	leave, ok := g.TryEnter()
	if !ok {
		t.Fatal("expected TryEnter to succeed")
	}
	defer leave()

	start := time.Now()
	if g.WaitForDrain(150 * time.Millisecond) {
		t.Fatal("expected WaitForDrain to time out while a request is still in flight")
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Errorf("expected WaitForDrain to wait out the timeout, returned after %v", elapsed)
	}
}

func TestWriteGate_EngageAfterTryEnterStillLetsInFlightRequestFinish(t *testing.T) {
	// Engage only blocks *new* entries; a request already admitted keeps its
	// leave() usable so WaitForDrain can observe it finishing normally.
	g := NewWriteGate()

	leave, ok := g.TryEnter()
	if !ok {
		t.Fatal("expected TryEnter to succeed before Engage")
	}
	g.Engage()

	leave()
	if !g.WaitForDrain(time.Second) {
		t.Fatal("expected drain to succeed after the pre-Engage request left")
	}
}

func TestWriteGate_ConcurrentTryEnterIsSafe(t *testing.T) {
	g := NewWriteGate()

	var wg sync.WaitGroup
	admitted := make([]bool, 50)
	for i := range admitted {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			leave, ok := g.TryEnter()
			admitted[idx] = ok
			if ok {
				leave()
			}
		}(i)
	}
	wg.Wait()

	for i, ok := range admitted {
		if !ok {
			t.Errorf("goroutine %d expected TryEnter to succeed on a disengaged gate", i)
		}
	}
}
