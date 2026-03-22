package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestUpdateSessionMessageReinsertsAssistantAfterToolOnly(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	com := &common.Common{Styles: &theme}
	ui := &UI{
		com:  com,
		chat: NewChat(com),
	}

	assistantMsg := message.Message{
		ID:   "assistant-1",
		Role: message.Assistant,
	}
	ui.chat.AppendMessages(chat.NewAssistantMessageItem(ui.com.Styles, &assistantMsg))
	require.NotNil(t, ui.chat.MessageItem(assistantMsg.ID))

	// First update: assistant message becomes tool-only; UI removes the assistant item.
	assistantMsg.Parts = append(assistantMsg.Parts, message.ToolCall{
		ID:       "tool-1",
		Name:     "bash",
		Finished: true,
	})
	_ = ui.updateSessionMessage(assistantMsg)
	require.Nil(t, ui.chat.MessageItem(assistantMsg.ID))
	require.NotNil(t, ui.chat.MessageItem("tool-1"))

	// Second update: same assistant message gets text content; UI should re-insert it.
	assistantMsg.Parts = append(assistantMsg.Parts, message.TextContent{Text: "Hello"})
	_ = ui.updateSessionMessage(assistantMsg)

	require.NotNil(t, ui.chat.MessageItem(assistantMsg.ID))
	require.NotNil(t, ui.chat.MessageItem("tool-1"))
	require.Less(t, ui.chat.idInxMap[assistantMsg.ID], ui.chat.idInxMap["tool-1"])
}

func TestShouldRefreshSessionUsage(t *testing.T) {
	t.Parallel()

	ui := &UI{}
	msg := message.Message{
		ID:   "assistant-1",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "done"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 100},
		},
	}

	require.True(t, ui.shouldRefreshSessionUsage(pubsub.UpdatedEvent, msg))
	require.True(t, ui.shouldRefreshSessionUsage(pubsub.UpdatedEvent, msg))

	changed := msg
	changed.Parts = []message.ContentPart{
		message.TextContent{Text: "done!"},
		message.Finish{Reason: message.FinishReasonEndTurn, Time: 100},
	}
	require.True(t, ui.shouldRefreshSessionUsage(pubsub.UpdatedEvent, changed))
	require.False(t, ui.shouldRefreshSessionUsage(pubsub.CreatedEvent, changed))

	unfinished := message.Message{ID: "assistant-2", Role: message.Assistant}
	require.False(t, ui.shouldRefreshSessionUsage(pubsub.UpdatedEvent, unfinished))
}

func TestSetSessionMessagesSuppressesStaleLoadingStateForRestoredSession(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	com := &common.Common{Styles: &theme}
	ui := &UI{
		com:     com,
		chat:    NewChat(com),
		session: &session.Session{ID: "session-1"},
	}

	cmd := ui.setSessionMessages([]message.Message{
		{
			ID:   "assistant-1",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ReasoningContent{Thinking: "still thinking"},
			},
		},
		{
			ID:   "assistant-2",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{
					ID:       "tool-1",
					Name:     "bash",
					Input:    `{"command":"sleep 10"}`,
					Finished: false,
				},
			},
		},
	})
	_ = cmd

	assistantItem := ui.chat.MessageItem("assistant-1")
	require.NotNil(t, assistantItem)
	assistantRendered := ansi.Strip(assistantItem.Render(80))
	require.Contains(t, assistantRendered, "still thinking")
	require.NotContains(t, assistantRendered, "Thinking")

	toolItem := ui.chat.MessageItem("tool-1")
	require.NotNil(t, toolItem)
	toolRendered := ansi.Strip(toolItem.Render(80))
	require.Contains(t, toolRendered, "Bash")
	require.NotContains(t, toolRendered, "Waiting for tool response...")
}
