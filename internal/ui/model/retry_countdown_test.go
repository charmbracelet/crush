package model

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

func newRetryTestUI(t *testing.T) *UI {
	t.Helper()
	com := &common.Common{}
	return &UI{
		com:    com,
		chat:   NewChat(com, ""),
		status: &Status{com: com},
	}
}

// TestClearRetryCountdownLeavesUnrelatedStatusAlone guards a state bug:
// clearRetryCountdown runs on every TypeAgentFinished / TypeAgentError,
// so it must be a no-op unless a countdown is actually live. A guard
// keyed on the sequence counter instead of the active status silently
// wiped the status bar on every turn after the session's first retry.
func TestClearRetryCountdownLeavesUnrelatedStatusAlone(t *testing.T) {
	t.Parallel()

	m := newRetryTestUI(t)

	// One retry happens and completes normally.
	m.beginRetryCountdown(notify.Notification{
		Type:       notify.TypeRetry,
		Message:    "rate limit",
		RetryDelay: 5 * time.Second,
		Attempt:    1,
		MaxRetries: 6,
	})
	require.Contains(t, m.status.msg.Msg, "Retrying in")
	m.clearRetryCountdown()
	require.Empty(t, m.status.msg.Msg, "countdown must be cleared once the retry resolves")

	// A later, unrelated status message must survive the next end-of-turn.
	m.status.SetInfoMsg(util.InfoMsg{Type: util.InfoTypeInfo, Msg: "Copied to clipboard"})
	m.clearRetryCountdown()
	require.Equal(t, "Copied to clipboard", m.status.msg.Msg,
		"clearRetryCountdown wiped a status message while no countdown was active")
}

// TestRetryCountdownTicksAreSequenced ensures a stale tick from a
// superseded countdown cannot resurrect the status bar.
func TestRetryCountdownTicksAreSequenced(t *testing.T) {
	t.Parallel()

	m := newRetryTestUI(t)
	m.beginRetryCountdown(notify.Notification{
		Type: notify.TypeRetry, RetryDelay: 5 * time.Second, Attempt: 1, MaxRetries: 6,
	})
	stale := m.retrySeq
	m.clearRetryCountdown()
	require.Nil(t, m.handleRetryTick(retryTickMsg{seq: stale}))
	require.Empty(t, m.status.msg.Msg)
}

// TestRetryCountdownIsSessionScoped ensures a retry notification for
// session A does not render a countdown when the user is viewing
// session B.
func TestRetryCountdownIsSessionScoped(t *testing.T) {
	t.Parallel()

	m := newRetryTestUI(t)
	m.session = &session.Session{ID: "s1"}

	cmd := m.handleAgentNotification(notify.Notification{
		SessionID:  "s1",
		Type:       notify.TypeRetry,
		Message:    "rate limit",
		RetryDelay: 10 * time.Second,
		Attempt:    1,
		MaxRetries: 6,
	})
	require.NotNil(t, cmd, "beginRetryCountdown must return a tick command")
	require.Contains(t, m.status.msg.Msg, "Retrying in")
	require.Contains(t, m.status.msg.Msg, "rate limit")

	cmd = m.handleAgentNotification(notify.Notification{
		SessionID:  "s2",
		Type:       notify.TypeRetry,
		Message:    "timeout",
		RetryDelay: 15 * time.Second,
		Attempt:    2,
		MaxRetries: 6,
	})
	require.Nil(t, cmd, "must ignore retry notification for a different session")
	require.Contains(t, m.status.msg.Msg, "Retrying in")
	require.Contains(t, m.status.msg.Msg, "rate limit")
	require.NotContains(t, m.status.msg.Msg, "timeout")

	m.clearRetryCountdown()
}

func TestRetryNotificationWithNoSessionIsIgnored(t *testing.T) {
	t.Parallel()

	m := newRetryTestUI(t)
	require.Nil(t, m.session)

	cmd := m.handleAgentNotification(notify.Notification{
		SessionID:  "s1",
		Type:       notify.TypeRetry,
		RetryDelay: 5 * time.Second,
		Attempt:    1,
		MaxRetries: 6,
	})
	require.Nil(t, cmd)
	require.Empty(t, m.status.msg.Msg)
}

// retryStubWorkspace is the minimum workspace surface handleAgentNotification
// touches: Config for the notification policy check and the agent probes the
// off-thread busy refresh performs. Anything else panics, which is the point.
type retryStubWorkspace struct {
	workspace.Workspace
}

func (retryStubWorkspace) Config() *config.Config { return nil }

// recordingNotifyBackend captures desktop notifications instead of sending them.
type recordingNotifyBackend struct {
	sent []notification.Notification
}

func (b *recordingNotifyBackend) Send(n notification.Notification) tea.Cmd {
	b.sent = append(b.sent, n)
	return nil
}

// newTerminalEdgeTestUI builds a UI wired far enough to observe the
// TypeAgentFinished / TypeAgentError terminal edge: desktop notification,
// turn-timer stop and busy/queue invalidation.
func newTerminalEdgeTestUI(t *testing.T) (*UI, *recordingNotifyBackend) {
	t.Helper()
	com := &common.Common{Workspace: retryStubWorkspace{}}
	backend := &recordingNotifyBackend{}
	m := &UI{
		com:           com,
		chat:          NewChat(com, ""),
		status:        &Status{com: com},
		notifyBackend: backend,
	}
	// shouldSendNotification requires focus reporting and an unfocused window.
	m.caps.ReportFocusEvents = true
	m.notifyWindowFocused = false
	return m, backend
}

// TestBackgroundSessionFinishStillNotifies guards a regression introduced by
// scoping the countdown: the session filter was applied to the whole of
// handleAgentNotification, so a TypeAgentFinished for a session other than the
// one on screen returned early and skipped the desktop notification, the turn
// timer stop and the busy/prompt-queue invalidation. Those are exactly the
// events that matter when the user is looking somewhere else -- the
// notification body even names the session ("Agent's turn completed in %q"),
// which is only meaningful for a session that is not the visible one. The
// countdown alone is session-scoped.
func TestBackgroundSessionFinishStillNotifies(t *testing.T) {
	// Not parallel: asserts on the process-wide turn timer.
	m, backend := newTerminalEdgeTestUI(t)
	m.session = &session.Session{ID: "visible"}

	common.StartTurn()
	busyGen := m.busyFetchGen
	queueGen := m.promptQueueGen

	m.handleAgentNotification(notify.Notification{
		SessionID:    "background",
		SessionTitle: "Other work",
		Type:         notify.TypeAgentFinished,
	})

	require.Len(t, backend.sent, 1,
		"a background session finishing must still raise a desktop notification")
	require.Contains(t, backend.sent[0].Message, "Other work")
	require.Empty(t, common.Elapsed(),
		"common.StopTurn must still run for a background session's terminal edge")
	require.Greater(t, m.busyFetchGen, busyGen,
		"the busy cache must still be invalidated on a background terminal edge")
	require.Greater(t, m.promptQueueGen, queueGen,
		"the prompt queue must still be invalidated on a background terminal edge")
}

// TestBackgroundSessionFinishDoesNotClearVisibleCountdown is the other half:
// the terminal edge of an unrelated session must not wipe the countdown the
// user is watching, even though it now runs the rest of the handler.
func TestBackgroundSessionFinishDoesNotClearVisibleCountdown(t *testing.T) {
	// Not parallel: asserts on the process-wide turn timer.
	m, _ := newTerminalEdgeTestUI(t)
	m.session = &session.Session{ID: "visible"}

	require.NotNil(t, m.handleAgentNotification(notify.Notification{
		SessionID:  "visible",
		Type:       notify.TypeRetry,
		Message:    "rate limit",
		RetryDelay: 30 * time.Second,
		Attempt:    1,
		MaxRetries: 6,
	}))
	require.Contains(t, m.status.msg.Msg, "Retrying in")

	m.handleAgentNotification(notify.Notification{
		SessionID: "background",
		Type:      notify.TypeAgentFinished,
	})
	require.Equal(t, notify.TypeRetry, m.retryStatus.Type,
		"a background session finishing must not clear the visible session's countdown")
	require.Contains(t, m.status.msg.Msg, "Retrying in")

	// The visible session's own terminal edge does clear it.
	m.handleAgentNotification(notify.Notification{
		SessionID: "visible",
		Type:      notify.TypeAgentFinished,
	})
	require.Empty(t, m.status.msg.Msg,
		"the countdown's own session finishing must clear it")
}
