package importer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanStoreSet(t *testing.T) {
	s := NewPlanStore()
	plan := &StoredPlan{ExecutionStatus: ExecutionStatusPlanned}
	if err := s.Set(plan); err != nil {
		t.Fatalf("Set err: %v", err)
	}

	retrieved, err := s.Get()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if retrieved == plan {
		t.Fatalf("expected Get to return a snapshot copy")
	}
	if retrieved.ExecutionStatus != plan.ExecutionStatus {
		t.Fatalf("expected execution status %q, got %q", plan.ExecutionStatus, retrieved.ExecutionStatus)
	}
}

func TestPlanStoreGet(t *testing.T) {
	s := NewPlanStore()

	_, err := s.Get()
	if err == nil {
		t.Fatalf("expected error when getting plan from empty store")
	}
}

func TestPlanStoreSetAndGet(t *testing.T) {
	s := NewPlanStore()
	plan := &StoredPlan{
		Plan:            &PlanResult{ID: "plan-1"},
		ExecutionStatus: ExecutionStatusPlanned,
	}
	if err := s.Set(plan); err != nil {
		t.Fatalf("Set err: %v", err)
	}

	retrieved, err := s.Get()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if retrieved == plan {
		t.Fatalf("expected Get to return a snapshot copy")
	}
	if retrieved.Plan == nil || retrieved.Plan.ID != "plan-1" {
		t.Fatalf("expected retrieved plan ID to match, got %#v", retrieved.Plan)
	}
}

func TestPlanStoreClear(t *testing.T) {
	s := NewPlanStore()
	plan := &StoredPlan{}
	if err := s.Set(plan); err != nil {
		t.Fatalf("Set err: %v", err)
	}

	if _, err := s.Clear(); err != nil {
		t.Fatalf("Clear err: %v", err)
	}

	_, err := s.Get()
	if err == nil {
		t.Fatalf("expected error when getting plan from cleared store")
	}
}

func TestPlanStore_PersistsAndLoadsState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "current-plan.json")
	store := NewPlanStore(stateFile)
	if err := store.Set(&StoredPlan{
		Plan:            &PlanResult{ID: "plan-1"},
		ExecutionStatus: ExecutionStatusRunning,
		ExecutionUserID: "user-1",
	}); err != nil {
		t.Fatalf("Set err: %v", err)
	}

	loaded := NewPlanStore(stateFile)
	retrieved, err := loaded.Get()
	if err != nil {
		t.Fatalf("expected persisted plan, got %v", err)
	}
	if retrieved.Plan == nil || retrieved.Plan.ID != "plan-1" {
		t.Fatalf("expected loaded plan id plan-1, got %#v", retrieved.Plan)
	}
	if retrieved.ExecutionUserID != "user-1" {
		t.Fatalf("expected persisted execution user, got %q", retrieved.ExecutionUserID)
	}
}

func TestPlanStore_LoadError_IsReported(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "current-plan.json")
	if err := os.WriteFile(stateFile, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile err: %v", err)
	}

	store := NewPlanStore(stateFile)
	if _, err := store.Get(); !errors.Is(err, ErrImportStateUnavailable) {
		t.Fatalf("expected ErrImportStateUnavailable, got %v", err)
	}
}

func TestPlanStoreTryStartExecution_WithNilPlanPayload(t *testing.T) {
	store := NewPlanStore()
	if err := store.Set(&StoredPlan{
		Plan:            nil,
		ExecutionStatus: ExecutionStatusPlanned,
	}); err != nil {
		t.Fatalf("Set err: %v", err)
	}

	_, _, err := store.TryStartExecution("user-1")
	if !errors.Is(err, ErrImportStateUnavailable) {
		t.Fatalf("expected ErrImportStateUnavailable, got %v", err)
	}
}

func TestPlanStoreUpdateExecutionProgress_UpdatesEmbeddedFields(t *testing.T) {
	store := NewPlanStore()
	if err := store.Set(&StoredPlan{
		Plan:            &PlanResult{ID: "plan-1"},
		ExecutionStatus: ExecutionStatusRunning,
	}); err != nil {
		t.Fatalf("Set err: %v", err)
	}

	now := time.Now()
	sourcePath := "docs/readme.md"
	err := store.UpdateExecutionProgress("plan-1", ExecutionProgress{
		ProcessedItems:        2,
		TotalItems:            5,
		CurrentItemSourcePath: &sourcePath,
		StartedAt:             &now,
	}, nil)
	if err != nil {
		t.Fatalf("UpdateExecutionProgress err: %v", err)
	}

	retrieved, err := store.Get()
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if retrieved.ProcessedItems != 2 || retrieved.TotalItems != 5 {
		t.Fatalf("unexpected progress values: %#v", retrieved.ExecutionProgress)
	}
	if retrieved.CurrentItemSourcePath == nil || *retrieved.CurrentItemSourcePath != sourcePath {
		t.Fatalf("unexpected current item source path: %#v", retrieved.CurrentItemSourcePath)
	}
	if retrieved.StartedAt == nil || !retrieved.StartedAt.Equal(now) {
		t.Fatalf("expected started_at to be updated, got %#v", retrieved.StartedAt)
	}
}
