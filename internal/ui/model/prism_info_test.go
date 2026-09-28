package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// prismWorkspace is a minimal [workspace.Workspace] stub providing the
// methods the assistant-info rendering path needs.
type prismWorkspace struct {
	workspace.Workspace
}

func (w *prismWorkspace) Config() *config.Config {
	return &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
}
func (w *prismWorkspace) WorkingDir() string { return "/" }

func newPrismTestUI() *UI {
	com := common.DefaultCommon(&prismWorkspace{})
	return &UI{
		com:     com,
		status:  NewStatus(com, nil),
		chat:    NewChat(com, config.ScrollbarDefault),
		state:   uiChat,
		focus:   uiFocusEditor,
		width:   140,
		height:  45,
		session: &session.Session{ID: "s1"},
		keyMap:  DefaultKeyMap(),
		dialog:  dialog.NewOverlay(),
	}
}

func prismToolTurn(finished bool, prismName string) message.Message {
	parts := []message.ContentPart{
		message.ToolCall{ID: "tc1", Name: "bash", Input: "{}", Finished: true},
	}
	if finished {
		parts = append(parts, message.Finish{Reason: message.FinishReasonToolUse, Time: 1735689600})
	}
	return message.Message{
		ID:             "a1",
		SessionID:      "s1",
		Role:           message.Assistant,
		Model:          "prism-model",
		Provider:       "hyper",
		Parts:          parts,
		PrismModelName: prismName,
	}
}

// TestUpdateSessionMessage_PrismInfoBetweenTurns reproduces the between-turn
// flow: an assistant message streams without a finish part, then the turn
// finishes as a tool-use turn with a Prism-routed model name. The info item
// must appear once the routed name is available.
func TestUpdateSessionMessage_PrismInfoBetweenTurns(t *testing.T) {
	m := newPrismTestUI()

	// Turn streams: no finish part, no prism info item yet.
	_ = m.appendSessionMessage(prismToolTurn(false, ""))
	require.Nil(t, m.chat.MessageItem(chat.AssistantInfoID("a1")))

	// An update mid-stream changes nothing.
	_ = m.updateSessionMessage(prismToolTurn(false, ""))
	require.Nil(t, m.chat.MessageItem(chat.AssistantInfoID("a1")))

	// Turn finishes as a tool-use turn with a routed model name: the
	// info item must be appended.
	_ = m.updateSessionMessage(prismToolTurn(true, "GLM 5.3"))
	require.NotNil(t, m.chat.MessageItem(chat.AssistantInfoID("a1")), "expected assistant info item between turns")

	// A later update for the same turn must not remove it.
	_ = m.updateSessionMessage(prismToolTurn(true, "GLM 5.3"))
	require.NotNil(t, m.chat.MessageItem(chat.AssistantInfoID("a1")))
}

// TestUpdateSessionMessage_NoPrismInfoWithoutName ensures intermediate turns
// without a Prism-routed model name do not get an info item.
func TestUpdateSessionMessage_NoPrismInfoWithoutName(t *testing.T) {
	m := newPrismTestUI()

	_ = m.appendSessionMessage(prismToolTurn(false, ""))
	_ = m.updateSessionMessage(prismToolTurn(true, ""))
	require.Nil(t, m.chat.MessageItem(chat.AssistantInfoID("a1")))
}

// TestUpdateSessionMessage_PrismInfoWhileStreaming verifies that the routed
// model shows while the turn is still streaming, and that the streaming
// item is rebuilt into the full footer when the turn finishes.
func TestUpdateSessionMessage_PrismInfoWhileStreaming(t *testing.T) {
	m := newPrismTestUI()

	// The routed model name is known from the router headers as soon as
	// the stream starts: the info item shows while the turn streams.
	_ = m.appendSessionMessage(prismToolTurn(false, "GLM 5.3"))
	item := m.chat.MessageItem(chat.AssistantInfoID("a1"))
	require.NotNil(t, item, "expected info item while streaming")
	infoItem, ok := item.(*chat.AssistantInfoItem)
	require.True(t, ok)
	require.False(t, infoItem.HasFinishedMessage())

	// When the turn finishes, the streaming item is rebuilt with the
	// full footer.
	_ = m.updateSessionMessage(prismToolTurn(true, "GLM 5.3"))
	item = m.chat.MessageItem(chat.AssistantInfoID("a1"))
	require.NotNil(t, item)
	infoItem, ok = item.(*chat.AssistantInfoItem)
	require.True(t, ok)
	require.True(t, infoItem.HasFinishedMessage())
}

// prismStreamingTurn builds an assistant turn that is still streaming:
// optional text content, no finish part.
func prismStreamingTurn(prismName, text string) message.Message {
	var parts []message.ContentPart
	if text != "" {
		parts = append(parts, message.TextContent{Text: text})
	}
	return message.Message{
		ID:             "a1",
		SessionID:      "s1",
		Role:           message.Assistant,
		Model:          "prism",
		Provider:       "hyper",
		Parts:          parts,
		PrismModelName: prismName,
	}
}

// TestUpdateSessionMessage_PrismInfoRidesSpinner pins that while the
// assistant item is spinning (no content, no tool calls) the routed model
// info rides the spinner line and no standalone info item is rendered;
// the info item appears once content starts streaming.
func TestUpdateSessionMessage_PrismInfoRidesSpinner(t *testing.T) {
	m := newPrismTestUI()

	_ = m.appendSessionMessage(prismStreamingTurn("GLM 5.3", ""))
	require.Nil(t, m.chat.MessageItem(chat.AssistantInfoID("a1")), "info item must not show while spinning")

	_ = m.updateSessionMessage(prismStreamingTurn("GLM 5.3", ""))
	require.Nil(t, m.chat.MessageItem(chat.AssistantInfoID("a1")))

	// Content starts streaming: the info item appears (the spinner, and
	// with it the suffix, is gone).
	_ = m.updateSessionMessage(prismStreamingTurn("GLM 5.3", "hello"))
	require.NotNil(t, m.chat.MessageItem(chat.AssistantInfoID("a1")))
}
