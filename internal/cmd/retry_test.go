package cmd

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestRetryProgressDoesNotFinishHeadlessRun(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	stream := &runStream{sessionID: "session", runID: "run", out: &stdout, errOut: &stderr, read: make(map[string]int)}
	event := pubsub.Event[proto.AgentEvent]{Payload: proto.AgentEvent{Type: proto.AgentEventType("retry"), SessionID: "session", RunID: "run", RetryAttempt: 2, RetryMaxAttempts: 3, RetryDelayMS: 150}}
	done, err := stream.handle(event, nil)
	require.NoError(t, err)
	require.False(t, done)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "2/3")
	event.Payload.Done = true
	done, err = stream.handle(event, nil)
	require.NoError(t, err)
	require.False(t, done)
}
