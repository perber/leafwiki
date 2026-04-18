package importer

import (
	"context"
	"io"

	coreimporter "github.com/perber/wiki/internal/importer"
)

// ─── CreateImportPlanUseCase ─────────────────────────────────────────────────

type CreateImportPlanInput struct {
	File           io.Reader
	TargetBasePath string
}

type CreateImportPlanOutput struct {
	Plan *coreimporter.CurrentPlanState
}

type CreateImportPlanUseCase struct {
	svc *coreimporter.ImporterService
}

func NewCreateImportPlanUseCase(svc *coreimporter.ImporterService) *CreateImportPlanUseCase {
	return &CreateImportPlanUseCase{svc: svc}
}

func (uc *CreateImportPlanUseCase) Execute(_ context.Context, in CreateImportPlanInput) (*CreateImportPlanOutput, error) {
	if _, err := uc.svc.CreateImportPlanFromZipUpload(in.File, in.TargetBasePath); err != nil {
		return nil, err
	}
	plan, err := uc.svc.GetCurrentPlan()
	if err != nil {
		return nil, err
	}
	return &CreateImportPlanOutput{Plan: plan}, nil
}

// ─── GetImportPlanUseCase ────────────────────────────────────────────────────

type GetImportPlanOutput struct {
	Plan *coreimporter.CurrentPlanState
}

type GetImportPlanUseCase struct {
	svc *coreimporter.ImporterService
}

func NewGetImportPlanUseCase(svc *coreimporter.ImporterService) *GetImportPlanUseCase {
	return &GetImportPlanUseCase{svc: svc}
}

func (uc *GetImportPlanUseCase) Execute(_ context.Context) (*GetImportPlanOutput, error) {
	plan, err := uc.svc.GetCurrentPlan()
	if err != nil {
		return nil, err
	}
	return &GetImportPlanOutput{Plan: plan}, nil
}

// ─── ExecuteImportUseCase ────────────────────────────────────────────────────

type ExecuteImportInput struct {
	UserID string
}

type ExecuteImportOutput struct {
	State   *coreimporter.CurrentPlanState
	Started bool
}

type ExecuteImportUseCase struct {
	svc *coreimporter.ImporterService
}

func NewExecuteImportUseCase(svc *coreimporter.ImporterService) *ExecuteImportUseCase {
	return &ExecuteImportUseCase{svc: svc}
}

func (uc *ExecuteImportUseCase) Execute(_ context.Context, in ExecuteImportInput) (*ExecuteImportOutput, error) {
	state, started, err := uc.svc.StartCurrentPlanExecution(in.UserID)
	if err != nil {
		return nil, err
	}
	return &ExecuteImportOutput{State: state, Started: started}, nil
}

// ─── ClearImportPlanUseCase ──────────────────────────────────────────────────

type ClearImportPlanUseCase struct {
	svc *coreimporter.ImporterService
}

func NewClearImportPlanUseCase(svc *coreimporter.ImporterService) *ClearImportPlanUseCase {
	return &ClearImportPlanUseCase{svc: svc}
}

func (uc *ClearImportPlanUseCase) Execute(_ context.Context) (*coreimporter.CurrentPlanState, error) {
	state, _, err := uc.svc.CancelCurrentPlan()
	if err == nil && state != nil && state.ExecutionStatus == coreimporter.ExecutionStatusRunning && state.CancelRequested {
		return state, nil
	}
	if err != nil && err != coreimporter.ErrNoPlan {
		return nil, err
	}
	if err := uc.svc.ClearCurrentPlan(); err != nil {
		return nil, err
	}
	return nil, nil
}
