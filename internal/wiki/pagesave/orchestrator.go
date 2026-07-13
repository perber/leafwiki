package pagesave

import (
	"time"

	httpmetrics "github.com/perber/wiki/internal/http/metrics"
)

// PageSideEffect is implemented by any component that reacts to a page mutation.
// Apply is always called synchronously and errors are handled internally (best-effort).
type PageSideEffect interface {
	Apply(event PageSaveEvent)
}

type namedPageSideEffect interface {
	Name() string
}

// PageSaveOrchestrator fans out a PageSaveEvent to all registered side effects.
type PageSaveOrchestrator struct {
	metrics     *httpmetrics.HTTPMetrics
	sideEffects []PageSideEffect
}

// NewPageSaveOrchestrator creates an orchestrator with the given side effects.
func NewPageSaveOrchestrator(metrics *httpmetrics.HTTPMetrics, effects ...PageSideEffect) *PageSaveOrchestrator {
	return &PageSaveOrchestrator{metrics: metrics, sideEffects: effects}
}

// Run delivers the event to each side effect in registration order.
func (o *PageSaveOrchestrator) Run(event PageSaveEvent) {
	for _, se := range o.sideEffects {
		started := time.Now()
		se.Apply(event)
		o.metrics.ObservePageSaveSideEffect(string(event.Operation), sideEffectName(se), started)
	}
}

func sideEffectName(effect PageSideEffect) string {
	if named, ok := effect.(namedPageSideEffect); ok {
		return named.Name()
	}
	return "unknown"
}
