package shell

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestForegroundWait_ReleaseUnblocks(t *testing.T) {
	t.Parallel()

	sessionID := t.Name()
	shellID := "001"

	ch := RegisterForegroundWait(sessionID, shellID)
	defer UnregisterForegroundWait(sessionID, shellID)

	require.False(t, HasForegroundWaits("other-session"))
	require.True(t, HasForegroundWaits(sessionID))

	done := make(chan struct{})
	go func() {
		<-ch
		close(done)
	}()

	require.Equal(t, 1, ReleaseForegroundWaits(sessionID))

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("release did not unblock wait")
	}

	require.False(t, HasForegroundWaits(sessionID))
	require.Equal(t, 0, ReleaseForegroundWaits(sessionID))
}

func TestForegroundWait_UnregisterBeforeRelease(t *testing.T) {
	t.Parallel()

	sessionID := t.Name()
	shellID := "002"

	_ = RegisterForegroundWait(sessionID, shellID)
	UnregisterForegroundWait(sessionID, shellID)

	require.False(t, HasForegroundWaits(sessionID))
	require.Equal(t, 0, ReleaseForegroundWaits(sessionID))
}

func TestForegroundWait_MultipleShells(t *testing.T) {
	t.Parallel()

	sessionID := t.Name()
	ch1 := RegisterForegroundWait(sessionID, "a")
	ch2 := RegisterForegroundWait(sessionID, "b")
	defer UnregisterForegroundWait(sessionID, "a")
	defer UnregisterForegroundWait(sessionID, "b")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { <-ch1; wg.Done() }()
	go func() { <-ch2; wg.Done() }()

	require.Equal(t, 2, ReleaseForegroundWaits(sessionID))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("not all waits released")
	}
}

func TestForegroundWait_RegisterIdempotent(t *testing.T) {
	t.Parallel()

	sessionID := t.Name()
	ch1 := RegisterForegroundWait(sessionID, "x")
	ch2 := RegisterForegroundWait(sessionID, "x")
	defer UnregisterForegroundWait(sessionID, "x")

	require.Equal(t, ch1, ch2)
	require.Equal(t, 1, ReleaseForegroundWaits(sessionID))
}
