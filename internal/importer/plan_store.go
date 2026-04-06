package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrNoPlan = errors.New("no plan available")
var ErrImportExecutionRunning = errors.New("import execution already running")
var ErrImportCanceled = errors.New("import execution canceled")
var ErrImportStateUnavailable = errors.New("import state unavailable")

type ExecutionStatus string

const (
	ExecutionStatusPlanned   ExecutionStatus = "planned"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCanceled  ExecutionStatus = "canceled"
)

type StoredPlan struct {
	Plan            *PlanResult
	PlanOptions     PlanOptions
	WorkspaceRoot   string
	CreatedAt       time.Time
	ExecutionStatus ExecutionStatus
	ExecutionUserID string
	CancelRequested bool
	ExecutionResult *ExecutionResult
	ExecutionError  *string
	ExecutionProgress
}

type PlanStore struct {
	mu        sync.RWMutex
	plan      *StoredPlan
	stateFile string
	stateErr  error
}

func NewPlanStore(stateFile ...string) *PlanStore {
	ps := &PlanStore{}
	if len(stateFile) > 0 {
		ps.stateFile = stateFile[0]
		if err := ps.load(); err != nil {
			ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
		}
	}
	return ps
}

func (ps *PlanStore) Set(sp *StoredPlan) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.stateErr != nil {
		return ps.stateErr
	}
	ps.plan = sp
	if err := ps.persistLocked(); err != nil {
		ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
		return ps.stateErr
	}
	return nil
}

func (ps *PlanStore) Get() (*StoredPlan, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if ps.stateErr != nil {
		return nil, ps.stateErr
	}
	if ps.plan == nil {
		return nil, ErrNoPlan
	}
	return cloneStoredPlan(ps.plan), nil
}

func (ps *PlanStore) Clear() (*StoredPlan, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.stateErr != nil {
		return nil, ps.stateErr
	}
	old := ps.plan
	ps.plan = nil
	if err := ps.persistLocked(); err != nil {
		ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
		return old, ps.stateErr
	}
	return old, nil
}

func (ps *PlanStore) TryStartExecution(userID string) (*StoredPlan, bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.stateErr != nil {
		return nil, false, ps.stateErr
	}

	if ps.plan == nil {
		return nil, false, ErrNoPlan
	}
	if ps.plan.Plan == nil {
		return nil, false, ErrImportStateUnavailable
	}

	switch ps.plan.ExecutionStatus {
	case ExecutionStatusRunning, ExecutionStatusCompleted:
		return cloneStoredPlan(ps.plan), false, nil
	}

	ps.plan.ExecutionStatus = ExecutionStatusRunning
	ps.plan.ExecutionUserID = userID
	ps.plan.CancelRequested = false
	ps.plan.ExecutionResult = nil
	ps.plan.ExecutionError = nil
	now := time.Now()
	ps.plan.ExecutionProgress = ExecutionProgress{
		ProcessedItems: 0,
		TotalItems:     len(ps.plan.Plan.Items),
		StartedAt:      &now,
	}
	if err := ps.persistLocked(); err != nil {
		ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
		return nil, false, ps.stateErr
	}

	return cloneStoredPlan(ps.plan), true, nil
}

func (ps *PlanStore) FinishExecution(planID string, result *ExecutionResult, execErr error) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.stateErr != nil {
		return ps.stateErr
	}

	if ps.plan == nil || ps.plan.Plan == nil || ps.plan.Plan.ID != planID {
		return nil
	}

	ps.plan.ExecutionResult = result
	ps.plan.ExecutionUserID = ""
	ps.plan.CancelRequested = false

	if execErr != nil {
		finishedAt := time.Now()
		ps.plan.FinishedAt = &finishedAt
		ps.plan.CurrentItemSourcePath = nil
		if errors.Is(execErr, ErrImportCanceled) {
			ps.plan.ExecutionStatus = ExecutionStatusCanceled
			ps.plan.ExecutionError = nil
			if err := ps.persistLocked(); err != nil {
				ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
				return ps.stateErr
			}
			return nil
		}
		errMsg := execErr.Error()
		ps.plan.ExecutionStatus = ExecutionStatusFailed
		ps.plan.ExecutionError = &errMsg
		if err := ps.persistLocked(); err != nil {
			ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
			return ps.stateErr
		}
		return nil
	}

	ps.plan.ExecutionStatus = ExecutionStatusCompleted
	ps.plan.ExecutionError = nil
	finishedAt := time.Now()
	ps.plan.FinishedAt = &finishedAt
	ps.plan.ProcessedItems = ps.plan.TotalItems
	ps.plan.CurrentItemSourcePath = nil
	if err := ps.persistLocked(); err != nil {
		ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
		return ps.stateErr
	}
	return nil
}

func (ps *PlanStore) UpdateExecutionProgress(planID string, progress ExecutionProgress, partialResult *ExecutionResult) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.stateErr != nil {
		return ps.stateErr
	}

	if ps.plan == nil || ps.plan.Plan == nil || ps.plan.Plan.ID != planID {
		return nil
	}

	ps.plan.ProcessedItems = progress.ProcessedItems
	ps.plan.TotalItems = progress.TotalItems
	ps.plan.CurrentItemSourcePath = progress.CurrentItemSourcePath
	if progress.StartedAt != nil {
		ps.plan.StartedAt = progress.StartedAt
	}
	if progress.FinishedAt != nil {
		ps.plan.FinishedAt = progress.FinishedAt
	}
	if partialResult != nil {
		ps.plan.ExecutionResult = cloneExecutionResult(partialResult)
	}
	if err := ps.persistLocked(); err != nil {
		ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
		return ps.stateErr
	}
	return nil
}

func (ps *PlanStore) RequestCancel() (*StoredPlan, bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.stateErr != nil {
		return nil, false, ps.stateErr
	}

	if ps.plan == nil {
		return nil, false, ErrNoPlan
	}
	if ps.plan.ExecutionStatus != ExecutionStatusRunning {
		return cloneStoredPlan(ps.plan), false, nil
	}
	if ps.plan.CancelRequested {
		return cloneStoredPlan(ps.plan), false, nil
	}

	ps.plan.CancelRequested = true
	if err := ps.persistLocked(); err != nil {
		ps.stateErr = fmt.Errorf("%w: %v", ErrImportStateUnavailable, err)
		return nil, false, ps.stateErr
	}
	return cloneStoredPlan(ps.plan), true, nil
}

func (ps *PlanStore) IsCancelRequested(planID string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return ps.plan != nil &&
		ps.plan.Plan != nil &&
		ps.plan.Plan.ID == planID &&
		ps.plan.CancelRequested
}

func (ps *PlanStore) load() error {
	if ps.stateFile == "" {
		return nil
	}

	raw, err := os.ReadFile(ps.stateFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var sp StoredPlan
	if err := json.Unmarshal(raw, &sp); err != nil {
		return err
	}

	ps.plan = &sp
	return nil
}

func (ps *PlanStore) persistLocked() error {
	if ps.stateFile == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(ps.stateFile), 0o755); err != nil {
		return err
	}

	if ps.plan == nil {
		if err := os.Remove(ps.stateFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	raw, err := json.Marshal(ps.plan)
	if err != nil {
		return err
	}

	tmpPath := ps.stateFile + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, ps.stateFile)
}

func cloneStoredPlan(sp *StoredPlan) *StoredPlan {
	if sp == nil {
		return nil
	}

	clone := *sp
	clone.ExecutionResult = cloneExecutionResult(sp.ExecutionResult)
	return &clone
}

func cloneExecutionResult(res *ExecutionResult) *ExecutionResult {
	if res == nil {
		return nil
	}

	clone := *res
	if res.Items != nil {
		clone.Items = append([]ExecutionItemResult(nil), res.Items...)
	}
	return &clone
}
