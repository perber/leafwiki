package importer

import (
	"errors"
	"sync"
	"time"
)

var ErrNoPlan = errors.New("no plan available")

type StoredPlan struct {
	Plan          *PlanResult
	PlanOptions   PlanOptions
	WorkspaceRoot string
	CreatedAt     time.Time
}

type PlanStore struct {
	mu   sync.RWMutex
	plan *StoredPlan
}

func NewPlanStore() *PlanStore {
	return &PlanStore{}
}

func (ps *PlanStore) Set(sp *StoredPlan) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.plan = sp
}

func (ps *PlanStore) Get() (*StoredPlan, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if ps.plan == nil {
		return nil, ErrNoPlan
	}
	return ps.plan, nil
}

func (ps *PlanStore) Clear() *StoredPlan {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	old := ps.plan
	ps.plan = nil
	return old
}
