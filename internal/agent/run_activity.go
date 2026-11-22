package agent

import (
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// runActivityTracker publishes display-only run transitions and retains their
// completed forms for RunComplete reconciliation.
type runActivityTracker struct {
	publisher pubsub.Publisher[notify.RunActivity]
	sessionID string
	runID     string

	mu               sync.Mutex
	completed        []notify.RunActivity
	toolCalls        map[string]message.ToolCall
	thinkingFinished map[string]struct{}
	toolsStarted     map[string]struct{}
	toolsFinished    map[string]struct{}
}

func newRunActivityTracker(publisher pubsub.Publisher[notify.RunActivity], call SessionAgentCall) *runActivityTracker {
	return &runActivityTracker{
		publisher:        publisher,
		sessionID:        call.SessionID,
		runID:            call.RunID,
		toolCalls:        make(map[string]message.ToolCall),
		thinkingFinished: make(map[string]struct{}),
		toolsStarted:     make(map[string]struct{}),
		toolsFinished:    make(map[string]struct{}),
	}
}

func (t *runActivityTracker) publish(activity notify.RunActivity) {
	if t.publisher == nil || t.runID == "" {
		return
	}
	activity.SessionID = t.sessionID
	activity.RunID = t.runID

	t.mu.Lock()
	if activity.Type == notify.RunActivityThinkingFinished || activity.Type == notify.RunActivityToolFinished {
		t.completed = append(t.completed, activity)
	}
	t.mu.Unlock()
	t.publisher.Publish(pubsub.UpdatedEvent, activity)
}

func (t *runActivityTracker) thinkingStarted(msg *message.Message) {
	if msg == nil || msg.ReasoningContent().StartedAt == 0 {
		return
	}
	t.publish(notify.RunActivity{MessageID: msg.ID, Type: notify.RunActivityThinkingStarted})
}

func (t *runActivityTracker) thinkingDone(msg *message.Message) {
	if msg == nil {
		return
	}
	reasoning := msg.ReasoningContent()
	if reasoning.StartedAt == 0 || reasoning.FinishedAt == 0 || reasoning.Thinking == "" {
		return
	}
	t.mu.Lock()
	_, done := t.thinkingFinished[msg.ID]
	if !done {
		t.thinkingFinished[msg.ID] = struct{}{}
	}
	t.mu.Unlock()
	if done {
		return
	}
	t.publish(notify.RunActivity{
		MessageID:        msg.ID,
		Type:             notify.RunActivityThinkingFinished,
		ThinkingDuration: msg.ThinkingDuration(),
	})
}

func (t *runActivityTracker) recordToolCall(toolCall message.ToolCall) {
	t.mu.Lock()
	t.toolCalls[toolCall.ID] = toolCall
	t.mu.Unlock()
}

func (t *runActivityTracker) toolStarted(messageID string, toolCall message.ToolCall) {
	if !toolCall.Finished {
		return
	}
	t.recordToolCall(toolCall)
	t.mu.Lock()
	_, started := t.toolsStarted[toolCall.ID]
	if !started {
		t.toolsStarted[toolCall.ID] = struct{}{}
	}
	t.mu.Unlock()
	if started {
		return
	}
	t.publish(notify.RunActivity{
		MessageID: messageID,
		Type:      notify.RunActivityToolStarted,
		ToolCall:  toolCall,
	})
}

func (t *runActivityTracker) toolFinished(messageID string, result message.ToolResult, cancelled bool) {
	t.mu.Lock()
	_, done := t.toolsFinished[result.ToolCallID]
	if !done {
		t.toolsFinished[result.ToolCallID] = struct{}{}
	}
	toolCall := t.toolCalls[result.ToolCallID]
	t.mu.Unlock()
	if done {
		return
	}

	if toolCall.ID == "" {
		toolCall.ID = result.ToolCallID
		toolCall.Name = result.Name
	}
	toolCall.Finished = true
	toolCallResult := message.ToolResult{
		ToolCallID: result.ToolCallID,
		Name:       result.Name,
		IsError:    result.IsError,
	}
	t.publish(notify.RunActivity{
		MessageID:  messageID,
		Type:       notify.RunActivityToolFinished,
		ToolCall:   toolCall,
		ToolResult: toolCallResult,
		Cancelled:  cancelled,
	})
}

func (t *runActivityTracker) summary() []notify.RunActivity {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.completed)
}
