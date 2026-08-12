package workspace

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// ConsumeEventsForTest runs the event-handling loop on the given
// channel, invoking send for translated domain messages and refreshing
// the cached workspace snapshot on ConfigChanged. Exposed for
// cross-package integration tests that cannot rely on a real
// *tea.Program. It returns when evc is closed.
func (w *ClientWorkspace) ConsumeEventsForTest(evc <-chan any, send func(tea.Msg)) {
	w.consumeEvents(evc, send)
}

// RunSubscriptionForTest runs the full subscribe/reconnect/recovery loop.
// Exposed for cross-package integration tests. It returns once the
// workspace is shut down.
func (w *ClientWorkspace) RunSubscriptionForTest(send func(tea.Msg)) {
	w.runSubscription(send)
}

// WorkspaceIDForTest returns the currently cached workspace ID, which
// recovery may have re-minted.
func (w *ClientWorkspace) WorkspaceIDForTest() string {
	return w.workspaceID()
}

// SetSSEBackoffForTest shrinks the subscription reconnect backoff and
// returns a restore function for t.Cleanup.
func SetSSEBackoffForTest(initial, maxBackoff time.Duration) (restore func()) {
	origInitial, origMax := sseReconnectInitialBackoff, sseReconnectMaxBackoff
	sseReconnectInitialBackoff, sseReconnectMaxBackoff = initial, maxBackoff
	return func() {
		sseReconnectInitialBackoff, sseReconnectMaxBackoff = origInitial, origMax
	}
}

// recoveryCreateTimeoutForTest shrinks the bound on a single workspace
// re-registration attempt and returns a restore function for t.Cleanup.
func recoveryCreateTimeoutForTest(d time.Duration) (restore func()) {
	orig := recoveryCreateTimeout
	recoveryCreateTimeout = d
	return func() { recoveryCreateTimeout = orig }
}
