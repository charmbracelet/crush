//go:build !windows

package format

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
)

func TestActivityPresenterLiveTTYStates(t *testing.T) {
	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()

	originalStderr := os.Stderr
	os.Stderr = slave
	defer func() {
		os.Stderr = originalStderr
		_ = slave.Close()
	}()

	var output spinnerBuffer
	readDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&output, struct{ io.Reader }{master})
		close(readDone)
	}()

	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{
		Output: slave,
		TTY:    true,
		Width:  100,
	})
	presenter.Handle(notify.RunActivity{MessageID: "assistant", Type: notify.RunActivityThinkingStarted})
	require.Eventuallyf(t, func() bool {
		return bytes.Contains([]byte(ansi.Strip(output.String())), []byte("✦ Thinking…"))
	}, 3*time.Second, 20*time.Millisecond, "%q", ansi.Strip(output.String()))

	presenter.Handle(notify.RunActivity{
		MessageID:        "assistant",
		Type:             notify.RunActivityThinkingFinished,
		ThinkingDuration: 2 * time.Second,
	})
	require.Eventually(t, func() bool {
		return bytes.Contains([]byte(ansi.Strip(output.String())), []byte("✦ Thought for 2s"))
	}, 3*time.Second, 20*time.Millisecond)

	presenter.Handle(notify.RunActivity{
		MessageID: "assistant",
		Type:      notify.RunActivityToolStarted,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"main.go"}`,
			Finished: true,
		},
	})
	require.Eventuallyf(t, func() bool {
		return bytes.Contains([]byte(ansi.Strip(output.String())), []byte("● View"))
	}, 3*time.Second, 20*time.Millisecond, "%q", ansi.Strip(output.String()))

	presenter.Handle(notify.RunActivity{
		MessageID: "assistant",
		Type:      notify.RunActivityToolFinished,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"main.go"}`,
			Finished: true,
		},
		ToolResult: message.ToolResult{ToolCallID: "call", Name: "view"},
	})
	presenter.Close()
	require.Eventually(t, func() bool {
		return bytes.Contains([]byte(ansi.Strip(output.String())), []byte("✓ View"))
	}, 3*time.Second, 20*time.Millisecond)
	require.NoError(t, slave.Close())
	select {
	case <-readDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out draining terminal output")
	}
}
