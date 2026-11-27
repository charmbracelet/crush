package hooks

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestPreModelUsePayload(t *testing.T) {
	t.Parallel()
	ev := Event{Name: EventPreModelUse, SessionID: "session", Provider: "local", Model: "test/model"}
	var payload Payload
	require.NoError(t, json.Unmarshal(BuildPayload(ev, "/work"), &payload))
	require.Equal(t, "local", payload.Provider)
	require.Equal(t, "test/model", payload.Model)
	require.Equal(t, "session", payload.SessionID)
}

func TestPreModelUseFailure(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, command, reason string
	}{
		{"silent failure", "exit 1", "exit status 1"},
		{"timeout", "sleep 5", "deadline"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hook := HookConfig{Command: tt.command, Timeout: 1}
			dir := t.TempDir()
			r := NewRunner(Static(map[string][]HookConfig{EventPreModelUse: {hook}}), dir, dir)
			result := r.Run(t.Context(), Event{Name: EventPreModelUse, Provider: "local", Model: "test"})
			require.True(t, result.Blocked())
			require.Contains(t, result.Reason, tt.command)
			require.Contains(t, result.Reason, tt.reason)
		})
	}
}

func TestPreModelUseAbandonedHookBlocksRequest(t *testing.T) {
	original := runShell
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	runShell = func(_ context.Context, _ shell.RunOptions) error {
		close(started)
		<-release
		close(finished)
		return nil
	}
	t.Cleanup(func() {
		<-started
		close(release)
		<-finished
		runShell = original
	})

	dir := t.TempDir()
	hook := HookConfig{Command: "unresponsive", Timeout: 1}
	r := NewRunner(Static(map[string][]HookConfig{EventPreModelUse: {hook}}), dir, dir)
	result := r.Run(t.Context(), Event{Name: EventPreModelUse, Provider: "local", Model: "test"})
	require.True(t, result.Blocked())
	require.Contains(t, result.Reason, "unresponsive")
	require.Contains(t, result.Reason, "deadline")
}
