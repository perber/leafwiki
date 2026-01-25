package importer

import "testing"

func TestPlanStoreSet(t *testing.T) {
	s := NewPlanStore()
	plan := &StoredPlan{}
	s.Set(plan)

	retrieved, err := s.Get()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if retrieved != plan {
		t.Fatalf("expected retrieved plan to be the same as set plan")
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
	plan := &StoredPlan{}
	s.Set(plan)

	retrieved, err := s.Get()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if retrieved != plan {
		t.Fatalf("expected retrieved plan to be the same as set plan")
	}
}

func TestPlanStoreClear(t *testing.T) {
	s := NewPlanStore()
	plan := &StoredPlan{}
	s.Set(plan)

	s.Clear()

	_, err := s.Get()
	if err == nil {
		t.Fatalf("expected error when getting plan from cleared store")
	}
}
