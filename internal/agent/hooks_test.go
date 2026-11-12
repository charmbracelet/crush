package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

// errorStreamModel fails the stream with the given error so Run takes
// the provider-error path.
type errorStreamModel struct {
	finishStreamModel
	err error
}

func (m *errorStreamModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return nil, m.err
}

// recordingHooks installs hooks for the given events on the agent that
// append "event [last_assistant_message|error|trigger]" to a log, and
// returns a reader for it.
func recordingHooks(t *testing.T, sa *sessionAgent, events ...string) func() []string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "hooks.log")
	record := config.HookConfig{Command: `jq -r '[.hook_event_name, (.last_assistant_message // .error // .trigger // empty)] | join(" ")' >> ` + logPath}
	cfg := make(map[string][]config.HookConfig, len(events))
	for _, ev := range events {
		cfg[ev] = []config.HookConfig{record}
	}
	sa.hooks = hooks.NewRunner(hooks.Static(cfg), dir, dir)
	return func() []string {
		data, _ := os.ReadFile(logPath)
		text := strings.TrimSpace(string(data))
		if text == "" {
			return nil
		}
		return strings.Split(text, "\n")
	}
}

var turnEvents = []string{hooks.EventTurnStart, hooks.EventTurnEnd, hooks.EventInterrupt, hooks.EventStopFailure}

func TestTurnHooks_StartAndEnd(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)
	read := recordingHooks(t, sa, turnEvents...)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
	require.NoError(t, err)

	require.Equal(t, []string{"TurnStart", "TurnEnd done"}, read())
}

func TestTurnHooks_SubAgentFiresNone(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)
	read := recordingHooks(t, sa, turnEvents...)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi", NonInteractive: true})
	require.NoError(t, err)

	require.Empty(t, read())
}

func TestTurnHooks_ProviderErrorFiresStopFailureOnly(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &errorStreamModel{err: errors.New("the provider fell over")}
	sa := testSessionAgent(env, model, model, "system").(*sessionAgent)
	read := recordingHooks(t, sa, turnEvents...)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
	require.Error(t, err)

	lines := read()
	require.Len(t, lines, 2)
	require.Equal(t, "TurnStart", lines[0])
	require.True(t, strings.HasPrefix(lines[1], "StopFailure "), lines[1])
	require.Contains(t, lines[1], "fell over")
}

func TestTurnHooks_CancelFiresInterruptOnly(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &errorStreamModel{err: context.Canceled}
	sa := testSessionAgent(env, model, model, "system").(*sessionAgent)
	read := recordingHooks(t, sa, turnEvents...)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
	require.ErrorIs(t, err, context.Canceled)

	require.Equal(t, []string{"TurnStart", "Interrupt"}, read())
}

func TestCompactHooks_PreCompactBlocks(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)
	dir := t.TempDir()
	sa.hooks = hooks.NewRunner(hooks.Static(map[string][]config.HookConfig{
		hooks.EventPreCompact:  {{Command: `jq -e '.trigger == "manual"' >/dev/null || exit 1; echo "not now" >&2; exit 2`}},
		hooks.EventPostCompact: {{Command: `touch ` + filepath.Join(dir, "post")}},
	}), dir, dir)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
	require.NoError(t, err)

	err = sa.Summarize(t.Context(), sess.ID, nil, nil)
	require.ErrorIs(t, err, ErrCompactionBlocked)
	require.Contains(t, err.Error(), "not now")
	require.NoFileExists(t, filepath.Join(dir, "post"))
	require.False(t, sa.IsSessionBusy(sess.ID))
}

func TestCompactHooks_PostCompactFires(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)
	read := recordingHooks(t, sa, hooks.EventPreCompact, hooks.EventPostCompact)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(t.Context(), sess.ID, nil, nil))
	require.Equal(t, []string{"PreCompact manual", "PostCompact manual"}, read())
}
