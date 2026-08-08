package importer

import (
	"context"
	"errors"
	"io"

	coreshared "github.com/perber/wiki/internal/core/shared"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
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
		// Deliberately static, user-facing messages: err's wrapped chain
		// (e.g. "extract zip to temp: extract zip: write file: file too
		// large: 200 bytes (max 100)") is internal implementation detail —
		// function names and call-path context, not anything a user should
		// see. logRejectedZipExtraction (routes.go) logs it server-side
		// instead.
		if errors.Is(err, coreshared.ErrFileTooLarge) {
			return nil, sharederrors.NewLocalizedError(
				ErrCodeImporterZipEntryTooLarge, "A file inside the uploaded archive is too large",
				"zip entry exceeds the per-file size limit", err,
			)
		}
		if errors.Is(err, coreshared.ErrCumulativeSizeTooLarge) {
			return nil, sharederrors.NewLocalizedError(
				ErrCodeImporterZipExtractedTooLarge, "The uploaded archive is too large once decompressed",
				"decompressed zip contents exceed the total size limit", err,
			)
		}
		if errors.Is(err, coreshared.ErrDecompressionRatioTooHigh) {
			return nil, sharederrors.NewLocalizedError(
				ErrCodeImporterZipRatioTooHigh, "The uploaded archive looks like a decompression bomb",
				"zip entry's decompression ratio exceeds the allowed maximum", err,
			)
		}
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
		if errors.Is(err, coreimporter.ErrNoPlan) {
			return nil, sharederrors.NewLocalizedError("importer_no_plan", "No import plan available", "no import plan available", err)
		}
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
		if errors.Is(err, coreimporter.ErrImportExecutionRunning) {
			return nil, sharederrors.NewLocalizedError("importer_execution_running", "Import is already running", "import is already running", err)
		}
		if errors.Is(err, coreimporter.ErrNoPlan) {
			return nil, sharederrors.NewLocalizedError("importer_no_plan", "No import plan available", "no import plan available", err)
		}
		if errors.Is(err, coreimporter.ErrImportStateUnavailable) {
			return nil, sharederrors.NewLocalizedError("importer_state_unavailable", "Import state is unavailable", "import state is unavailable", err)
		}
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
	if err != nil && !errors.Is(err, coreimporter.ErrNoPlan) {
		if errors.Is(err, coreimporter.ErrImportStateUnavailable) {
			return nil, sharederrors.NewLocalizedError("importer_state_unavailable", "Import state is unavailable", "import state is unavailable", err)
		}
		return nil, err
	}
	if err := uc.svc.ClearCurrentPlan(); err != nil {
		return nil, err
	}
	return nil, nil
}
