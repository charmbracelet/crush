package acp

import (
	"github.com/charmbracelet/crush/internal/message"
	"github.com/coder/acp-go-sdk"
	"iter"
)

type updateIterator struct {
	lastT  map[message.MessageRole]int
	lastR  int
	prompt string
}

func newUpdatesIterator(prompt string) *updateIterator {
	return &updateIterator{
		lastT:  make(map[message.MessageRole]int),
		prompt: prompt,
	}
}

func (it *updateIterator) next(msg *message.Message) iter.Seq[*acp.SessionUpdate] {
	return func(yield func(*acp.SessionUpdate) bool) {
		for _, p := range msg.Parts {
			if n, ok := it.getUpdate(msg.Role, p); ok && !yield(n) {
				return
			}
		}
	}
}

// FIXME: Add support for different types of content (image, audio and etc)
func (it *updateIterator) getContentBlock(role message.MessageRole, part message.ContentPart) *acp.ContentBlock {
	switch v := part.(type) {
	case message.TextContent:
		lastLen := it.lastT[role]
		nextLen := len(v.Text)
		if nextLen <= lastLen {
			return nil
		}
		delta := v.Text[lastLen:]
		it.lastT[role] = nextLen
		if delta != "" {
			return &acp.ContentBlock{
				Text: &acp.ContentBlockText{
					Text: delta,
				},
			}
		}

	case message.ReasoningContent:
		if len(v.Thinking) <= it.lastR {
			return nil
		}

		delta := v.Thinking[it.lastR:]
		it.lastR = len(v.Thinking)
		if delta != "" {
			return &acp.ContentBlock{
				Text: &acp.ContentBlockText{
					Text: delta,
				},
			}
		}

	case message.BinaryContent:
	case message.ImageURLContent:
	case message.Finish:
	}

	return nil
}

func (it *updateIterator) getUpdate(role message.MessageRole, part message.ContentPart) (*acp.SessionUpdate, bool) {
	content := it.getContentBlock(role, part)
	if content == nil {
		return nil, false
	}

	switch role {
	case message.Assistant:
		switch part.(type) {
		case message.ReasoningContent:
			return &acp.SessionUpdate{
				AgentThoughtChunk: &acp.SessionUpdateAgentThoughtChunk{
					Content: *content,
				},
			}, true

		case message.TextContent:
			return &acp.SessionUpdate{
				AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
					Content: *content,
				},
			}, true
		}
	case message.User:
		if content.Text == nil || content.Text.Text != it.prompt {
			return &acp.SessionUpdate{
				UserMessageChunk: &acp.SessionUpdateUserMessageChunk{
					Content: *content,
				},
			}, true
		}

	case message.Tool:
		//kind = "tool_result_chunk"
	case message.System:
		//kind = "agent_message_chunk"
	}

	return nil, false
}
