package model

import (
	"testing"

	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/planmode"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
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

func TestUpdateSessionMessageRemovesStaleToolItemsAfterRetryReset(t *testing.T) {
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
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "tool-1",
				Name:     "write",
				Input:    `{"file_path":"retry.txt","content":"before"}`,
				Finished: true,
			},
		},
	}
	_ = ui.updateSessionMessage(assistantMsg)
	require.NotNil(t, ui.chat.MessageItem("tool-1"))

	assistantMsg.Parts = nil
	_ = ui.updateSessionMessage(assistantMsg)

	require.Nil(t, ui.chat.MessageItem("tool-1"))
}

func TestDeletedAssistantMessageRemovesAssociatedToolItems(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	com := &common.Common{Styles: &theme}
	ui := &UI{
		com:     com,
		chat:    NewChat(com),
		session: &session.Session{ID: "session-1"},
	}

	toolCall := message.ToolCall{
		ID:       "tool-1",
		Name:     "write",
		Input:    `{"file_path":"retry.txt","content":"before"}`,
		Finished: true,
	}
	ui.chat.SetMessages(chat.NewToolMessageItem(ui.com.Styles, "assistant-1", toolCall, nil, false))
	require.NotNil(t, ui.chat.MessageItem("tool-1"))

	_, _ = ui.Update(pubsub.Event[message.Message]{
		Type: pubsub.DeletedEvent,
		Payload: message.Message{
			ID:        "assistant-1",
			SessionID: "session-1",
			Role:      message.Assistant,
			Parts:     []message.ContentPart{toolCall},
		},
	})

	require.Nil(t, ui.chat.MessageItem("tool-1"))
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

func TestHandleChildSessionMessageShowsAndClearsRetryStatus(t *testing.T) {
	t.Parallel()

	ui, parent, generalChild, _, _, _ := testSessionUI(t)
	ui.session = parent

	msgs, err := ui.com.App.Messages.List(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	toolCalls := msgs[0].ToolCalls()
	require.NotEmpty(t, toolCalls)
	ui.chat.SetMessages(chat.NewToolMessageItem(ui.com.Styles, msgs[0].ID, toolCalls[0], nil, false))

	_ = ui.handleChildSessionMessage(pubsub.Event[message.Message]{
		Type: pubsub.CreatedEvent,
		Payload: message.Message{
			ID:        "child-retry-1",
			SessionID: generalChild.ID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Service temporarily unavailable. Retrying in 3 seconds... (attempt 1/5)"},
			},
		},
	})

	rendered := ansi.Strip(ui.chat.MessageItem(toolCalls[0].ID).Render(100))
	require.Contains(t, rendered, "Retrying in 3 seconds")

	_ = ui.handleChildSessionMessage(pubsub.Event[message.Message]{
		Type: pubsub.CreatedEvent,
		Payload: message.Message{
			ID:        "child-assistant-2",
			SessionID: generalChild.ID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Final child answer"},
			},
		},
	})

	rendered = ansi.Strip(ui.chat.MessageItem(toolCalls[0].ID).Render(100))
	require.NotContains(t, rendered, "Retrying in 3 seconds")
}

func TestHandleChildSessionMessageClearsRetryStatusOnDelete(t *testing.T) {
	t.Parallel()

	ui, parent, generalChild, _, _, _ := testSessionUI(t)
	ui.session = parent

	msgs, err := ui.com.App.Messages.List(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	toolCalls := msgs[0].ToolCalls()
	require.NotEmpty(t, toolCalls)
	ui.chat.SetMessages(chat.NewToolMessageItem(ui.com.Styles, msgs[0].ID, toolCalls[0], nil, false))

	retryMsg := message.Message{
		ID:        "child-retry-1",
		SessionID: generalChild.ID,
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Service temporarily unavailable. Retrying in 3 seconds... (attempt 1/5)"},
		},
	}

	_ = ui.handleChildSessionMessage(pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: retryMsg,
	})

	rendered := ansi.Strip(ui.chat.MessageItem(toolCalls[0].ID).Render(100))
	require.Contains(t, rendered, "Retrying in 3 seconds")

	_ = ui.handleChildSessionMessage(pubsub.Event[message.Message]{
		Type:    pubsub.DeletedEvent,
		Payload: retryMsg,
	})

	rendered = ansi.Strip(ui.chat.MessageItem(toolCalls[0].ID).Render(100))
	require.NotContains(t, rendered, "Retrying in 3 seconds")
}

func TestMaybeOpenProposedPlanDialogRequiresPlanExit(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	com := &common.Common{Styles: &theme}
	ui := &UI{
		com:     com,
		dialog:  dialog.NewOverlay(),
		session: &session.Session{ID: "session-1", CollaborationMode: session.CollaborationModePlan},
	}

	msg := message.Message{
		ID:        "assistant-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: planmode.WrapProposedPlan("- Step 1")},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
	}

	require.Nil(t, ui.maybeOpenProposedPlanDialog(msg))
	require.False(t, ui.dialog.ContainsDialog(dialog.ProposedPlanID))

	msg.Parts = append(msg.Parts,
		message.ToolCall{ID: "tool-1", Name: agenttools.PlanExitToolName, Finished: true},
	)

	require.Nil(t, ui.maybeOpenProposedPlanDialog(msg))
	require.True(t, ui.dialog.ContainsDialog(dialog.ProposedPlanID))
}

func TestHandleChildSessionMessageRemovesStaleNestedToolsAfterRetryReset(t *testing.T) {
	t.Parallel()

	ui, parent, generalChild, _, _, _ := testSessionUI(t)
	ui.session = parent

	msgs, err := ui.com.App.Messages.List(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	toolCalls := msgs[0].ToolCalls()
	require.NotEmpty(t, toolCalls)
	ui.chat.SetMessages(chat.NewToolMessageItem(ui.com.Styles, msgs[0].ID, toolCalls[0], nil, false))

	childAssistant := message.Message{
		ID:        "child-assistant-1",
		SessionID: generalChild.ID,
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "child-write-1",
				Name:     "write",
				Input:    `{"file_path":"retry.txt","content":"before"}`,
				Finished: false,
			},
		},
	}

	_ = ui.handleChildSessionMessage(pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: childAssistant,
	})

	parentTool, ok := ui.chat.MessageItem(toolCalls[0].ID).(chat.NestedToolContainer)
	require.True(t, ok)
	require.Len(t, parentTool.NestedTools(), 1)

	childAssistant.Parts = nil
	_ = ui.handleChildSessionMessage(pubsub.Event[message.Message]{
		Type:    pubsub.UpdatedEvent,
		Payload: childAssistant,
	})

	parentTool, ok = ui.chat.MessageItem(toolCalls[0].ID).(chat.NestedToolContainer)
	require.True(t, ok)
	require.Empty(t, parentTool.NestedTools())
}

func TestUpdateLatestProposedPlanRequiresPlanModeAndPlanExit(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	com := &common.Common{Styles: &theme}
	ui := &UI{
		com:     com,
		session: &session.Session{ID: "session-1", CollaborationMode: session.CollaborationModeDefault},
	}

	planMsg := message.Message{
		ID:        "assistant-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: planmode.WrapProposedPlan("- Step 1")},
			message.ToolCall{ID: "tool-1", Name: agenttools.PlanExitToolName, Finished: true},
		},
	}

	ui.updateLatestProposedPlan(planMsg)
	require.Empty(t, ui.latestProposedPlan)

	ui.session.CollaborationMode = session.CollaborationModePlan
	planMsg.Parts = []message.ContentPart{message.TextContent{Text: planmode.WrapProposedPlan("- Step 1")}}
	ui.updateLatestProposedPlan(planMsg)
	require.Empty(t, ui.latestProposedPlan)

	planMsg.Parts = append(planMsg.Parts, message.ToolCall{ID: "tool-2", Name: agenttools.PlanExitToolName, Finished: true})
	ui.updateLatestProposedPlan(planMsg)
	require.Equal(t, "- Step 1", ui.latestProposedPlan)
}
