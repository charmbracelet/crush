package agent

import (
	"context"
	"errors"

	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
)

// ErrCompactionBlocked is returned by Summarize when a PreCompact hook
// blocks compaction. The conversation is left as it was.
var ErrCompactionBlocked = errors.New("compaction blocked by hook")

// Compaction triggers reported to PreCompact and PostCompact hooks.
const (
	compactTriggerAuto   = "auto"
	compactTriggerManual = "manual"
)

// fireTurnHook runs one hook for a top-level turn. Subagent turns (the
// task tool, agentic_fetch, etc.) are skipped so a delegated turn does
// not fire the user's hook N times, matching the subagent exclusion
// applied to tool hooks.
func (a *sessionAgent) fireTurnHook(ctx context.Context, call SessionAgentCall, ev hooks.Event) {
	if call.NonInteractive {
		return
	}
	ev.SessionID = call.SessionID
	a.hooks.Run(ctx, ev)
}

// turnEndEvent names how a turn ended so exactly one hook fires for it:
// Interrupt when the user cancelled, StopFailure when the provider
// failed, and TurnEnd otherwise.
func turnEndEvent(err error, assistant *message.Message) hooks.Event {
	switch {
	case errors.Is(err, context.Canceled):
		return hooks.Event{Name: hooks.EventInterrupt}
	case err != nil:
		return hooks.Event{Name: hooks.EventStopFailure, Error: err.Error()}
	}
	ev := hooks.Event{Name: hooks.EventTurnEnd}
	if assistant != nil {
		ev.LastAssistantMessage = new(assistant.Content().String())
	}
	return ev
}
