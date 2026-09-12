package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPayloadWithTranscript(t *testing.T) {
	t.Parallel()

	t.Run("transcript included when non-empty", func(t *testing.T) {
		t.Parallel()
		payload := BuildPayloadWithTranscript(EventPreToolUse, "sess-1", "/work", "bash", `{"command":"ls"}`, "User: hello")
		var p Payload
		require.NoError(t, json.Unmarshal(payload, &p))
		require.Equal(t, "User: hello", p.Transcript)
		require.Equal(t, EventPreToolUse, p.Event)
	})

	t.Run("transcript omitted when empty", func(t *testing.T) {
		t.Parallel()
		payload := BuildPayloadWithTranscript(EventPreToolUse, "sess-1", "/work", "bash", `{"command":"ls"}`, "")
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(payload, &raw))
		require.NotContains(t, raw, "transcript")
	})

	t.Run("BuildPayload never includes transcript", func(t *testing.T) {
		t.Parallel()
		payload := BuildPayload(EventPreToolUse, "sess-1", "/work", "bash", `{"command":"ls"}`)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(payload, &raw))
		require.NotContains(t, raw, "transcript")
	})
}

func TestBuildEnvWithTranscript(t *testing.T) {
	t.Parallel()

	t.Run("CRUSH_TRANSCRIPT set when non-empty", func(t *testing.T) {
		t.Parallel()
		env := BuildEnvWithTranscript(EventPreToolUse, "bash", "sess-1", "/work", "/work", `{}`, "line one\nline two")
		require.Contains(t, env, "CRUSH_TRANSCRIPT=line one\nline two")
	})

	t.Run("CRUSH_TRANSCRIPT omitted when empty", func(t *testing.T) {
		t.Parallel()
		env := BuildEnvWithTranscript(EventPreToolUse, "bash", "sess-1", "/work", "/work", `{}`, "")
		for _, kv := range env {
			require.NotContains(t, kv, "CRUSH_TRANSCRIPT=")
		}
	})

	t.Run("BuildEnv never sets CRUSH_TRANSCRIPT", func(t *testing.T) {
		t.Parallel()
		env := BuildEnv(EventPreToolUse, "bash", "sess-1", "/work", "/work", `{}`)
		for _, kv := range env {
			require.NotContains(t, kv, "CRUSH_TRANSCRIPT=")
		}
	})
}

func TestRunnerTranscriptProviderNotCalledWithoutOptIn(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	r := NewRunner([]config.HookConfig{
		{Command: `echo '{"decision":"allow"}'`},
	}, t.TempDir(), t.TempDir())
	r.WithTranscriptProvider(func(context.Context, string) string {
		calls.Add(1)
		return "should not be consulted"
	})
	_, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Zero(t, calls.Load())
}

func TestRunnerTranscriptDeliveredWhenOptedIn(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "payload.txt")
	var calls atomic.Int64
	r := NewRunner([]config.HookConfig{
		{Command: "cat > " + out},
		{Command: `echo '{"decision":"allow"}'`, IncludeTranscript: true},
	}, t.TempDir(), t.TempDir())
	r.WithTranscriptProvider(func(context.Context, string) string {
		calls.Add(1)
		return "User: please run the tests"
	})
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{"command":"npm test"}`)
	require.NoError(t, err)
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
	r := NewRunner([]config.HookConfig{
		{Command: `echo '{"decision":"allow"}'`, IncludeTranscript: true},
	}, t.TempDir(), t.TempDir())
	// No provider wired: hooks still run, transcript just stays empty.
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
}
