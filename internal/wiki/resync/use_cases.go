package wikiresync

import (
	"context"

	httpmetrics "github.com/perber/wiki/internal/http/metrics"
)

// ─── TriggerResyncUseCase ─────────────────────────────────────────────────────

type TriggerResyncUseCase struct {
	job     *ResyncJob
	trigger func()
	metrics *httpmetrics.HTTPMetrics
}

func NewTriggerResyncUseCase(job *ResyncJob, trigger func(), metrics *httpmetrics.HTTPMetrics) *TriggerResyncUseCase {
	return &TriggerResyncUseCase{job: job, trigger: trigger, metrics: metrics}
}

func (uc *TriggerResyncUseCase) Execute(_ context.Context) error {
	if !uc.job.Start() {
		return ErrResyncAlreadyRunning
	}
	uc.trigger()
	return nil
}

// ─── GetResyncStatusUseCase ───────────────────────────────────────────────────

type GetResyncStatusOutput struct {
	Status JobStatus
}

type GetResyncStatusUseCase struct {
	job *ResyncJob
}

func NewGetResyncStatusUseCase(job *ResyncJob) *GetResyncStatusUseCase {
	return &GetResyncStatusUseCase{job: job}
}

func (uc *GetResyncStatusUseCase) Execute(_ context.Context) GetResyncStatusOutput {
	return GetResyncStatusOutput{Status: uc.job.Status()}
}
