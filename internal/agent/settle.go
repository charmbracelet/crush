package agent

import (
	"context"
	"errors"
	"log/slog"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/message"
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
				Error: errors.New("tool call was interrupted and did not produce a result, you may retry this call if the result is still needed"),
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

// closeUnfinishedToolCalls finishes unfinished tool calls and records an error
// result for each. A turn stopped at its output token limit, or stopped by a
// safety classifier, never reaches OnToolCall, so the call would otherwise
// animate forever with nothing left to cancel, and providers reject a tool
// call that has no reply.
func (a *sessionAgent) closeUnfinishedToolCalls(ctx context.Context, assistant *message.Message) error {
	var unfinished []message.ToolCall
	for _, tc := range assistant.ToolCalls() {
		if !tc.Finished {
			unfinished = append(unfinished, tc)
		}
	}
	if len(unfinished) == 0 {
		return nil
	}
	result := unfinishedToolCallResult(assistant)

	for _, tc := range unfinished {
		slog.Warn("Closing a tool call the model never finished",
			"session_id", assistant.SessionID,
			"tool_call_id", tc.ID,
			"tool_name", tc.Name)
		tc.Finished = true
		if tc.Input == "" {
			tc.Input = "{}"
		}
		assistant.AddToolCall(tc)
	}
	if err := a.messages.Update(ctx, *assistant); err != nil {
		return err
	}

	for _, tc := range unfinished {
		if _, err := a.messages.Create(ctx, assistant.SessionID, message.CreateMessageParams{
			Role: message.Tool,
			Parts: []message.ContentPart{message.ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    result,
				IsError:    true,
			}},
		}); err != nil {
			return err
		}
	}
	return nil
}
