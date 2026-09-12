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

// Killing the process after the arguments finished streaming but before the
// result came back leaves a call that never spins, so the spinner check alone
// misses it and it sits on "Waiting for tool response..." forever.
func TestFinishedToolCallWithoutResultIsUnresolved(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID: "call-1", Name: "bash", Input: `{"command":"sleep 30"}`, Finished: true,
	}, nil, false, "/tmp")

	animatable, ok := item.(Animatable)
	require.True(t, ok)
	require.False(t, animatable.Spinning(), "a fully streamed call does not spin")
	require.True(t, item.Unresolved(), "but it still has no result")

	item.SetStatus(ToolStatusCanceled)
	require.False(t, item.Unresolved())
}

// Unresolved covers the half-streamed case too, and leaves settled calls be.
func TestUnresolvedToolCallStates(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	newItem := func(finished bool, result *message.ToolResult) ToolMessageItem {
		return NewToolMessageItem(&sty, "msg-1", message.ToolCall{
			ID: "call-1", Name: "write", Finished: finished,
		}, result, false, "/tmp")
	}

	require.True(t, newItem(false, nil).Unresolved(), "half-streamed call")
	require.False(t,
		newItem(true, &message.ToolResult{ToolCallID: "call-1", Content: "ok"}).Unresolved(),
		"successful call")
	require.False(t,
		newItem(true, &message.ToolResult{ToolCallID: "call-1", IsError: true}).Unresolved(),
		"errored call")
}
