package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
)

// ErrCompactionBlocked is returned by Summarize when a PreCompact hook
// blocks compaction. The conversation is left as it was.
var ErrCompactionBlocked = errors.New("compaction blocked by hook")

// ErrPromptBlocked is returned by Run when a UserPromptSubmit hook blocks
// the prompt. Nothing is stored and nothing is sent to the model.
var ErrPromptBlocked = errors.New("prompt blocked by hook")

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

// checkPrompt runs UserPromptSubmit hooks for a prompt and returns an
// error wrapping ErrPromptBlocked when a hook denies it or halts the turn.
// Subagent prompts and subagent reports are skipped since the user did not
// type them, and a call that already passed is not checked again.
func (a *sessionAgent) checkPrompt(ctx context.Context, call *SessionAgentCall) error {
	if call.NonInteractive || call.Report != nil || call.promptChecked {
		return nil
	}
	res := a.hooks.Run(ctx, hooks.Event{
		Name:      hooks.EventUserPromptSubmit,
		SessionID: call.SessionID,
		Prompt:    new(call.Prompt),
	})
	if res.Blocked() {
		return fmt.Errorf("%w: %s", ErrPromptBlocked, res.Reason)
	}
	call.promptChecked = true
	return nil
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
