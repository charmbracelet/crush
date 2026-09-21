package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

// newSessionStartAgent builds the minimum agent for reportSessionStart.
func newSessionStartAgent(t *testing.T, marker string) *sessionAgent {
	t.Helper()

	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventSessionStart: {{
				Command: `printf '%s\n' "$CRUSH_SESSION_ID" >> ` + marker,
			}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())

	dir := filepath.Dir(marker)
	return &sessionAgent{
		hookDispatcher: hooks.NewDispatcher(cfg.Hooks, dir, dir),
	}
}

func TestReportSessionStartFiresOncePerSession(t *testing.T) {
	t.Parallel()

	marker := filepath.Join(t.TempDir(), "starts.log")
	agent := newSessionStartAgent(t, marker)

	agent.reportSessionStart(t.Context(), "session-1")
	agent.reportSessionStart(t.Context(), "session-1")

	// A second session is a new start, not a repeat.
	agent.reportSessionStart(t.Context(), "session-2")

	got, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "session-1\nsession-2\n", string(got))
}

// An agent without a dispatcher must be a no-op rather than a panic.
func TestReportSessionStartWithoutHooks(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	agent.reportSessionStart(t.Context(), "session-1")
}
