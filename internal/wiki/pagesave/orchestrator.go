package pagesave

// PageSideEffect is implemented by any component that reacts to a page mutation.
// Apply is always called synchronously and errors are handled internally (best-effort).
type PageSideEffect interface {
	Apply(event PageSaveEvent)
}

// PageSaveOrchestrator fans out a PageSaveEvent to all registered side effects.
type PageSaveOrchestrator struct {
	sideEffects []PageSideEffect
}

// NewPageSaveOrchestrator creates an orchestrator with the given side effects.
func NewPageSaveOrchestrator(effects ...PageSideEffect) *PageSaveOrchestrator {
	return &PageSaveOrchestrator{sideEffects: effects}
}

// Run delivers the event to each side effect in registration order.
func (o *PageSaveOrchestrator) Run(event PageSaveEvent) {
	for _, se := range o.sideEffects {
		se.Apply(event)
	}
}
