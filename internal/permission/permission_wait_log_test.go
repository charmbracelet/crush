package permission

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogs installs a slog handler that records everything written during the
// test and restores the previous default afterwards.
func captureLogs(t *testing.T) *syncBuf {
	t.Helper()
	buf := &syncBuf{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// newTestService returns a service with a short wait-log interval. The interval
// is a per-instance field, never a package var: a shared mutable tunable read by
// one test's goroutine while another writes it is a real data race, and this
// repo has already shipped that bug once.
func newTestService(t *testing.T, waitLog time.Duration) *permissionService {
	t.Helper()
	svc := NewPermissionService(t.TempDir(), false, nil).(*permissionService)
	svc.waitLogInterval = waitLog
	return svc
}

// TestRequestLogsWhileWaiting is the regression test for a blocked agent leaving
// no trace at all. A permission prompt has no timeout, so if the user does not
// notice it the agent stops for good -- and before this, permission.go contained
// zero log statements, so nothing anywhere said what it was waiting for.
func TestRequestLogsWhileWaiting(t *testing.T) {
	logs := captureLogs(t)
	svc := newTestService(t, 20*time.Millisecond)

	reqs := svc.Subscribe(t.Context())

	done := make(chan bool, 1)
	go func() {
		granted, _ := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID:  "session-1",
			ToolCallID: "call-1",
			ToolName:   "edit",
			Action:     "write",
			Path:       "/tmp/does-not-matter/file.go",
		})
		done <- granted
	}()

	// The request must reach subscribers before the wait begins.
	var got PermissionRequest
	select {
	case ev := <-reqs:
		got = ev.Payload
	case <-time.After(3 * time.Second):
		t.Fatal("permission request was never published")
	}

	// EventuallyWithT, not Eventually: msgAndArgs to Eventually are evaluated at
	// call time, so a `logs.String()` passed there is always the empty buffer
	// from before the wait and the failure tells you nothing. The CollectT body
	// re-reads the buffer on the final attempt.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Contains(c, logs.String(),
			"still unanswered", "a permission left unanswered must be logged")
	}, 3*time.Second, 10*time.Millisecond)

	out := logs.String()
	require.Contains(t, out, "Permission requested; waiting for the user")
	require.Contains(t, out, "edit", "the log must name the tool")
	require.Contains(t, out, "file.go", "the log must name the path")

	svc.Grant(got)
	require.True(t, <-done)
	require.Contains(t, logs.String(), "Permission request resolved")
}

// TestRequestLogsCancellation covers the path the user actually takes today:
// give up and cancel. That must say so, and say how long it waited.
func TestRequestLogsCancellation(t *testing.T) {
	logs := captureLogs(t)
	svc := newTestService(t, time.Hour) // never tick; only the cancel path matters

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := svc.Request(ctx, CreatePermissionRequest{
			SessionID:  "session-1",
			ToolCallID: "call-1",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/tmp/x",
		})
		done <- err
	}()

	require.Eventually(t, func() bool {
		return strings.Contains(logs.String(), "waiting for the user")
	}, 3*time.Second, 10*time.Millisecond)

	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	require.Contains(t, logs.String(), "Permission request cancelled")
}

// TestQueuedRequestIsLogged documents and guards the consequence of holding
// requestMu across the wait for a human: a second request blocks before it can
// publish, so the UI never shows it while the first is unanswered. That is
// intentional serialisation, but it is invisible, which is what makes a stalled
// fan-out impossible to diagnose.
func TestQueuedRequestIsLogged(t *testing.T) {
	logs := captureLogs(t)
	svc := newTestService(t, time.Hour)

	reqs := svc.Subscribe(t.Context())

	first := make(chan bool, 1)
	go func() {
		granted, _ := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID: "session-1", ToolCallID: "call-1",
			ToolName: "edit", Action: "write", Path: "/tmp/first.go",
		})
		first <- granted
	}()

	var firstReq PermissionRequest
	select {
	case ev := <-reqs:
		firstReq = ev.Payload
	case <-time.After(3 * time.Second):
		t.Fatal("first request was never published")
	}

	second := make(chan bool, 1)
	go func() {
		granted, _ := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID: "session-2", ToolCallID: "call-2",
			ToolName: "bash", Action: "execute", Path: "/tmp/second.sh",
		})
		second <- granted
	}()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Contains(c, logs.String(), "queued behind an unanswered prompt",
			"a request blocked behind an unanswered prompt must be logged")
	}, 3*time.Second, 10*time.Millisecond)

	// Nothing else should have been published while the first is pending.
	select {
	case ev := <-reqs:
		t.Fatalf("second request published while the first was unanswered: %+v", ev.Payload)
	case <-time.After(100 * time.Millisecond):
	}

	svc.Grant(firstReq)
	require.True(t, <-first)

	// Now the queued one gets its turn.
	select {
	case ev := <-reqs:
		require.Equal(t, "call-2", ev.Payload.ToolCallID)
		svc.Grant(ev.Payload)
	case <-time.After(3 * time.Second):
		t.Fatal("queued request never published after the first resolved")
	}
	require.True(t, <-second)
}
