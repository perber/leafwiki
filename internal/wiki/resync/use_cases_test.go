package wikiresync_test

import (
	"context"
	"errors"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	. "github.com/perber/wiki/internal/wiki/resync"
)

func TestTriggerResyncUseCase_Execute_LaunchesTrigger(t *testing.T) {
	job := NewResyncJob()
	called := false
	uc := NewTriggerResyncUseCase(job, func() { called = true })

	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !called {
		t.Error("trigger was not called")
	}
}

func TestTriggerResyncUseCase_Execute_ReturnsLocalizedErrorWhenAlreadyRunning(t *testing.T) {
	job := NewResyncJob()
	job.Start() // simulate running

	uc := NewTriggerResyncUseCase(job, func() { t.Error("trigger must not be called") })
	err := uc.Execute(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected LocalizedError, got %T", err)
	}
	if loc.Code != ErrCodeResyncAlreadyRunning {
		t.Errorf("expected code %q, got %q", ErrCodeResyncAlreadyRunning, loc.Code)
	}
}

func TestGetResyncStatusUseCase_Execute_ReturnsJobStatus(t *testing.T) {
	job := NewResyncJob()
	job.Start()
	job.SetPhase(PhaseTags)
	job.Finish(errors.New("something went wrong"))

	uc := NewGetResyncStatusUseCase(job)
	out := uc.Execute(context.Background())

	if out.Status.Running {
		t.Error("finished job should not be running")
	}
	if !out.Status.Done {
		t.Error("expected done=true")
	}
	if out.Status.Error != "something went wrong" {
		t.Errorf("unexpected error message: %q", out.Status.Error)
	}
}
