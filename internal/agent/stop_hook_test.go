package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// stopHookAgent builds a session agent whose Stop hooks run the given
// shell command.
func stopHookAgent(t *testing.T, env fakeEnv, command string, broker *pubsub.Broker[notify.RunComplete]) *sessionAgent {
	t.Helper()
	runner := hooks.NewRunner([]config.HookConfig{{Command: command}}, env.workingDir, env.workingDir)
	return NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: &finishStreamModel{text: "x"}, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 1000}},
		SmallModel:  Model{Model: &finishStreamModel{text: "t"}, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 1000}},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
		StopHooks:   runner,
	}).(*sessionAgent)
}

// A queued prompt that is discarded by Cancel never ran, so it owes no
// Stop event. Firing one there blocked Cancel — which holds the
// per-session mutex — for the hook's full timeout, wedging Escape and
// app shutdown behind a slow Stop hook.
func TestCancelDoesNotBlockOnStopHookForDroppedQueuedPrompts(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	marker := filepath.Join(t.TempDir(), "stop-fired")
	sa := stopHookAgent(t, env, fmt.Sprintf("touch %s; sleep 100", quoteShellPath(marker)), broker)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, RunID: "run-q", Prompt: "hi"})
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		sa.Cancel(sess.ID)
		done <- time.Since(start)
	}()

	select {
	case d := <-done:
		require.Less(t, d, 2*time.Second,
			"Cancel must not block on a Stop hook for a prompt that never ran")
	case <-time.After(45 * time.Second):
		t.Fatal("Cancel is wedged behind the Stop hook; Escape and shutdown hang")
	}

	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))
	_, statErr := os.Stat(marker)
	require.True(t, os.IsNotExist(statErr),
		"Stop must not fire for a queued prompt that never produced a turn")
}

// When the run context is already cancelled (Escape, shutdown) the Stop
// hook gets a short leash instead of its full configured timeout, so the
// session is not pinned busy behind an observational hook.
func TestStopHookCappedDuringTeardown(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	sa := stopHookAgent(t, env, "sleep 100", broker)

	dead, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		sa.publishRunComplete(dead, SessionAgentCall{SessionID: "s1", RunID: "r1"},
			notify.RunComplete{SessionID: "s1", RunID: "r1", Cancelled: true})
		done <- time.Since(start)
	}()

	select {
	case d := <-done:
		require.Less(t, d, stopHookTeardownGrace+3*time.Second,
			"a cancelled turn must not wait out the Stop hook's full timeout")
	case <-time.After(45 * time.Second):
		t.Fatal("cancelled turn wedged behind the Stop hook")
	}
}

// A Stop hook still fires, with its full timeout, on a clean turn end.
func TestStopHookFiresOnCleanCompletion(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	marker := filepath.Join(t.TempDir(), "stop-fired")
	sa := stopHookAgent(t, env, fmt.Sprintf("touch %s", quoteShellPath(marker)), broker)

	sa.publishRunComplete(t.Context(), SessionAgentCall{SessionID: "s1", RunID: "r1"},
		notify.RunComplete{SessionID: "s1", RunID: "r1"})

	_, statErr := os.Stat(marker)
	require.NoError(t, statErr, "Stop must fire when a real turn ends")
}

// quoteShellPath single-quotes a path for the embedded POSIX shell.
// Windows temp paths contain backslashes that the shell would otherwise
// treat as escapes, so the marker never appears and the test fails.
func quoteShellPath(path string) string {
	return "'" + strings.ReplaceAll(filepath.ToSlash(path), "'", `'"'"'`) + "'"
}
