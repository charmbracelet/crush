package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestAssistantSpinnerSuffix pins that the routed model info rides the
// working spinner line while the turn is spinning, and disappears once
// content starts streaming (the info item takes over from there).
func TestAssistantSpinnerSuffix(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	// A message that is still generating with no content or tool calls
	// is the "working" state: it spins.
	msg := &message.Message{ID: "m1", SessionID: "s1", Role: message.Assistant}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	require.True(t, item.Spinning(), "an unfinished message with no content must spin")

	item.SetSpinnerSuffix("Prism → GLM 5.3")
	out := ansi.Strip(item.RawRender(80))
	require.Contains(t, out, "Prism → GLM 5.3")

	// Once content streams the item stops spinning; the suffix is no
	// longer part of the rendered output.
	msg.Parts = append(msg.Parts, message.TextContent{Text: "hello"})
	require.False(t, item.Spinning())
	item.SetMessage(msg)
	require.NotContains(t, ansi.Strip(item.RawRender(80)), "Prism → GLM 5.3")
}

// TestToolSpinnerSuffix pins that the routed model info rides a pending
// tool's spinner line.
func TestToolSpinnerSuffix(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	// A tool call that has not finished yet renders its pending spinner.
	toolCall := message.ToolCall{ID: "tc1", Name: "bash", Input: `{"command":"ls"}`, Finished: false}
	item := NewToolMessageItem(&sty, "m1", toolCall, nil, false, "/")
	animatable, ok := item.(Animatable)
	require.True(t, ok)
	require.True(t, animatable.Spinning())

	setter, ok := item.(interface{ SetSpinnerSuffix(string) })
	require.True(t, ok, "tool items must accept a spinner suffix")
	setter.SetSpinnerSuffix("Prism → GLM 5.3")
	require.Contains(t, ansi.Strip(item.RawRender(80)), "Prism → GLM 5.3")
}
