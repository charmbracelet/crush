package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// stubCoordinator records the prompts it is run with. outcome, when set,
// decides each run's result by its index; pub, when set, receives the run's
// terminal RunComplete the way coordinator.run publishes it. drained, when
// set, is the RunID of a queued prompt the first run drains, publishing its
// RunComplete too (sessionAgent's queue recursion). busySession is the
// session the stub reports busy.
type stubCoordinator struct {
	Coordinator
	prompts     []string
	accepted    int
	begun       int
	cancels     []string
	onBegin     func()
	outcome     func(n int) (*fantasy.AgentResult, error)
	pub         pubsub.Publisher[notify.RunComplete]
	drained     string
	busySession string
}

func (s *stubCoordinator) IsSessionBusy(sessionID string) bool {
	return sessionID != "" && sessionID == s.busySession
}
func (s *stubCoordinator) IsBusy() bool { return s.busySession != "" }

func (s *stubCoordinator) BeginAccepted(string) *AcceptedRun {
	s.begun++
	if s.onBegin != nil {
		s.onBegin()
	}
	return nil
}

func (s *stubCoordinator) Cancel(sessionID string) { s.cancels = append(s.cancels, sessionID) }
func (s *stubCoordinator) CancelAll()              { s.cancels = append(s.cancels, "*") }

func (s *stubCoordinator) Run(ctx context.Context, sessionID, prompt string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	n := len(s.prompts)
	s.prompts = append(s.prompts, prompt)
	res, err := &fantasy.AgentResult{}, error(nil)
	if s.outcome != nil {
		res, err = s.outcome(n)
	}
	if err == nil && s.pub != nil {
		if n == 0 && s.drained != "" {
			s.pub.PublishMustDeliver(ctx, pubsub.UpdatedEvent, notify.RunComplete{
				SessionID: sessionID,
				RunID:     s.drained,
				Text:      "drained",
			})
		}
		s.pub.PublishMustDeliver(ctx, pubsub.UpdatedEvent, notify.RunComplete{
			SessionID: sessionID,
			RunID:     RunIDFromContext(ctx),
			Text:      prompt,
		})
		MarkRunCompletePublished(ctx)
	}
	return res, err
}

func (s *stubCoordinator) RunAccepted(ctx context.Context, _ *AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	s.accepted++
	return s.Run(ctx, sessionID, prompt, attachments...)
}

// sessionAgentCoordinator runs prompts straight on a real SessionAgent,
// which publishes its own RunComplete (no OnComplete), carrying the RunID.
type sessionAgentCoordinator struct {
	Coordinator
	sa SessionAgent
}

func (c sessionAgentCoordinator) Run(ctx context.Context, sessionID, prompt string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	return c.RunAccepted(ctx, nil, sessionID, prompt)
}

func (c sessionAgentCoordinator) RunAccepted(ctx context.Context, accept *AcceptedRun, sessionID, prompt string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	return c.sa.Run(ctx, SessionAgentCall{SessionID: sessionID, RunID: RunIDFromContext(ctx), Prompt: prompt, Accepted: accept})
}

func (c sessionAgentCoordinator) BeginAccepted(sessionID string) *AcceptedRun {
	return c.sa.BeginAccepted(sessionID)
}

// stopHookScript installs script as an executable Stop hook (configured
// under the lower-case "stop" key) and returns its runner source and dir.
func stopHookScript(t *testing.T, script string) (func() *hooks.Runner, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "stop.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755))
	cfg := &config.Config{Hooks: map[string][]config.HookConfig{
		"stop": {{Command: path}},
	}}
	require.NoError(t, cfg.ValidateHooks())
	require.Len(t, cfg.Hooks[hooks.EventStop], 1)
	runner := hooks.NewRunner(cfg.Hooks[hooks.EventStop], dir, dir)
	return func() *hooks.Runner { return runner }, dir
}

func TestWithStopHooksWithoutHooks(t *testing.T) {
	t.Parallel()
	inner := &stubCoordinator{}
	require.Same(t, inner, WithStopHooks(inner, nil))
	// No Stop hook in the current config: the turn just ends.
	_, err := WithStopHooks(inner, func() *hooks.Runner { return nil }).Run(t.Context(), "s1", "go")
	require.NoError(t, err)
	require.Equal(t, []string{"go"}, inner.prompts)
}

func TestStopHookAllowsFinish(t *testing.T) {
	t.Parallel()
	for name, script := range map[string]string{
		"exit 0":           "exit 0\n",
		"non-blocking err": "echo oops >&2; exit 1\n",
		"halt":             "echo enough >&2; exit 49\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stop, _ := stopHookScript(t, script)
			inner := &stubCoordinator{}
			_, err := WithStopHooks(inner, stop).Run(t.Context(), "s1", "fix the bug")
			require.NoError(t, err)
			require.Equal(t, []string{"fix the bug"}, inner.prompts)
		})
	}
}

func TestStopHookBlockReruns(t *testing.T) {
	t.Parallel()
	// Claude Code style: record stdin, block once with decision "block".
	stop, dir := stopHookScript(t, `cat >> payloads; echo >> payloads
if [ ! -f blocked ]; then
  touch blocked
  echo '{"decision":"block","reason":"lint is red: fix main.go"}'
fi
`)
	inner := &stubCoordinator{}
	_, err := WithStopHooks(inner, stop).RunAccepted(t.Context(), nil, "s1", "fix the bug")
	require.NoError(t, err)
	require.Equal(t, []string{"fix the bug", "lint is red: fix main.go"}, inner.prompts)
	// The re-run takes a fresh accept, so a Cancel landing before it is
	// active is still caught (cancel-on-entry).
	require.Equal(t, 1, inner.begun)
	require.Equal(t, 2, inner.accepted)

	data, err := os.ReadFile(filepath.Join(dir, "payloads"))
	require.NoError(t, err)
	payloads := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, payloads, 2)
	require.Contains(t, payloads[0], `"event":"Stop"`)
	require.Contains(t, payloads[0], `"session_id":"s1"`)
	require.Contains(t, payloads[0], `"stop_hook_active":false`)
	require.Contains(t, payloads[1], `"stop_hook_active":true`)
}

func TestStopHookLoopGuard(t *testing.T) {
	t.Parallel()
	stop, _ := stopHookScript(t, "echo still red >&2; exit 2\n")
	inner := &stubCoordinator{}
	_, err := WithStopHooks(inner, stop).Run(t.Context(), "s1", "go")
	require.NoError(t, err)
	require.Equal(t, []string{"go", "still red", "still red", "still red"}, inner.prompts)
}

func TestStopHookBlockWithoutReason(t *testing.T) {
	t.Parallel()
	stop, _ := stopHookScript(t, `[ -f blocked ] || { touch blocked; echo '{"decision":"block"}'; }
`)
	inner := &stubCoordinator{}
	_, err := WithStopHooks(inner, stop).Run(t.Context(), "s1", "go")
	require.NoError(t, err)
	require.Len(t, inner.prompts, 2)
	require.NotEmpty(t, inner.prompts[1], "an empty prompt would fail the re-run")
}

func TestStopHookSkippedOnError(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for name, tc := range map[string]struct {
		ctx     context.Context
		outcome func(int) (*fantasy.AgentResult, error)
	}{
		"run error": {outcome: func(int) (*fantasy.AgentResult, error) { return nil, errors.New("boom") }},
		"canceled":  {ctx: canceled},
		// A busy session queues the prompt and returns (nil, nil): no turn
		// finished here, so there is nothing to gate.
		"queued": {outcome: func(int) (*fantasy.AgentResult, error) { return nil, nil }},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stop, dir := stopHookScript(t, "touch ran; exit 2\n")
			inner := &stubCoordinator{outcome: tc.outcome}
			ctx := tc.ctx
			if ctx == nil {
				ctx = t.Context()
			}
			_, _ = WithStopHooks(inner, stop).Run(ctx, "s1", "go")
			require.Len(t, inner.prompts, 1)
			require.NoFileExists(t, filepath.Join(dir, "ran"))
		})
	}
}

// A `run --json` caller exits on the first RunComplete carrying its RunID,
// so a blocked attempt must not publish one: only the final attempt's
// RunComplete may reach subscribers.
func TestStopHookPublishesOnlyFinalRunComplete(t *testing.T) {
	t.Parallel()
	const blockOnce = `if [ ! -f blocked ]; then touch blocked; echo lint is red >&2; exit 2; fi
`

	t.Run("final attempt", func(t *testing.T) {
		t.Parallel()
		broker := pubsub.NewBroker[notify.RunComplete]()
		events := broker.Subscribe(t.Context())
		stop, _ := stopHookScript(t, blockOnce)
		inner := &stubCoordinator{pub: holdRunComplete(broker)}
		ctx := WithRunCompleteMarker(WithRunID(t.Context(), "run-1"))

		_, err := WithStopHooks(inner, stop).Run(ctx, "s1", "fix the bug")
		require.NoError(t, err)
		require.Len(t, inner.prompts, 2)
		require.True(t, RunCompletePublished(ctx))
		ev := <-events
		require.Equal(t, "run-1", ev.Payload.RunID)
		require.Equal(t, "lint is red", ev.Payload.Text)
		select {
		case ev := <-events:
			t.Fatalf("unexpected extra RunComplete: %+v", ev.Payload)
		default:
		}
	})

	// A re-run that fails before publishing leaves nothing released and the
	// caller's marker unset, so backend.runAgent publishes its error
	// fallback instead of the caller waiting forever.
	t.Run("re-run fails early", func(t *testing.T) {
		t.Parallel()
		broker := pubsub.NewBroker[notify.RunComplete]()
		events := broker.Subscribe(t.Context())
		stop, _ := stopHookScript(t, blockOnce)
		inner := &stubCoordinator{
			pub: holdRunComplete(broker),
			outcome: func(n int) (*fantasy.AgentResult, error) {
				if n > 0 {
					return nil, errors.New("models unavailable")
				}
				return &fantasy.AgentResult{}, nil
			},
		}
		ctx := WithRunCompleteMarker(WithRunID(t.Context(), "run-1"))

		_, err := WithStopHooks(inner, stop).Run(ctx, "s1", "fix the bug")
		require.EqualError(t, err, "models unavailable")
		require.False(t, RunCompletePublished(ctx))
		select {
		case ev := <-events:
			t.Fatalf("unexpected RunComplete: %+v", ev.Payload)
		default:
		}
	})

	// Same contract on the real sessionAgent publish path: one RunComplete,
	// for the re-run's assistant message.
	t.Run("real session agent", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		broker := pubsub.NewBroker[notify.RunComplete]()
		t.Cleanup(broker.Shutdown)
		events := broker.Subscribe(t.Context())
		model := Model{
			Model:      &finishStreamModel{text: "done"},
			CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000},
		}
		sa := NewSessionAgent(SessionAgentOptions{
			LargeModel:  model,
			SmallModel:  model,
			IsYolo:      true,
			Sessions:    env.sessions,
			Messages:    env.messages,
			RunComplete: holdRunComplete(broker),
		})
		sess, err := env.sessions.Create(t.Context(), "session")
		require.NoError(t, err)
		stop, _ := stopHookScript(t, blockOnce)
		ctx := WithRunCompleteMarker(WithRunID(t.Context(), "run-1"))

		_, err = WithStopHooks(sessionAgentCoordinator{sa: sa}, stop).Run(ctx, sess.ID, "fix the bug")
		require.NoError(t, err)

		msgs, err := env.messages.List(t.Context(), sess.ID)
		require.NoError(t, err)
		var users []string
		var lastAssistant string
		for _, m := range msgs {
			switch m.Role {
			case message.User:
				users = append(users, m.Content().String())
			case message.Assistant:
				lastAssistant = m.ID
			}
		}
		require.Equal(t, []string{"fix the bug", "lint is red"}, users)
		require.True(t, RunCompletePublished(ctx))
		ev := <-events
		require.Equal(t, "run-1", ev.Payload.RunID)
		require.Equal(t, lastAssistant, ev.Payload.MessageID)
		select {
		case ev := <-events:
			t.Fatalf("unexpected extra RunComplete: %+v", ev.Payload)
		default:
		}
	})

	// Outside a gate, and inside one for a sub-agent's child session,
	// events pass straight through; a queued prompt of the gated session
	// with its own RunID is held (TestStopHookHoldsQueuedRunComplete).
	t.Run("pass-through", func(t *testing.T) {
		t.Parallel()
		broker := pubsub.NewBroker[notify.RunComplete]()
		events := broker.Subscribe(t.Context())
		pub := holdRunComplete(broker)
		pub.PublishMustDeliver(t.Context(), pubsub.UpdatedEvent, notify.RunComplete{RunID: "r"})
		require.Equal(t, "r", (<-events).Payload.RunID)

		held := &stopGate{sessionID: "s1"}
		gated := context.WithValue(t.Context(), stopGateKey{}, held)
		pub.PublishMustDeliver(gated, pubsub.UpdatedEvent, notify.RunComplete{SessionID: "child"})
		pub.PublishMustDeliver(gated, pubsub.UpdatedEvent, notify.RunComplete{SessionID: "s1", RunID: "queued"})
		select {
		case ev := <-events:
			require.Equal(t, "child", ev.Payload.SessionID)
		default:
			t.Fatal("a child session's RunComplete was held")
		}
		require.Empty(t, events)
		require.Len(t, held.held, 1)
		require.Equal(t, "queued", held.held[0].rc.RunID)
	})
}

// Cancel while a Stop hook runs (or right after it blocked) must stop the
// gate: no further inner run, and exactly one RunComplete, cancelled.
func TestStopHookCancel(t *testing.T) {
	t.Parallel()
	// Blocks once, but only after the test creates "go".
	const waitForGo = `[ -f blocked ] && exit 0
touch started
while [ ! -f go ]; do sleep 0.05; done
touch blocked
echo red >&2; exit 2
`
	for name, tc := range map[string]struct {
		cancel func(Coordinator)
		stops  bool
	}{
		"Cancel during hook":    {func(c Coordinator) { c.Cancel("s1") }, true},
		"CancelAll during hook": {func(c Coordinator) { c.CancelAll() }, true},
		"other session":         {func(c Coordinator) { c.Cancel("other") }, false},
		// "" names no session; it is not the CancelAll sentinel.
		"empty session": {func(c Coordinator) { c.Cancel("") }, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stop, dir := stopHookScript(t, waitForGo)
			broker := pubsub.NewBroker[notify.RunComplete]()
			events := broker.Subscribe(t.Context())
			inner := &stubCoordinator{pub: holdRunComplete(broker)}
			gated := WithStopHooks(inner, stop)

			done := make(chan error, 1)
			go func() {
				_, err := gated.Run(WithRunID(t.Context(), "run-1"), "s1", "go")
				done <- err
			}()
			require.Eventually(t, func() bool {
				_, err := os.Stat(filepath.Join(dir, "started"))
				return err == nil
			}, 5*time.Second, 10*time.Millisecond)
			tc.cancel(gated)
			if !tc.stops {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "go"), nil, 0o644))
			}
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("gate did not return")
			}

			require.Len(t, inner.cancels, 1, "the cancel must reach the inner coordinator")
			ev := <-events
			require.Equal(t, "run-1", ev.Payload.RunID)
			require.Equal(t, tc.stops, ev.Payload.Cancelled)
			if tc.stops {
				require.Equal(t, []string{"go"}, inner.prompts)
			} else {
				require.Equal(t, []string{"go", "red"}, inner.prompts)
			}
			select {
			case ev := <-events:
				t.Fatalf("unexpected extra RunComplete: %+v", ev.Payload)
			default:
			}
		})
	}

	t.Run("after hook blocked", func(t *testing.T) {
		t.Parallel()
		stop, _ := stopHookScript(t, "echo red >&2; exit 2\n")
		broker := pubsub.NewBroker[notify.RunComplete]()
		events := broker.Subscribe(t.Context())
		inner := &stubCoordinator{pub: holdRunComplete(broker)}
		gated := WithStopHooks(inner, stop)
		// The cancel lands between the hook's block and the re-run.
		inner.onBegin = func() { gated.Cancel("s1") }

		_, err := gated.Run(WithRunID(t.Context(), "run-1"), "s1", "go")
		require.NoError(t, err)
		require.Equal(t, []string{"go"}, inner.prompts)
		require.Equal(t, 0, inner.accepted)
		ev := <-events
		require.True(t, ev.Payload.Cancelled)
		select {
		case ev := <-events:
			t.Fatalf("unexpected extra RunComplete: %+v", ev.Payload)
		default:
		}
	})
}

// The app builds its coordinator through NewCoordinatorWithStopHooks: the
// real coordinator must publish through the gate's hold, and Stop hooks are
// read from the current config per run (like PreToolUse), so a reload
// applies without re-init.
func TestNewCoordinatorWithStopHooks(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}},
  "hooks": {"stop": [{"command": "true"}]}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	c, err := NewCoordinatorWithStopHooks(t.Context(), CoordinatorOptions{
		Config:      cfg,
		Sessions:    env.sessions,
		Messages:    env.messages,
		Permissions: env.permissions,
		RunComplete: pubsub.NewBroker[notify.RunComplete](),
	})
	require.NoError(t, err)
	gated, ok := c.(*stopHookCoordinator)
	require.True(t, ok, "got %T", c)
	inner, ok := gated.Coordinator.(*coordinator)
	require.True(t, ok, "got %T", gated.Coordinator)
	require.NoError(t, inner.readyWg.Wait())
	require.IsType(t, holdingPublisher{}, inner.runComplete)

	require.NotNil(t, gated.stopHooks())
	cfg.Config().Hooks = nil
	require.Nil(t, gated.stopHooks())
}

// A RunID-bearing prompt queued behind a gated turn (send --wait, run
// --json) is drained inside the gate's first attempt. Its waiter must hear
// nothing before the Stop hook decides, and after a block every waiter gets
// the final attempt's result.
func TestStopHookHoldsQueuedRunComplete(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	events := broker.Subscribe(t.Context())
	model := &gatedStreamModel{text: "done", gate: make(chan struct{}), entered: make(chan struct{})}
	cw := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: model, CatwalkCfg: cw},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: cw},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: holdRunComplete(broker),
	})
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	stop, dir := stopHookScript(t, `[ -f blocked ] && exit 0
touch started
while [ ! -f go ]; do sleep 0.05; done
touch blocked; echo lint is red >&2; exit 2
`)
	gated := WithStopHooks(sessionAgentCoordinator{sa: sa}, stop)

	done := make(chan error, 1)
	go func() {
		_, err := gated.Run(WithRunID(t.Context(), "run-1"), sess.ID, "first")
		done <- err
	}()
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("run-1 never streamed")
	}
	res, err := gated.Run(WithRunID(t.Context(), "run-2"), sess.ID, "second")
	require.NoError(t, err)
	require.Nil(t, res, "run-2 must be queued behind run-1")
	close(model.gate)

	// Both turns have finished once the hook runs.
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, "started"))
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
	select {
	case ev := <-events:
		t.Fatalf("RunComplete before the Stop hook decided: %+v", ev.Payload)
	default:
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go"), nil, 0o644))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("gate did not return")
	}

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var users []string
	var last message.Message
	for _, m := range msgs {
		switch m.Role {
		case message.User:
			users = append(users, m.Content().String())
		case message.Assistant:
			last = m
		}
	}
	require.Equal(t, []string{"first", "second", "lint is red"}, users)
	got := map[string]string{}
	for len(events) > 0 {
		ev := <-events
		require.NotContains(t, got, ev.Payload.RunID, "duplicate RunComplete")
		got[ev.Payload.RunID] = ev.Payload.MessageID
		require.Equal(t, last.Content().String(), ev.Payload.Text)
		require.False(t, ev.Payload.Cancelled)
	}
	require.Equal(t, map[string]string{"run-1": last.ID, "run-2": last.ID}, got)
}

// Every RunComplete a gate holds is released when it ends, each under its
// own RunID: a blocked attempt's events carry the final attempt's result,
// and a lost one would hang `send --wait` forever.
func TestStopHookReleasesDrainedRunComplete(t *testing.T) {
	t.Parallel()
	const blockOnce = `if [ ! -f blocked ]; then touch blocked; echo lint is red >&2; exit 2; fi
`
	type want = notify.RunComplete
	for name, tc := range map[string]struct {
		script  string
		outcome func(int) (*fantasy.AgentResult, error)
		cancel  bool
		want    map[string]want
	}{
		"hook allows": {
			script: "exit 0\n",
			want:   map[string]want{"run-1": {Text: "go"}, "run-2": {Text: "drained"}},
		},
		"hook blocks": {
			script: blockOnce,
			want:   map[string]want{"run-1": {Text: "lint is red"}, "run-2": {Text: "lint is red"}},
		},
		// run-1 is left to backend.runAgent's error fallback (marker unset).
		"re-run fails early": {
			script: blockOnce,
			outcome: func(n int) (*fantasy.AgentResult, error) {
				if n > 0 {
					return nil, errors.New("models unavailable")
				}
				return &fantasy.AgentResult{}, nil
			},
			want: map[string]want{"run-2": {Error: "models unavailable"}},
		},
		"canceled after block": {
			script: blockOnce,
			cancel: true,
			want:   map[string]want{"run-1": {Text: "go", Cancelled: true}, "run-2": {Text: "drained", Cancelled: true}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stop, _ := stopHookScript(t, tc.script)
			broker := pubsub.NewBroker[notify.RunComplete]()
			events := broker.Subscribe(t.Context())
			inner := &stubCoordinator{pub: holdRunComplete(broker), drained: "run-2", outcome: tc.outcome}
			gated := WithStopHooks(inner, stop)
			if tc.cancel {
				inner.onBegin = func() { gated.Cancel("s1") }
			}

			ctx := WithRunCompleteMarker(WithRunID(t.Context(), "run-1"))
			_, _ = gated.Run(ctx, "s1", "go")
			// Unset only when run-1's event is left to runAgent's fallback.
			require.Equal(t, tc.want["run-1"] != want{}, RunCompletePublished(ctx))
			got := map[string]want{}
			for len(events) > 0 {
				ev := (<-events).Payload
				require.NotContains(t, got, ev.RunID, "duplicate RunComplete")
				got[ev.RunID] = want{Text: ev.Text, Error: ev.Error, Cancelled: ev.Cancelled}
			}
			require.Equal(t, tc.want, got)
		})
	}
}

// While a Stop hook judges a finished turn, the session must read busy: the
// UI keeps its spinner, SetMainAgent stays refused, and a new prompt queues
// instead of racing the re-run.
func TestStopHookSessionBusy(t *testing.T) {
	t.Parallel()
	stop, dir := stopHookScript(t, `touch started
while [ ! -f go ]; do sleep 0.05; done
`)
	inner := &stubCoordinator{busySession: "busy"}
	gated := WithStopHooks(inner, stop)
	require.True(t, gated.IsSessionBusy("busy"), "inner busy state must show through")
	require.False(t, gated.IsSessionBusy("s1"))
	inner.busySession = ""
	require.False(t, gated.IsBusy())

	done := make(chan error, 1)
	go func() {
		_, err := gated.Run(t.Context(), "s1", "go")
		done <- err
	}()
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, "started"))
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
	require.True(t, gated.IsSessionBusy("s1"))
	require.False(t, gated.IsSessionBusy("other"))
	require.True(t, gated.IsBusy())

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go"), nil, 0o644))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("gate did not return")
	}
	require.False(t, gated.IsSessionBusy("s1"))
	require.False(t, gated.IsBusy())
}
