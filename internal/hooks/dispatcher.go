package hooks

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"
)

// Dispatcher runs hooks for several events, one Runner per event so matchers
// stay scoped. The nil value is valid and does nothing.
type Dispatcher struct {
	runners map[string]*Runner
}

// NewDispatcher returns nil when no event has any hook configured.
func NewDispatcher(configured map[string][]config.HookConfig, cwd, projectDir string) *Dispatcher {
	runners := make(map[string]*Runner, len(configured))
	for event, eventHooks := range configured {
		if len(eventHooks) == 0 {
			continue
		}
		runners[event] = NewRunner(eventHooks, cwd, projectDir)
	}
	if len(runners) == 0 {
		return nil
	}
	return &Dispatcher{runners: runners}
}

// Has reports whether any hook is configured for the event.
func (d *Dispatcher) Has(event string) bool {
	if d == nil {
		return false
	}
	return d.runners[event] != nil
}

// Run returns the aggregated result of the event's hooks. Errors are logged
// rather than returned: a broken hook must not derail an agent turn.
func (d *Dispatcher) Run(ctx context.Context, event string, data EventData) AggregateResult {
	if d == nil || d.runners[event] == nil {
		return AggregateResult{Decision: DecisionNone}
	}
	result, err := d.runners[event].RunEvent(ctx, event, data)
	if err != nil {
		slog.Warn("Hook execution error", "event", event, "error", err)
		return AggregateResult{Decision: DecisionNone}
	}
	return result
}
