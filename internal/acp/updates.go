package acp

import (
	"github.com/charmbracelet/crush/internal/message"
	"github.com/coder/acp-go-sdk"
	"iter"
	"log/slog"
)

type updateIterator struct {
	lastT map[message.MessageRole]int
	lastR int
}

func newUpdatesIterator() *updateIterator {
	i := &updateIterator{}
	i.reset()
	return i
}

func (it *updateIterator) next(msg *message.Message) iter.Seq[acp.SessionUpdate] {
	return func(yield func(acp.SessionUpdate) bool) {
		for _, p := range msg.Parts {
			if n := it.getUpdate(msg.Role, p); n != (acp.SessionUpdate{}) && !yield(n) {
				return
			}
		}
	}
}

// FIXME: Add support for different types of content (image, audio and etc)
func (it *updateIterator) getContentBlock(role message.MessageRole, part message.ContentPart) (result acp.ContentBlock) {
	switch v := part.(type) {
	case message.TextContent:
		lastLen := it.lastT[role]
		nextLen := len(v.Text)
		if nextLen <= lastLen {
			return
		}
		delta := v.Text[lastLen:]
		it.lastT[role] = nextLen
		if delta != "" {
			return acp.ContentBlock{
				Text: &acp.ContentBlockText{
					Text: delta,
				},
			}
		}

	case message.ReasoningContent:
		if len(v.Thinking) <= it.lastR {
			return
		}

		delta := v.Thinking[it.lastR:]
		it.lastR = len(v.Thinking)
		if delta != "" {
			return acp.ContentBlock{
				Text: &acp.ContentBlockText{
					Text: delta,
				},
			}
		}

	case message.BinaryContent:
	case message.ImageURLContent:
	case message.Finish:
		it.reset()
	}

	return
}

func (it *updateIterator) reset() {
	it.lastT = make(map[message.MessageRole]int)
	it.lastR = 0
}

func (it *updateIterator) getUpdate(role message.MessageRole, part message.ContentPart) (result acp.SessionUpdate) {
	content := it.getContentBlock(role, part)
	hasContent := content != (acp.ContentBlock{})

	switch t := part.(type) {
	case message.ToolCall:
		{
			slog.Info("ToolCall", "t", t)
			tool := ToolCall(t)
			if !t.Finished {
				return acp.SessionUpdate{ToolCall: tool.StartToolCall()}
			}
			return acp.SessionUpdate{ToolCallUpdate: tool.UpdateToolCall()}
		}
	case message.ToolResult:
		{
			slog.Info("ToolResult")
			status := acp.ToolCallStatusCompleted
			if t.IsError {
				status = acp.ToolCallStatusFailed
			}

			// FIXME: refactor it in the same way as ToolCall
			// TODO: add support for images?
			return acp.UpdateToolCall(
				acp.ToolCallId(t.ToolCallID),
				acp.WithUpdateStatus(status),
				acp.WithUpdateContent([]acp.ToolCallContent{
					acp.ToolContent(acp.ContentBlock{
						Text: &acp.ContentBlockText{
							Text: t.Content,
							Meta: t.Metadata,
						},
					}),
				}),
			)
		}
	case message.ReasoningContent:
		if hasContent {
			return acp.UpdateAgentThought(content)
		}
	default:
		{
			switch role {
			case message.Assistant:
				if hasContent {
					return acp.UpdateAgentMessage(content)
				}
			case message.User:
				if hasContent {
					return acp.UpdateUserMessage(content)
				}
			}
		}
	}

	return
}
