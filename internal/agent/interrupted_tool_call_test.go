package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// killedTurn seeds the shape a process killed mid-tool-call leaves behind: an
// assistant message holding a tool call, no tool result anywhere, and no
// finish part. finished says whether the call's arguments had finished
// arriving before the process died.
func killedTurn(t *testing.T, env fakeEnv, finished bool) (session.Session, message.Message) {
	t.Helper()
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "killed")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "run the thing"}},
	})
	require.NoError(t, err)

	input := `{"command":"sleep 30"}`
	if !finished {
		input = ""
	}
	assistant, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "call_killed",
				Name:     "bash",
				Input:    input,
				Finished: finished,
			},
		},
	})
	require.NoError(t, err)
	return sess, assistant
}

func TestRepairInterruptedToolCalls_WritesTheRepair(t *testing.T) {
	env := testEnv(t)
	agent := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, assistant := killedTurn(t, env, false)

	msgs, err := agent.getSessionMessages(ctx, sess)
	require.NoError(t, err)

	// The repair is durable, not just applied to the slice we were handed.
	stored, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	var result *message.ToolResult
	for _, msg := range stored {
		for _, tr := range msg.ToolResults() {
			if tr.ToolCallID == "call_killed" {
				tr := tr
				result = &tr
			}
		}
	}
	require.NotNil(t, result, "the orphaned call must be given a stored result")
	require.True(t, result.IsError)
	require.Equal(t, interruptedToolResult, result.Content)

	reloaded, err := env.messages.Get(ctx, assistant.ID)
	require.NoError(t, err)
	calls := reloaded.ToolCalls()
	require.Len(t, calls, 1)
	require.True(t, calls[0].Finished, "a call that never got its arguments is settled")
	require.Equal(t, "{}", calls[0].Input, "empty input becomes {} so no provider has to guess")
	require.True(t, reloaded.IsFinished(), "the turn is marked finished")

	// The returned slice carries the repair too, so the turn about to be
	// built sees a complete transcript without re-reading.
	var found bool
	for _, msg := range msgs {
		for _, tr := range msg.ToolResults() {
			found = found || tr.ToolCallID == "call_killed"
		}
	}
	require.True(t, found, "the repaired result must be in the returned messages")
}

// The finish part records when the turn actually stopped. Stamping it with
// now would tell the response-time statistics that a turn killed months ago
// took months to answer.
func TestRepairInterruptedToolCalls_BackdatesTheFinish(t *testing.T) {
	env := testEnv(t)
	agent := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, assistant := killedTurn(t, env, true)

	before, err := env.messages.Get(ctx, assistant.ID)
	require.NoError(t, err)

	_, err = agent.getSessionMessages(ctx, sess)
	require.NoError(t, err)

	reloaded, err := env.messages.Get(ctx, assistant.ID)
	require.NoError(t, err)
	finish := reloaded.FinishPart()
	require.NotNil(t, finish)
	require.Equal(t, before.UpdatedAt, finish.Time, "the finish is stamped with when the turn stopped")
}

// A turn can end cleanly and still lose a tool result, so a present finish
// part must not be taken as proof that nothing is dangling.
func TestRepairInterruptedToolCalls_RepairsFinishedTurn(t *testing.T) {
	env := testEnv(t)
	agent := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "finished-but-orphaned")
	require.NoError(t, err)

	assistant, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call_a", Name: "bash", Input: "{}", Finished: true},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1000},
		},
	})
	require.NoError(t, err)

	_, err = agent.getSessionMessages(ctx, sess)
	require.NoError(t, err)

	stored, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)
	var found bool
	for _, msg := range stored {
		for _, tr := range msg.ToolResults() {
			found = found || tr.ToolCallID == "call_a"
		}
	}
	require.True(t, found, "an orphan on a finished turn is still repaired")

	reloaded, err := env.messages.Get(ctx, assistant.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), reloaded.FinishPart().Time, "an existing finish is left alone")
}

func TestRepairInterruptedToolCalls_LeavesSettledCallsAlone(t *testing.T) {
	env := testEnv(t)
	agent := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "healthy")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call_ok", Name: "bash", Input: `{"command":"ls"}`, Finished: true},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1000},
		},
	})
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.Tool,
		Parts: []message.ContentPart{message.ToolResult{ToolCallID: "call_ok", Name: "bash", Content: "ok"}},
	})
	require.NoError(t, err)

	before, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	msgs, err := agent.getSessionMessages(ctx, sess)
	require.NoError(t, err)
	require.Len(t, msgs, len(before), "a healthy session gains no rows")

	after, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, after, len(before))
}

// Repairing twice must not stack up results, since every turn re-reads the
// session.
func TestRepairInterruptedToolCalls_IsIdempotent(t *testing.T) {
	env := testEnv(t)
	agent := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, _ := killedTurn(t, env, true)

	_, err := agent.getSessionMessages(ctx, sess)
	require.NoError(t, err)
	first, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	_, err = agent.getSessionMessages(ctx, sess)
	require.NoError(t, err)
	second, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	require.Len(t, second, len(first), "a second pass adds nothing")
}

// Parallel tool calls die together, and each needs its own result or the
// provider rejects the whole message.
func TestRepairInterruptedToolCalls_HandlesParallelCalls(t *testing.T) {
	env := testEnv(t)
	agent := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "parallel")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call_1", Name: "bash", Input: `{"command":"ls"}`, Finished: true},
			message.ToolCall{ID: "call_2", Name: "view", Input: "", Finished: false},
			message.ToolCall{ID: "call_3", Name: "grep", Input: `{"pattern":"x"}`, Finished: true},
		},
	})
	require.NoError(t, err)
	// Only the middle call got an answer before the process died.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.Tool,
		Parts: []message.ContentPart{message.ToolResult{ToolCallID: "call_2", Name: "view", Content: "file"}},
	})
	require.NoError(t, err)

	_, err = agent.getSessionMessages(ctx, sess)
	require.NoError(t, err)

	stored, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)
	results := map[string]message.ToolResult{}
	for _, msg := range stored {
		for _, tr := range msg.ToolResults() {
			results[tr.ToolCallID] = tr
		}
	}
	require.Len(t, results, 3, "every call ends up with exactly one result")
	require.Equal(t, "file", results["call_2"].Content, "the real result is left alone")
	require.True(t, results["call_1"].IsError)
	require.True(t, results["call_3"].IsError)
}
