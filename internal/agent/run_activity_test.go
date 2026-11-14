package agent

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestRunActivityTrackerPublishesAndReconciles(t *testing.T) {
	t.Parallel()

	broker := pubsub.NewBroker[notify.RunActivity]()
	events := broker.Subscribe(t.Context())
	tracker := newRunActivityTracker(broker, SessionAgentCall{SessionID: "session", RunID: "run"})
	assistant := &message.Message{ID: "assistant", Parts: []message.ContentPart{
		message.ReasoningContent{Thinking: "private", StartedAt: 10, FinishedAt: 13},
	}}
	tracker.thinkingStarted(assistant)
	tracker.thinkingDone(assistant)
	tracker.thinkingDone(assistant)
	tracker.toolStarted(assistant.ID, message.ToolCall{ID: "call", Name: "view", Input: `{"file_path":"main.go"}`, Finished: true})
	tracker.toolFinished(assistant.ID, message.ToolResult{
		ToolCallID: "call",
		Name:       "view",
		Content:    "private result",
		Data:       "private data",
	}, false)
	tracker.toolFinished(assistant.ID, message.ToolResult{ToolCallID: "call", Name: "view"}, false)

	var activities []notify.RunActivity
	for len(activities) < 4 {
		select {
		case event := <-events:
			activities = append(activities, event.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("Timed out waiting for activity events")
		}
	}

	require.Equal(t, notify.RunActivityThinkingStarted, activities[0].Type)
	require.Equal(t, notify.RunActivityThinkingFinished, activities[1].Type)
	require.Equal(t, 3*time.Second, activities[1].ThinkingDuration)
	require.Equal(t, notify.RunActivityToolStarted, activities[2].Type)
	require.Equal(t, notify.RunActivityToolFinished, activities[3].Type)
	require.Equal(t, "view", activities[2].ToolCall.Name)
	require.Empty(t, activities[2].ToolResult.Content)
	require.Empty(t, activities[2].ToolResult.Data)
	for _, activity := range activities {
		require.Equal(t, "session", activity.SessionID)
		require.Equal(t, "run", activity.RunID)
	}

	summary := tracker.summary()
	require.Len(t, summary, 2)
	require.Equal(t, notify.RunActivityThinkingFinished, summary[0].Type)
	require.Equal(t, notify.RunActivityToolFinished, summary[1].Type)
}

func TestRunActivityTrackerRequiresRunID(t *testing.T) {
	t.Parallel()

	broker := pubsub.NewBroker[notify.RunActivity]()
	tracker := newRunActivityTracker(broker, SessionAgentCall{SessionID: "session"})
	tracker.toolFinished("assistant", message.ToolResult{ToolCallID: "call", Name: "view"}, false)
	require.Empty(t, tracker.summary())
}
