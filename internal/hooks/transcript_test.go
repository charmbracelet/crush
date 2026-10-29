package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildPayloadTranscript(t *testing.T) {
	t.Parallel()

	t.Run("transcript included when non-empty", func(t *testing.T) {
		t.Parallel()
		payload := BuildPayload(Event{
			Name:       EventPreToolUse,
			SessionID:  "sess-1",
			Tool:       &ToolCall{Name: "bash", Input: `{"command":"ls"}`},
			Transcript: "User: hello",
		}, "/work")
		var p Payload
		require.NoError(t, json.Unmarshal(payload, &p))
		require.Equal(t, "User: hello", p.Transcript)
		require.Equal(t, EventPreToolUse, p.Event)
	})

	t.Run("transcript omitted when empty", func(t *testing.T) {
		t.Parallel()
		payload := BuildPayload(Event{
			Name:      EventPreToolUse,
			SessionID: "sess-1",
			Tool:      &ToolCall{Name: "bash", Input: `{"command":"ls"}`},
		}, "/work")
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(payload, &raw))
		require.NotContains(t, raw, "transcript")
	})
}

func TestBuildEnvTranscript(t *testing.T) {
	t.Parallel()

	t.Run("CRUSH_TRANSCRIPT set when non-empty", func(t *testing.T) {
		t.Parallel()
		env := BuildEnv(Event{
			Name:       EventPreToolUse,
			SessionID:  "sess-1",
			Tool:       &ToolCall{Name: "bash", Input: `{}`},
			Transcript: "line one\nline two",
		}, "/work", "/work")
		require.Contains(t, env, "CRUSH_TRANSCRIPT=line one\nline two")
	})

	t.Run("CRUSH_TRANSCRIPT omitted when empty", func(t *testing.T) {
		t.Parallel()
		env := BuildEnv(Event{
			Name:      EventPreToolUse,
			SessionID: "sess-1",
			Tool:      &ToolCall{Name: "bash", Input: `{}`},
		}, "/work", "/work")
		for _, kv := range env {
			require.NotContains(t, kv, "CRUSH_TRANSCRIPT=")
		}
	})
}

func TestRunnerTranscriptProviderNotCalledWithoutOptIn(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	r := NewRunner(Static(map[string][]HookConfig{
		EventPreToolUse: {
			{Command: `echo '{"decision":"allow"}'`},
		},
	}), t.TempDir(), t.TempDir())
	r.WithTranscriptProvider(func(context.Context, string) string {
		calls.Add(1)
		return "should not be consulted"
	})
	r.Run(context.Background(), Event{
		Name:      EventPreToolUse,
		SessionID: "sess",
		Tool:      &ToolCall{Name: "bash", Input: `{}`},
	})
	require.Zero(t, calls.Load())
}

func TestRunnerTranscriptDeliveredWhenOptedIn(t *testing.T) {
	t.Parallel()
	// Redirect relative to the runner's cwd: an absolute Windows path
	// would carry backslashes that the embedded POSIX shell treats as
	// escape characters, mangling the redirect target.
	cwd := t.TempDir()
	out := filepath.Join(cwd, "payload.txt")
	var calls atomic.Int64
	r := NewRunner(Static(map[string][]HookConfig{
		EventPreToolUse: {
			{Command: "cat > payload.txt"},
			{Command: `echo '{"decision":"allow"}'`, IncludeTranscript: true},
		},
	}), cwd, t.TempDir())
	r.WithTranscriptProvider(func(context.Context, string) string {
		calls.Add(1)
		return "User: please run the tests"
	})
	result := r.Run(context.Background(), Event{
		Name:      EventPreToolUse,
		SessionID: "sess",
		Tool:      &ToolCall{Name: "bash", Input: `{"command":"npm test"}`},
	})
	require.Equal(t, DecisionAllow, result.Decision)
	// The provider is consulted exactly once per run, even with two
	// matching hooks, so concurrent hooks share one snapshot.
	require.Equal(t, int64(1), calls.Load())

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	var p Payload
	require.NoError(t, json.Unmarshal(data, &p))
	require.Equal(t, "User: please run the tests", p.Transcript)
	require.Equal(t, "bash", p.ToolName)
}

func TestRunnerTranscriptProviderNilSafe(t *testing.T) {
	t.Parallel()
	r := NewRunner(Static(map[string][]HookConfig{
		EventPreToolUse: {
			{Command: `echo '{"decision":"allow"}'`, IncludeTranscript: true},
		},
	}), t.TempDir(), t.TempDir())
	// No provider wired: hooks still run, transcript just stays empty.
	result := r.Run(context.Background(), Event{
		Name:      EventPreToolUse,
		SessionID: "sess",
		Tool:      &ToolCall{Name: "bash", Input: `{}`},
	})
	require.Equal(t, DecisionAllow, result.Decision)
}
