package agent

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// countingWriteService counts the writes a test provokes, so a test can
// prove a repair pass left an already-terminal transcript untouched.
type countingWriteService struct {
	message.Service
	creates atomic.Int64
	updates atomic.Int64
}

func (s *countingWriteService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	s.creates.Add(1)
	return s.Service.Create(ctx, sessionID, params)
}

func (s *countingWriteService) Update(ctx context.Context, msg message.Message) error {
	s.updates.Add(1)
	return s.Service.Update(ctx, msg)
}

// storeAssistantTurn writes one assistant message carrying the given
// tool calls and returns its ID.
func storeAssistantTurn(t *testing.T, env fakeEnv, sessionID string, calls ...message.ToolCall) string {
	t.Helper()
	parts := []message.ContentPart{message.TextContent{Text: "working"}}
	for _, tc := range calls {
		parts = append(parts, tc)
	}
	msg, err := env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: parts,
	})
	require.NoError(t, err)
	return msg.ID
}

func findMessage(t *testing.T, msgs []message.Message, id string) *message.Message {
	t.Helper()
	for i := range msgs {
		if msgs[i].ID == id {
			return &msgs[i]
		}
	}
	t.Fatalf("message %s not found", id)
	return nil
}

// TestRepairInterruptedSession_FinishesAbandonedTurn pins the fix for a
// transcript left behind by a run that died: the assistant message has
// no finish part and its tool call has no result, which the chat renders
// as a spinner and a permanent "Waiting for tool response...".
func TestRepairInterruptedSession_FinishesAbandonedTurn(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "frozen")
	require.NoError(t, err)
	msgID := storeAssistantTurn(t, env, sess.ID, message.ToolCall{
		ID:       "call-1",
		Name:     "bash",
		Input:    `{"command":"echo hi"}`,
		Finished: true,
	})

	require.NoError(t, RepairInterruptedSession(t.Context(), env.messages, sess.ID))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	assistant := findMessage(t, msgs, msgID)
	require.True(t, assistant.IsFinished(), "the assistant message must stop spinning")
	finish := assistant.FinishPart()
	require.NotNil(t, finish)
	require.Equal(t, message.FinishReasonError, finish.Reason)
	require.Equal(t, interruptedTurnTitle, finish.Message)

	results := map[string]message.ToolResult{}
	for _, m := range msgs {
		for _, tr := range m.ToolResults() {
			results[tr.ToolCallID] = tr
		}
	}
	result, ok := results["call-1"]
	require.True(t, ok, "the unanswered tool call must get a result")
	require.True(t, result.IsError)
	require.Equal(t, "No result was recorded for this tool call.", result.Content,
		"the repair must not claim a tool that may have run never ran")
}

// TestRepairInterruptedSession_FinishesUnstreamedToolCall covers the
// call whose input never finished streaming: it renders a live spinner,
// not just a waiting label.
func TestRepairInterruptedSession_FinishesUnstreamedToolCall(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "frozen")
	require.NoError(t, err)
	msgID := storeAssistantTurn(t, env, sess.ID, message.ToolCall{
		ID:       "call-1",
		Name:     "bash",
		Finished: false,
	})

	require.NoError(t, RepairInterruptedSession(t.Context(), env.messages, sess.ID))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	calls := findMessage(t, msgs, msgID).ToolCalls()
	require.Len(t, calls, 1)
	require.True(t, calls[0].Finished, "an unfinished tool call spins forever")
	require.Equal(t, "{}", calls[0].Input)
}

// TestRepairInterruptedSession_KeepsMaxTokensVerdict checks that a step
// whose finish part already proves the call was never dispatched keeps
// both its reason and the stronger result text.
func TestRepairInterruptedSession_KeepsMaxTokensVerdict(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "truncated")
	require.NoError(t, err)
	msgID := storeAssistantTurn(t, env, sess.ID, message.ToolCall{
		ID:       "call-1",
		Name:     "bash",
		Finished: true,
	})
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	truncated := findMessage(t, msgs, msgID)
	truncated.AddFinish(message.FinishReasonMaxTokens, "Max tokens", "")
	require.NoError(t, env.messages.Update(t.Context(), *truncated))

	require.NoError(t, RepairInterruptedSession(t.Context(), env.messages, sess.ID))

	msgs, err = env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, message.FinishReasonMaxTokens, findMessage(t, msgs, msgID).FinishReason(),
		"an existing finish reason must survive the repair")
	var content string
	for _, m := range msgs {
		for _, tr := range m.ToolResults() {
			if tr.ToolCallID == "call-1" {
				content = tr.Content
			}
		}
	}
	require.Contains(t, content, "never run")
}

// TestRepairInterruptedSession_LeavesTerminalTranscriptAlone keeps the
// repair off the hot path: it runs on every session load, so a healthy
// transcript must cost reads only.
func TestRepairInterruptedSession_LeavesTerminalTranscriptAlone(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "healthy")
	require.NoError(t, err)
	msgID := storeAssistantTurn(t, env, sess.ID, message.ToolCall{
		ID:       "call-1",
		Name:     "bash",
		Finished: true,
	})
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	done := findMessage(t, msgs, msgID)
	done.AddFinish(message.FinishReasonEndTurn, "", "")
	require.NoError(t, env.messages.Update(t.Context(), *done))
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call-1", Name: "bash", Content: "hi"},
		},
	})
	require.NoError(t, err)

	counter := &countingWriteService{Service: env.messages}
	require.NoError(t, RepairInterruptedSession(t.Context(), counter, sess.ID))

	require.Zero(t, counter.creates.Load(), "a terminal transcript needs no writes")
	require.Zero(t, counter.updates.Load(), "a terminal transcript needs no writes")
}

// TestRepairInterruptedSession_IgnoresUserMessages covers a session
// whose run died before it wrote an assistant row: only assistant
// messages carry the state a spinner is derived from, so nothing there
// may be rewritten.
func TestRepairInterruptedSession_IgnoresUserMessages(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "prompt only")
	require.NoError(t, err)
	user, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hi"}},
	})
	require.NoError(t, err)

	counter := &countingWriteService{Service: env.messages}
	require.NoError(t, RepairInterruptedSession(t.Context(), counter, sess.ID))

	require.Zero(t, counter.creates.Load())
	require.Zero(t, counter.updates.Load())
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	stored := findMessage(t, msgs, user.ID)
	require.Equal(t, message.FinishReason("stop"), stored.FinishReason(),
		"a user message keeps the finish part message.Service gave it")
}
