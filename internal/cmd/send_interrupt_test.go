package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// cancelRecordingSendClient fails the test's expectations if send ever
// reaches for a cancel: send hands work to a session and must not stop it.
type cancelRecordingSendClient struct {
	*fakeSendClient
	cancels int
}

func (c *cancelRecordingSendClient) CancelAgentSession(context.Context, string, string) error {
	c.cancels++
	return nil
}

// TestSendWaitInterruptOnlyStopsWaiting pins the ruling for `send --wait`:
// Ctrl+C stops waiting and leaves the run going on the server, and the
// error says so instead of a bare "context canceled".
func TestSendWaitInterruptOnlyStopsWaiting(t *testing.T) {
	t.Parallel()

	f := &cancelRecordingSendClient{fakeSendClient: newFakeSendClient(proto.Session{ID: "s1"})}
	ctx, interrupt := context.WithCancel(t.Context())
	f.onSend = func(string, string) { interrupt() }

	res, err := runSend(ctx, f, "ws", sendOptions{sessionID: "s1", prompt: "p", wait: true}, &bytes.Buffer{})
	require.EqualError(t, err, "stopped waiting; the run keeps going on session s1")
	require.Equal(t, "s1", res.SessionID)
	require.Zero(t, f.cancels, "interrupting send must not cancel the run")
}
