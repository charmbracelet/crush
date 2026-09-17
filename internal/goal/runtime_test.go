package goal

import (
	"context"
	"errors"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type runtimeStore struct {
	Service
	mu   sync.Mutex
	goal *Goal
}

func (s *runtimeStore) Get(context.Context, string) (*Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.goal == nil {
		return nil, nil
	}
	g := *s.goal
	return &g, nil
}

func (s *runtimeStore) UpdateStatus(ctx context.Context, sessionID, goalID string, status GoalStatus) (*Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.goal == nil || s.goal.GoalID != goalID {
		return nil, errors.New("stale goal")
	}
	s.goal.Status = status
	g := *s.goal
	return &g, nil
}

type runtimeAgent struct {
	err  error
	runs int
	run  func(context.Context) (*fantasy.AgentResult, error)
}

func (a *runtimeAgent) Run(ctx context.Context, _ string, _ string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	a.runs++
	if a.run != nil {
		return a.run(ctx)
	}
	return nil, a.err
}
func (*runtimeAgent) IsSessionBusy(string) bool { return false }
func (*runtimeAgent) QueuedPrompts(string) int  { return 0 }

func TestContinuationFailurePausesGoal(t *testing.T) {
	t.Parallel()
	for _, err := range []error{errors.New("provider unavailable"), context.Canceled, context.DeadlineExceeded} {
		t.Run(err.Error(), func(t *testing.T) {
			t.Parallel()
			store := &runtimeStore{goal: &Goal{SessionID: "session", GoalID: "goal", Status: GoalActive}}
			runner := &runtimeAgent{err: err}
			runtime := NewRuntime(store, runner, nil)
			require.ErrorIs(t, runtime.MaybeContinue(context.Background(), "session"), err)
			require.Equal(t, GoalPaused, store.goal.Status)
			require.NoError(t, runtime.MaybeContinue(context.Background(), "session"))
			require.Equal(t, 1, runner.runs)
		})
	}
}

func (s *runtimeStore) Clear(context.Context, string) (*Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.goal
	s.goal = nil
	return g, nil
}

func TestTurnStopped(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		result  *fantasy.AgentResult
		err     error
		stopped bool
	}{
		{name: "queued"},
		{name: "setup error", err: errors.New("model unavailable"), stopped: true},
		{name: "normal answer", result: &fantasy.AgentResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop}}},
		{name: "token limit", result: &fantasy.AgentResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonLength}}, stopped: true},
		{name: "refusal", result: &fantasy.AgentResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonContentFilter}}, stopped: true},
		{name: "loop guard", result: &fantasy.AgentResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop}, Steps: []fantasy.StepResult{{Response: fantasy.Response{FinishReason: fantasy.FinishReasonToolCalls}}}}, stopped: true},
		{name: "tool halt", result: &fantasy.AgentResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop, Content: fantasy.ResponseContent{fantasy.ToolResultContent{StopTurn: true}}}}, stopped: true},
		{name: "unknown stop", result: &fantasy.AgentResult{}, stopped: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.stopped, turnStopped(tt.result, tt.err))
		})
	}
}

func TestGoalPauseAndClearCancelRunningContinuation(t *testing.T) {
	t.Parallel()
	// startTurn launches a continuation that blocks until its context is cancelled.
	startTurn := func(t *testing.T) (*Runtime, *runtimeAgent, <-chan error) {
		store := &runtimeStore{goal: &Goal{SessionID: "session", GoalID: "goal", Status: GoalActive}}
		started := make(chan struct{})
		runner := &runtimeAgent{run: func(ctx context.Context) (*fantasy.AgentResult, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		runtime := NewRuntime(store, runner, nil)
		done := make(chan error, 1)
		go func() { done <- runtime.MaybeContinue(t.Context(), "session") }()
		<-started
		return runtime, runner, done
	}

	t.Run("pause", func(t *testing.T) {
		t.Parallel()
		runtime, runner, done := startTurn(t)
		_, err := runtime.Pause(t.Context(), "session")
		require.NoError(t, err)
		require.ErrorIs(t, <-done, context.Canceled)
		require.NoError(t, runtime.MaybeContinue(t.Context(), "session"))
		require.Equal(t, 1, runner.runs)
	})

	t.Run("clear", func(t *testing.T) {
		t.Parallel()
		runtime, runner, done := startTurn(t)
		_, err := runtime.Clear(t.Context(), "session")
		require.NoError(t, err)
		require.ErrorIs(t, <-done, context.Canceled)
		require.NoError(t, runtime.MaybeContinue(t.Context(), "session"))
		require.Equal(t, 1, runner.runs)
	})
}

func TestNormalAnswerContinuesUntilGoalComplete(t *testing.T) {
	t.Parallel()
	store := &runtimeStore{goal: &Goal{SessionID: "session", GoalID: "goal", Status: GoalActive}}
	turns := 0
	runner := &runtimeAgent{run: func(ctx context.Context) (*fantasy.AgentResult, error) {
		turns++
		if turns == 2 {
			_, err := store.UpdateStatus(ctx, "session", "goal", GoalComplete)
			require.NoError(t, err)
		}
		return &fantasy.AgentResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop}}, nil
	}}
	runtime := NewRuntime(store, runner, nil)
	require.NoError(t, runtime.MaybeContinue(t.Context(), "session"))
	require.Equal(t, 2, turns)
	_, err := runtime.Pause(t.Context(), "session")
	require.NoError(t, err)
	require.Equal(t, GoalComplete, store.goal.Status)
}

func TestAfterTurnPausesEvenWithCancelledContext(t *testing.T) {
	t.Parallel()
	store := &runtimeStore{goal: &Goal{SessionID: "session", GoalID: "goal", Status: GoalActive}}
	runtime := NewRuntime(store, &runtimeAgent{}, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runtime.AfterTurn(ctx, "session", nil, context.Canceled)
	require.Equal(t, GoalPaused, store.goal.Status)
}

func TestOldFailureDoesNotPauseReplacementGoal(t *testing.T) {
	t.Parallel()
	store := &runtimeStore{goal: &Goal{SessionID: "session", GoalID: "old", Status: GoalActive}}
	runner := &runtimeAgent{run: func(context.Context) (*fantasy.AgentResult, error) {
		store.mu.Lock()
		store.goal = &Goal{SessionID: "session", GoalID: "new", Status: GoalActive}
		store.mu.Unlock()
		return nil, errors.New("old turn failed")
	}}
	runtime := NewRuntime(store, runner, nil)
	require.Error(t, runtime.MaybeContinue(t.Context(), "session"))
	require.Equal(t, GoalActive, store.goal.Status)
}

func TestShutdownPausesAndPreventsContinuation(t *testing.T) {
	t.Parallel()
	store := &runtimeStore{goal: &Goal{SessionID: "session", GoalID: "goal", Status: GoalActive}}
	runner := &runtimeAgent{}
	runtime := NewRuntime(store, runner, nil)
	require.NoError(t, runtime.MaybeContinue(t.Context(), "session"))
	runtime.Stop(t.Context())
	require.Equal(t, GoalPaused, store.goal.Status)
	require.NoError(t, runtime.MaybeContinue(t.Context(), "session"))
	require.Equal(t, 1, runner.runs)
	_, err := runtime.Resume(t.Context(), "session")
	require.Error(t, err)
}

func TestCancelledBeforeDispatchPausesGoal(t *testing.T) {
	t.Parallel()
	store := &runtimeStore{goal: &Goal{SessionID: "session", GoalID: "goal", Status: GoalActive}}
	runner := &runtimeAgent{}
	runtime := NewRuntime(store, runner, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, runtime.MaybeContinue(ctx, "session"), context.Canceled)
	require.Equal(t, GoalPaused, store.goal.Status)
	require.Zero(t, runner.runs)
}

func TestResumeWaitsForCoordinatorFinalization(t *testing.T) {
	t.Parallel()
	store := &runtimeStore{goal: &Goal{SessionID: "session", GoalID: "goal", Status: GoalPaused}}
	runtime := NewRuntime(store, &runtimeAgent{}, nil)
	runtime.BeginTurn("session")
	_, err := runtime.Resume(t.Context(), "session")
	require.ErrorContains(t, err, "still stopping")
	runtime.AfterTurn(t.Context(), "session", nil, context.Canceled)
	runtime.mu.Lock()
	require.Zero(t, runtime.turns["session"])
	runtime.mu.Unlock()
	require.Equal(t, GoalPaused, store.goal.Status)
}
