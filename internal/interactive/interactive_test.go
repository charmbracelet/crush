package interactive

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRun_NoHandler(t *testing.T) {
	SetHandler(nil)
	defer SetHandler(nil)

	_, err := Run(context.Background(), Request{Command: "true"})
	require.ErrorIs(t, err, ErrNoHandler)
}

func TestRun_InvokesHandler(t *testing.T) {
	handler := func(ctx context.Context, req Request) (Result, error) {
		require.Equal(t, "echo hi", req.Command)
		require.Equal(t, "/tmp", req.WorkingDir)
		return Result{Output: "hi", ExitCode: 0}, nil
	}
	SetHandler(handler)
	defer SetHandler(nil)

	res, err := Run(context.Background(), Request{Command: "echo hi", WorkingDir: "/tmp"})
	require.NoError(t, err)
	require.Equal(t, "hi", res.Output)
	require.Zero(t, res.ExitCode)
}

func TestRun_HandlerError(t *testing.T) {
	boom := errors.New("boom")
	SetHandler(func(ctx context.Context, req Request) (Result, error) {
		return Result{}, boom
	})
	defer SetHandler(nil)

	_, err := Run(context.Background(), Request{Command: "true"})
	require.ErrorIs(t, err, boom)
}

func TestRun_SerializesRuns(t *testing.T) {
	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0
	release := make(chan struct{})

	SetHandler(func(ctx context.Context, req Request) (Result, error) {
		mu.Lock()
		inFlight++
		maxInFlight = max(maxInFlight, inFlight)
		mu.Unlock()
		// Hold the first run until released so the second must queue.
		select {
		case <-release:
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
		mu.Lock()
		inFlight--
		mu.Unlock()
		return Result{ExitCode: 0}, nil
	})
	defer SetHandler(nil)

	const runs = 3
	errs := make(chan error, runs)
	for range runs {
		go func() {
			_, err := Run(context.Background(), Request{Command: "true"})
			errs <- err
		}()
	}

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	require.Equal(t, 1, inFlight, "runs must be serialized, not concurrent")
	mu.Unlock()

	close(release)
	for range runs {
		require.NoError(t, <-errs)
	}

	mu.Lock()
	require.Equal(t, 1, maxInFlight, "only one interactive run may hold the terminal at a time")
	mu.Unlock()
}

func TestRun_CancelledWhileWaitingForLock(t *testing.T) {
	release := make(chan struct{})
	SetHandler(func(ctx context.Context, req Request) (Result, error) {
		<-release
		return Result{ExitCode: 0}, nil
	})
	defer SetHandler(nil)

	go func() {
		_, _ = Run(context.Background(), Request{Command: "first"})
	}()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, Request{Command: "second"})
	require.ErrorIs(t, err, context.Canceled)

	close(release)

	// The cancelled waiter must not have left the lock held: a fresh
	// run must still complete.
	done := make(chan struct{})
	go func() {
		_, _ = Run(context.Background(), Request{Command: "third"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("lock was left held by the cancelled waiter")
	}
}
