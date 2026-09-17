package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/message"
)

// Copy recorded for work a dead process never finished. Both strings are
// written to the database by repairInterruptedToolCalls, so they are what
// the model reads on the next turn and what the transcript shows.
const (
	interruptedToolResult  = "tool call was interrupted and did not produce a result, you may retry this call if the result is still needed"
	interruptedTurnMessage = "Interrupted"
)

// toolResultsForCalls builds the tool message that must immediately follow
// an assistant message with tool calls. LLM APIs require every tool call to
// be followed by its results before any other message; strict-adjacency
// providers reject the request otherwise. Results are taken from
// toolResultsByCall and consumed, so a result stored in a message that also
// holds results for calls of other assistant messages is emitted exactly
// once, next to the assistant that requested it. Tool calls without any
// stored result (e.g. an interrupted session) receive a synthetic error
// response so the conversation keeps working.
func toolResultsForCalls(m message.Message, toolResultsByCall map[string][]fantasy.MessagePart) fantasy.Message {
	content := make([]fantasy.MessagePart, 0, len(m.ToolCalls()))
	for _, tc := range m.ToolCalls() {
		parts := toolResultsByCall[tc.ID]
		delete(toolResultsByCall, tc.ID)
		if len(parts) > 0 {
			content = append(content, parts...)
			continue
		}
		slog.Warn(
			"Injecting synthetic tool result for orphaned tool call",
			"tool_call_id", tc.ID,
			"tool_name", tc.Name,
		)
		content = append(content, fantasy.ToolResultPart{
			ToolCallID: tc.ID,
			Output: fantasy.ToolResultOutputContentError{
				Error: errors.New(interruptedToolResult),
			},
		})
	}
	return fantasy.Message{
		Role:    fantasy.MessageRoleTool,
		Content: content,
	}
}

// truncatedToolCallResult is recorded for a tool call the model stopped
// writing because it ran out of room.
const truncatedToolCallResult = "the model ran out of output tokens before it finished writing this tool call, so the call was never run; retry with smaller arguments, splitting the work across several calls if that helps"

// refusedToolCallResult is recorded for a tool call abandoned when the
// provider's safety classifier stopped the response.
const refusedToolCallResult = "the provider's safety classifier stopped this response before the tool call was finished, so the call was never run; rephrase the request or try a different model"

// abandonedToolCallResult is recorded for a tool call the turn left
// unfinished for a reason we cannot name.
const abandonedToolCallResult = "the turn ended before the model finished writing this tool call, so the call was never run; retry the call if the result is still needed"

// unfinishedToolCallResult reports what to say about a tool call its turn
// abandoned. Why the turn stopped decides what a second attempt should
// change, so a refusal must not be reported as an output limit.
func unfinishedToolCallResult(assistant *message.Message) string {
	finish := assistant.FinishPart()
	if finish == nil {
		return abandonedToolCallResult
	}
	switch finish.Reason {
	case message.FinishReasonMaxTokens:
		return truncatedToolCallResult
	case message.FinishReasonContentFilter:
		return refusedToolCallResult
	default:
		return abandonedToolCallResult
	}
}

// closeUnfinishedToolCalls settles tool calls the model never finished
// writing. A turn stopped at its output token limit, or stopped by a safety
// classifier, never reaches OnToolCall, so the call would otherwise animate
// forever with nothing left to cancel.
func (a *sessionAgent) closeUnfinishedToolCalls(ctx context.Context, assistant *message.Message) error {
	var unfinished []message.ToolCall
	for _, tc := range assistant.ToolCalls() {
		if !tc.Finished {
			unfinished = append(unfinished, tc)
		}
	}
	_, err := a.settleToolCalls(ctx, assistant, unfinished, unfinishedToolCallResult(assistant))
	return err
}

// settleToolCalls closes out tool calls that will never be answered, and
// records why.
//
// A tool call is stored in two parts: the request, and a separate reply
// written once the tool answers. Anything that stops a turn between the two
// leaves the request standing alone, which providers reject and every reader
// of the session has to know to treat as dead. Rather than leave that to
// each reader, the ending is written down: the request is marked complete
// and answered with reason.
//
// Whatever arguments did arrive are kept as they were written, however
// partial. The transcript is the readable record of what the model said;
// making them valid JSON is the send path's job.
//
// The created reply messages are returned in the order they were written.
func (a *sessionAgent) settleToolCalls(ctx context.Context, assistant *message.Message, calls []message.ToolCall, reason string) ([]message.Message, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	for _, tc := range calls {
		slog.Warn("Settling a tool call that will never be answered",
			"session_id", assistant.SessionID,
			"tool_call_id", tc.ID,
			"tool_name", tc.Name,
			"reason", reason)
		if !tc.Finished {
			tc.Finished = true
			assistant.AddToolCall(tc)
		}
	}
	if err := a.messages.Update(ctx, *assistant); err != nil {
		return nil, fmt.Errorf("failed to settle tool calls: %w", err)
	}

	settled := make([]message.Message, 0, len(calls))
	for _, tc := range calls {
		created, err := a.messages.Create(ctx, assistant.SessionID, message.CreateMessageParams{
			Role: message.Tool,
			Parts: []message.ContentPart{message.ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    reason,
				IsError:    true,
			}},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to record settled tool result: %w", err)
		}
		settled = append(settled, created)
	}
	return settled, nil
}

// repairInterruptedToolCalls settles tool calls that a dead process left
// dangling, writing the repair back rather than papering over it on every
// read.
//
// Nothing else covers this. The cleanup for Ctrl+C lives in the turn loop's
// error branch and the cleanup for a model that ran out of room lives at the
// end of the turn; a SIGKILL, a panic, or a lost terminal reaches neither.
// Left alone the call stays pending forever, so every reader has to know to
// treat it as dead, and each reader that forgets reports a tool as still
// running months later.
//
// This is safe to run unguarded because it is only reachable from a turn
// that has already claimed the session: both callers check IsSessionBusy
// first and refuse if another run holds it, so a call found without a reply
// here cannot belong to a run still in flight.
func (a *sessionAgent) repairInterruptedToolCalls(ctx context.Context, sessionID string, msgs []message.Message) ([]message.Message, error) {
	resolved := make(map[string]struct{})
	for _, msg := range msgs {
		for _, tr := range msg.ToolResults() {
			resolved[tr.ToolCallID] = struct{}{}
		}
	}

	var repaired []message.Message
	for i := range msgs {
		msg := &msgs[i]
		if msg.Role != message.Assistant {
			continue
		}
		var orphans []message.ToolCall
		for _, tc := range msg.ToolCalls() {
			if _, ok := resolved[tc.ID]; !ok {
				orphans = append(orphans, tc)
			}
		}
		if len(orphans) == 0 {
			continue
		}
		// A turn can end cleanly and still lose a reply, so the finish is
		// repaired only when genuinely absent rather than used as the
		// signal that anything is wrong. It is stamped with when the turn
		// stopped rather than with now, since response-time statistics
		// average over that field.
		if !msg.IsFinished() {
			msg.AddFinishAt(message.FinishReasonError, interruptedTurnMessage, "", msg.UpdatedAt)
		}
		settled, err := a.settleToolCalls(ctx, msg, orphans, interruptedToolResult)
		if err != nil {
			return nil, err
		}
		repaired = append(repaired, settled...)
	}
	// Replies pair with their request by ID rather than by position, so the
	// repaired rows can simply follow the transcript.
	return append(msgs, repaired...), nil
}
