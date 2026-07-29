package permission

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestYoloBypassIsLogged proves a skipped permission leaves a record.
//
// The yolo branch is the first statement in Request and returns before the
// notification broker is touched, so -- unlike plan-mode denials, hook
// approvals and session auto-approvals, which all publish a notification --
// nothing downstream can observe it. `crush run --yolo` has no TUI prompt
// indicator either, so with no log line a bypassed permission is invisible
// both live and after the fact.
//
// Not parallel: captureLogs swaps the global slog default.
func TestYoloBypassIsLogged(t *testing.T) {
	buf := captureLogs(t)

	svc := NewPermissionService(t.TempDir(), true, nil)

	granted, err := svc.Request(context.Background(), CreatePermissionRequest{
		SessionID:  "session-1",
		ToolCallID: "call-1",
		ToolName:   "bash",
		Action:     "execute",
		Path:       "/tmp/somewhere-else",
	})
	require.NoError(t, err)
	require.True(t, granted, "yolo mode must still auto-grant")

	logs := buf.String()
	require.Contains(t, logs, "yolo mode is skipping permission requests",
		"a bypassed permission must leave a log record; got:\n%s", logs)
	for _, want := range []string{"bash", "execute", "/tmp/somewhere-else", "call-1", "session-1"} {
		require.Contains(t, logs, want,
			"the bypass record must identify what was auto-granted (missing %q); got:\n%s", want, logs)
	}
}

// TestYoloStartupIsLogged proves a service constructed in yolo mode says so
// once, so a log can be read back to tell whether the whole workspace ran
// without consent prompts.
func TestYoloStartupIsLogged(t *testing.T) {
	buf := captureLogs(t)

	NewPermissionService(t.TempDir(), true, nil)
	require.Contains(t, buf.String(), "started in yolo mode")
}

// TestNonYoloStartupIsNotLogged is the other side of the bound: a normal
// service must not claim to be in yolo mode.
func TestNonYoloStartupIsNotLogged(t *testing.T) {
	buf := captureLogs(t)

	NewPermissionService(t.TempDir(), false, nil)
	require.NotContains(t, strings.ToLower(buf.String()), "yolo")
}

// TestSetSkipRequestsLogsOnlyTransitions proves the runtime toggle (ctrl+y,
// and the workspace API behind it) records the change to the consent model,
// and that a no-op set does not spam the log.
func TestSetSkipRequestsLogsOnlyTransitions(t *testing.T) {
	buf := captureLogs(t)

	svc := NewPermissionService(t.TempDir(), false, nil)

	svc.SetSkipRequests(true)
	require.True(t, svc.SkipRequests(), "SetSkipRequests(true) must still take effect")
	require.Equal(t, 1, strings.Count(buf.String(), "Permission skip mode (yolo) changed"))

	svc.SetSkipRequests(true) // no-op
	require.Equal(t, 1, strings.Count(buf.String(), "Permission skip mode (yolo) changed"),
		"a repeated set must not log a transition that did not happen")

	svc.SetSkipRequests(false)
	require.False(t, svc.SkipRequests(), "SetSkipRequests(false) must still take effect")
	require.Equal(t, 2, strings.Count(buf.String(), "Permission skip mode (yolo) changed"))
}
