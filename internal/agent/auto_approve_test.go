package agent

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// askPermission calls the permission service the way a tool would. The
// bounded context is what turns "nobody can answer this prompt" into a
// test failure instead of a stuck test binary.
func askPermission(t *testing.T, perms permission.Service, sessionID string, wait time.Duration) (bool, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	return perms.Request(ctx, permission.CreatePermissionRequest{
		SessionID:   sessionID,
		ToolCallID:  "call-" + sessionID,
		ToolName:    "write",
		Description: "write a file",
		Action:      "write",
		Path:        "/tmp/whatever",
	})
}

// newAskingTool returns a tool that asks for permission while the turn
// is running and reports the verdict it got, so a test can observe the
// hold from inside the turn rather than inferring it afterwards.
func newAskingTool(t *testing.T, perms permission.Service, sessionID string, verdicts chan<- bool) fantasy.AgentTool {
	t.Helper()
	return fantasy.NewAgentTool(
		"ask",
		"Asks for permission.",
		func(_ context.Context, _ echoToolParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			granted, err := askPermission(t, perms, sessionID, time.Second)
			verdicts <- granted && err == nil
			return fantasy.NewTextResponse("asked"), nil
		},
	)
}

// autoApproveAgent builds a session agent backed by a real permission
// service that prompts (skip=false), so only an auto-approval hold can
// make a request return.
func autoApproveAgent(t *testing.T, env fakeEnv, perms permission.Service, large fantasy.LanguageModel, tools ...fantasy.AgentTool) *sessionAgent {
	t.Helper()
	return NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: large, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		Sessions:    env.sessions,
		Messages:    env.messages,
		Tools:       tools,
		Permissions: perms,
	}).(*sessionAgent)
}

// gatedToolStreamModel alternates between a tool-calling step and a
// clean finish, so every turn it drives makes exactly one tool call. The
// first Stream call blocks until the gate is released, which is what
// holds a turn "active" long enough for a test to queue a prompt behind
// it.
type gatedToolStreamModel struct {
	gate       chan struct{}
	entered    chan struct{}
	toolCallID string
	toolName   string
	toolInput  string
	calls      atomic.Int64
}

func (m *gatedToolStreamModel) Provider() string { return "fake" }
func (m *gatedToolStreamModel) Model() string    { return "fake-model" }

func (m *gatedToolStreamModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "title"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *gatedToolStreamModel) Stream(ctx context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	n := m.calls.Add(1)
	if n == 1 {
		close(m.entered)
		select {
		case <-m.gate:
		case <-ctx.Done():
		}
	}
	if n%2 == 0 {
		return func(yield func(fantasy.StreamPart) bool) {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: "done"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text"}) {
				return
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
		}, nil
	}
	id := fmt.Sprintf("%s-%d", m.toolCallID, n)
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: m.toolName}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputDelta, ID: id, Delta: m.toolInput}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: id}) {
			return
		}
		if !yield(fantasy.StreamPart{
			Type:          fantasy.StreamPartTypeToolCall,
			ID:            id,
			ToolCallName:  m.toolName,
			ToolCallInput: m.toolInput,
		}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
	}, nil
}

func (m *gatedToolStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *gatedToolStreamModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// TestRun_AutoApproveHoldLastsExactlyTheTurn is the core of the TODO-8
// change: the approval belongs to the run, not to the client that asked
// for it. A tool call inside the turn must be granted without a prompt,
// and the very same request must go back to prompting once the turn is
// over, so nothing is left silently approved for the next client.
func TestRun_AutoApproveHoldLastsExactlyTheTurn(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	perms := permission.NewPermissionService(env.workingDir, false, nil)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	verdicts := make(chan bool, 1)
	tool := newAskingTool(t, perms, sess.ID, verdicts)
	model := &scriptedStreamModel{steps: []scriptedStep{
		{
			toolCallID:   "call-1",
			toolName:     "ask",
			toolInput:    `{"value":"hi"}`,
			finishReason: fantasy.FinishReasonToolCalls,
		},
		{text: "done", finishReason: fantasy.FinishReasonStop},
	}}
	sa := autoApproveAgent(t, env, perms, model, tool)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID:   sess.ID,
		Prompt:      "write a file",
		AutoApprove: true,
	})
	require.NoError(t, err)

	require.True(t, <-verdicts,
		"a tool call in an auto-approved turn must be granted without a prompt")

	granted, err := askPermission(t, perms, sess.ID, 200*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"the hold must end with the turn, not linger on the session")
	require.False(t, granted)
}

// TestRun_WithoutAutoApproveStillPrompts pins the other side: an
// ordinary turn must not be silently approved just because the agent can
// now take a hold.
func TestRun_WithoutAutoApproveStillPrompts(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	perms := permission.NewPermissionService(env.workingDir, false, nil)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	verdicts := make(chan bool, 1)
	tool := newAskingTool(t, perms, sess.ID, verdicts)
	model := &scriptedStreamModel{steps: []scriptedStep{
		{
			toolCallID:   "call-1",
			toolName:     "ask",
			toolInput:    `{"value":"hi"}`,
			finishReason: fantasy.FinishReasonToolCalls,
		},
		{text: "done", finishReason: fantasy.FinishReasonStop},
	}}
	sa := autoApproveAgent(t, env, perms, model, tool)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "write a file"})
	require.NoError(t, err)
	require.False(t, <-verdicts,
		"a turn that did not ask for auto-approval must still wait for a human")
}

// TestRun_QueuedPromptTakesItsOwnHold pins the queue behavior: a prompt
// parked behind a busy session must get its approval when its own turn
// runs. `crush run` against a busy session is exactly this case, and it
// has no way to take the hold itself.
func TestRun_QueuedPromptTakesItsOwnHold(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	perms := permission.NewPermissionService(env.workingDir, false, nil)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	verdicts := make(chan bool, 2)
	tool := newAskingTool(t, perms, sess.ID, verdicts)
	// The gated model blocks the first turn inside Stream, which is what
	// makes the session busy long enough to queue behind it. Both turns
	// then call the tool.
	large := &gatedToolStreamModel{
		gate:       make(chan struct{}),
		entered:    make(chan struct{}),
		toolName:   "ask",
		toolInput:  `{"value":"hi"}`,
		toolCallID: "call",
	}
	sa := autoApproveAgent(t, env, perms, large, tool)

	mainDone := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			RunID:     "run-main",
			Prompt:    "main",
		})
		mainDone <- runErr
	}()

	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main run never entered Stream")
	}
	require.True(t, sa.IsSessionBusy(sess.ID))

	// Queue the auto-approving prompt behind the busy turn. A RunID keeps
	// it out of the fold path, so it runs as its own turn.
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID:   sess.ID,
		RunID:       "run-follow",
		Prompt:      "follow",
		AutoApprove: true,
	})
	require.NoError(t, err)
	require.Nil(t, res)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	close(large.gate)
	require.NoError(t, <-mainDone)

	require.False(t, <-verdicts,
		"the active turn did not ask for auto-approval, so it must still prompt")
	require.True(t, <-verdicts,
		"the queued prompt must get its hold when its own turn runs")
}

// TestRun_QueuedPromptDoesNotInheritTheHold is the inverse: the active
// turn's approval must not extend to the prompt that runs after it. The
// queued turn starts from inside the active turn's frame (Run recurses
// into it), so a hold released only when the outer Run returns would
// silently approve a prompt that never asked — e.g. a TUI prompt queued
// behind an active `crush run`.
func TestRun_QueuedPromptDoesNotInheritTheHold(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	perms := permission.NewPermissionService(env.workingDir, false, nil)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	verdicts := make(chan bool, 2)
	tool := newAskingTool(t, perms, sess.ID, verdicts)
	large := &gatedToolStreamModel{
		gate:       make(chan struct{}),
		entered:    make(chan struct{}),
		toolName:   "ask",
		toolInput:  `{"value":"hi"}`,
		toolCallID: "call",
	}
	sa := autoApproveAgent(t, env, perms, large, tool)

	mainDone := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID:   sess.ID,
			RunID:       "run-main",
			Prompt:      "main",
			AutoApprove: true,
		})
		mainDone <- runErr
	}()

	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main run never entered Stream")
	}
	require.True(t, sa.IsSessionBusy(sess.ID))

	// Queue an ordinary prompt behind the auto-approved turn. A RunID
	// keeps it out of the fold path, so it runs as its own turn.
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-follow",
		Prompt:    "follow",
	})
	require.NoError(t, err)
	require.Nil(t, res)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	close(large.gate)
	require.NoError(t, <-mainDone)

	require.True(t, <-verdicts,
		"the active turn asked for auto-approval, so its tool call is granted")
	require.False(t, <-verdicts,
		"the queued prompt did not ask for auto-approval, so it must still prompt")
}

// TestRun_CancelledTurnReleasesTheHold covers the exit path that started
// this change: `crush run` can go away mid-turn (Ctrl-C, a dead stream,
// a killed client). The server-side run owns the hold, so however the
// turn ends it must give the approval back — a leaked hold would leave
// the session silently approved for whichever client keeps the
// workspace alive.
func TestRun_CancelledTurnReleasesTheHold(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	perms := permission.NewPermissionService(env.workingDir, false, nil)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	large := &gatedToolStreamModel{
		gate:       make(chan struct{}),
		entered:    make(chan struct{}),
		toolName:   "ask",
		toolInput:  `{"value":"hi"}`,
		toolCallID: "call",
	}
	sa := autoApproveAgent(t, env, perms, large)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = sa.Run(t.Context(), SessionAgentCall{
			SessionID:   sess.ID,
			Prompt:      "write a file",
			AutoApprove: true,
		})
	}()

	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("run never entered Stream")
	}

	sa.Cancel(sess.ID)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled run never returned")
	}

	granted, err := askPermission(t, perms, sess.ID, 200*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"a cancelled turn must not leave its session auto-approved")
	require.False(t, granted)
}
