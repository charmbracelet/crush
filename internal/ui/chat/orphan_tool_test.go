package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// A tool call whose process died leaves an assistant message with no finish
// part at all, so nothing marks it cancelled and it reports Spinning
// forever. The scrambling spinner then chews up the tool's own arguments.
func TestUnfinishedToolCallSpinsUntilRetired(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID: "call-1", Name: "write", Input: `{"file_path":"/tmp/x.go"}`,
	}, nil, false, "/tmp")

	animatable, ok := item.(Animatable)
	require.True(t, ok)
	require.True(t, animatable.Spinning(), "an unfinished tool call spins")

	item.SetStatus(ToolStatusCanceled)
	require.False(t, animatable.Spinning(), "cancelling it stops the spinner")
}

// A tool call that produced a result is settled and must be left alone.
func TestFinishedToolCallDoesNotSpin(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID: "call-1", Name: "write", Finished: true,
	}, &message.ToolResult{ToolCallID: "call-1", Content: "ok"}, false, "/tmp")

	animatable, ok := item.(Animatable)
	require.True(t, ok)
	require.False(t, animatable.Spinning())
}
