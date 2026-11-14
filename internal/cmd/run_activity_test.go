package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/format"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestRunStreamActivityFilteringAndTerminalReconciliation(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	presenter := format.NewActivityPresenter(t.Context(), func() {}, format.ActivityPresenterOptions{
		Output: &stderr,
		Width:  100,
	})
	t.Cleanup(presenter.Close)
	stream := &runStream{
		sessionID: "session",
		runID:     "mine",
		out:       &stdout,
		read:      make(map[string]int),
		activity:  presenter,
	}

	foreign := pubsub.Event[proto.RunActivity]{Payload: proto.RunActivity{
		SessionID: "session",
		RunID:     "other",
		MessageID: "foreign",
		Type:      proto.RunActivityThinkingStarted,
	}}
	done, err := stream.handle(foreign, nil)
	require.NoError(t, err)
	require.False(t, done)
	require.Empty(t, stderr.String())

	thinkingStarted := proto.RunActivity{
		SessionID: "session",
		RunID:     "mine",
		MessageID: "assistant",
		Type:      proto.RunActivityThinkingStarted,
	}
	done, err = stream.handle(pubsub.Event[proto.RunActivity]{Payload: thinkingStarted}, nil)
	require.NoError(t, err)
	require.False(t, done)
	require.Equal(t, 1, strings.Count(stderr.String(), "✦ Thinking…\n"))

	toolFinished := proto.RunActivity{
		SessionID: "session",
		RunID:     "mine",
		MessageID: "assistant",
		Type:      proto.RunActivityToolFinished,
		ToolCall: &proto.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"main.go"}`,
			Finished: true,
		},
		ToolResult: &proto.ToolResult{ToolCallID: "call", Name: "view"},
	}
	toolStarted := toolFinished
	toolStarted.Type = proto.RunActivityToolStarted
	toolStarted.ToolResult = nil
	done, err = stream.handle(pubsub.Event[proto.RunActivity]{Payload: toolStarted}, nil)
	require.NoError(t, err)
	require.False(t, done)
	require.Equal(t, 1, strings.Count(stderr.String(), "View"))
	done, err = stream.handle(pubsub.Event[proto.RunActivity]{Payload: toolFinished}, nil)
	require.NoError(t, err)
	require.False(t, done)
	require.Equal(t, 2, strings.Count(stderr.String(), "View"))

	thinkingFinished := thinkingStarted
	thinkingFinished.Type = proto.RunActivityThinkingFinished
	thinkingFinished.ThinkingDuration = 2 * time.Second
	done, err = stream.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "session",
		RunID:     "mine",
		MessageID: "assistant",
		Text:      "final response",
		Activities: []proto.RunActivity{
			thinkingFinished,
			toolFinished,
		},
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "final response", stdout.String())
	require.Equal(t, 1, strings.Count(stderr.String(), "✦ Thought for 2s\n"))
	require.Equal(t, 2, strings.Count(stderr.String(), "View"))
	require.NotContains(t, stdout.String(), "Thinking")
	require.NotContains(t, stdout.String(), "View")
}
