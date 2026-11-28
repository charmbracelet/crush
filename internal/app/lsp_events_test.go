package app

import (
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/stretchr/testify/require"
)

func TestHandleLSPClientUpdateIsIdempotent(t *testing.T) {
	t.Parallel()

	const name = "test-handle-lsp-client-update"
	client := &lsp.Client{}
	client.SetServerState(lsp.StateStarting)

	handleLSPClientUpdate(name, client)
	info, ok := lspStates.Get(name)
	require.True(t, ok)
	require.Equal(t, lsp.StateStarting, info.State)

	updateLSPDiagnostics(name, 5)

	// Repeated calls for an unchanged client keep the diagnostic count.
	handleLSPClientUpdate(name, client)
	info, _ = lspStates.Get(name)
	require.Equal(t, 5, info.DiagnosticCount)

	// A state change is recorded, still keeping the count.
	client.SetServerState(lsp.StateReady)
	handleLSPClientUpdate(name, client)
	info, _ = lspStates.Get(name)
	require.Equal(t, lsp.StateReady, info.State)
	require.Equal(t, 5, info.DiagnosticCount)
	connectedAt := info.ConnectedAt
	require.False(t, connectedAt.IsZero())

	// The connection time is not reset while the client stays ready.
	handleLSPClientUpdate(name, client)
	info, _ = lspStates.Get(name)
	require.Equal(t, connectedAt, info.ConnectedAt)

	// Stopping the client, as StopAll and KillAll do, drops the count.
	updateLSPDiagnostics(name, 5)
	client.SetServerState(lsp.StateStopped)
	handleLSPClientUpdate(name, client)
	info, _ = lspStates.Get(name)
	require.Equal(t, lsp.StateStopped, info.State)
	require.Zero(t, info.DiagnosticCount)

	// A new client starts from scratch.
	replacement := &lsp.Client{}
	replacement.SetServerState(lsp.StateStarting)
	handleLSPClientUpdate(name, replacement)
	info, _ = lspStates.Get(name)
	require.Same(t, replacement, info.Client)
	require.Zero(t, info.DiagnosticCount)
}

func TestKeyedThrottleRunsOncePerBurst(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		th := &keyedThrottle{interval: diagnosticsPublishInterval, pending: map[string]bool{}}
		var gopls, other atomic.Int32
		for range 500 {
			th.trigger("gopls", func() { gopls.Add(1) })
		}
		th.trigger("other", func() { other.Add(1) })

		time.Sleep(diagnosticsPublishInterval)
		synctest.Wait()
		require.EqualValues(t, 1, gopls.Load(), "a burst runs once")
		require.EqualValues(t, 1, other.Load(), "keys are throttled independently")

		th.trigger("gopls", func() { gopls.Add(1) })
		time.Sleep(diagnosticsPublishInterval)
		synctest.Wait()
		require.EqualValues(t, 2, gopls.Load(), "a later change runs again")
	})
}
