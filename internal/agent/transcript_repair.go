package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/message"
)

// interruptedTurnTitle and interruptedTurnDetails are the finish part
// written over an assistant message that a run abandoned. The title is
// what the chat renders as the message's error banner.
const (
	interruptedTurnTitle   = "Interrupted"
	interruptedTurnDetails = "The run ended before this response finished."
)

// RepairInterruptedSession gives a terminal state to every assistant
// message of a session that a run abandoned: an unfinished message gets
// a finish part, and a tool call with no stored result gets one. The
// chat derives its spinners from the transcript alone — an assistant
// message with no finish part spins forever
// (`AssistantMessageItem.isSpinning`) and a resultless tool call renders
// "Waiting for tool response..." forever — so a session whose run died
// with the server is unusable and indistinguishable from a working one
// until the stored transcript itself says the turn ended.
//
// The caller MUST have established that the session has no live run;
// this rewrites messages an in-flight turn still owns. That check is the
// coordinator's IsSessionBusy, which only knows about runs in this
// process, so the guard assumes one server per project database.
//
// It is idempotent and does nothing when the transcript is already
// terminal, which is the common case on every session load.
func RepairInterruptedSession(ctx context.Context, messages message.Service, sessionID string) error {
	stored, err := messages.List(ctx, sessionID)
	if err != nil {
		return err
	}
	resulted := storedToolResultIDs(stored)
	var broken []*message.Message
	for i := range stored {
		msg := &stored[i]
		if msg.Role != message.Assistant {
			continue
		}
		if msg.IsFinished() && !hasUnresolvedToolCall(msg, resulted) {
			continue
		}
		broken = append(broken, msg)
	}
	if len(broken) == 0 {
		return nil
	}
	for _, msg := range broken {
		if msg.IsFinished() {
			continue
		}
		msg.AddFinish(message.FinishReasonError, interruptedTurnTitle, interruptedTurnDetails)
		if err := messages.Update(ctx, *msg); err != nil {
			return err
		}
	}
	// The finish parts are already in place, so unansweredToolCallContent
	// reports the generic text for the messages repaired above and keeps
	// the "never dispatched" text for a step that really did hit the
	// output token limit.
	_, err = finalizeUnresolvedToolCalls(ctx, messages, broken, unansweredToolCallContent)
	return err
}

// storedToolResultIDs collects the tool call IDs that already have a
// stored result.
func storedToolResultIDs(stored []message.Message) map[string]struct{} {
	resulted := make(map[string]struct{})
	for _, m := range stored {
		if m.Role != message.Tool {
			continue
		}
		for _, tr := range m.ToolResults() {
			resulted[tr.ToolCallID] = struct{}{}
		}
	}
	return resulted
}

// hasUnresolvedToolCall reports whether msg makes a tool call that never
// finished streaming its input or never produced a result.
func hasUnresolvedToolCall(msg *message.Message, resulted map[string]struct{}) bool {
	for _, tc := range msg.ToolCalls() {
		if !tc.Finished {
			return true
		}
		if _, ok := resulted[tc.ID]; !ok {
			return true
		}
	}
	return false
}

// finalizeUnresolvedToolCalls writes a terminal tool result for every
// tool call across msgs that never produced one, so the stored
// transcript cannot leave a tool call rendering as running forever. A
// tool call whose input never finished streaming is marked finished
// with an empty input first, for the same reason. contentFor supplies
// the result text for a given message, which lets a caller describe
// why that step's calls went unanswered. It returns how many results
// it wrote.
//
// Every assistant message of the turn has to be checked, not just the
// last one: fantasy creates one per step and discards the error from
// the OnToolResult callback, so a result row that failed to write
// leaves an unanswered call on a step the turn has already moved past.
//
// ctx must be detached from the run context and bounded: every in-turn
// caller reaches this after the run context is already cancelled or
// about to be, and these writes still have to land.
func finalizeUnresolvedToolCalls(ctx context.Context, messages message.Service, msgs []*message.Message, contentFor func(*message.Message) string) (int, error) {
	anyToolCalls := false
	for _, msg := range msgs {
		if len(msg.ToolCalls()) > 0 {
			anyToolCalls = true
			break
		}
	}
	if !anyToolCalls {
		return 0, nil
	}
	stored, err := messages.List(ctx, msgs[0].SessionID)
	if err != nil {
		return 0, err
	}
	resulted := storedToolResultIDs(stored)
	written := 0
	for _, msg := range msgs {
		toolCalls := msg.ToolCalls()
		if len(toolCalls) == 0 {
			continue
		}
		content := contentFor(msg)
		for _, tc := range toolCalls {
			if !tc.Finished {
				tc.Finished = true
				tc.Input = "{}"
				msg.AddToolCall(tc)
				if err := messages.Update(ctx, *msg); err != nil {
					return written, err
				}
			}
			if _, ok := resulted[tc.ID]; ok {
				continue
			}
			if _, err := messages.Create(ctx, msg.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					message.ToolResult{
						ToolCallID: tc.ID,
						Name:       tc.Name,
						Content:    content,
						IsError:    true,
					},
				},
			}); err != nil {
				return written, err
			}
			written++
		}
	}
	return written, nil
}

// unansweredToolCallContent is the tool result text stored for a call
// that a turn ended without answering. Only a step that hit the output
// token limit proves the call was never dispatched; any other cause
// means the tool may well have run, and telling the model it did not
// invites it to repeat a side effect.
func unansweredToolCallContent(msg *message.Message) string {
	if finish := msg.FinishPart(); finish != nil && finish.Reason == message.FinishReasonMaxTokens {
		return "The tool call was never run: the model's response hit the output token limit before the call could be dispatched."
	}
	return "No result was recorded for this tool call."
}
