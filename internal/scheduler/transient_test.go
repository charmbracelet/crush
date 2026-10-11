package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// dueClock lives in concurrency_test.go and is shared by these tests.

// TestTransientFireErrorRetries verifies a TransientError from the fire
// callback defers the task instead of recording a failure: a one-shot
// survives a transient error and fires on the retry.
func TestTransientFireErrorRetries(t *testing.T) {
	t.Parallel()

	store := NewStore("")
	clock := &dueClock{t: time.Date(2026, 10, 2, 10, 30, 0, 0, time.Local)}
	store.now = clock.now

	recurring := false
	_, err := store.Create("s1", "45 10 * * *", "one-shot", recurring, false)
	require.NoError(t, err)

	attempts := 0
	s := NewScheduler(store, func(ctx context.Context, task Task) error {
		attempts++
		if attempts == 1 {
			return &TransientError{Err: errors.New("database hiccup"), RetryIn: 30 * time.Second}
		}
		return nil
	}, nil)

	clock.t = clock.t.Add(16 * time.Minute)
	s.Tick(t.Context())

	require.Equal(t, 1, attempts, "the deferred task must not fire again before the retry delay")
	require.Len(t, store.List("s1"), 1, "a transient error must not retire the one-shot task")

	clock.t = clock.t.Add(time.Minute)
	s.Tick(t.Context())
	require.Equal(t, 2, attempts, "the task must fire once its retry time arrives")
	require.Empty(t, store.List("s1"), "the one-shot deletes itself after firing")
}

// TestPermanentFireErrorStillRetiresOneShot pins the non-transient
// behavior: a plain fire error is a real failure.
func TestPermanentFireErrorStillRetiresOneShot(t *testing.T) {
	t.Parallel()

	store := NewStore("")
	clock := &dueClock{t: time.Date(2026, 10, 2, 10, 30, 0, 0, time.Local)}
	store.now = clock.now

	recurring := false
	_, err := store.Create("s1", "45 10 * * *", "one-shot", recurring, false)
	require.NoError(t, err)

	s := NewScheduler(store, func(ctx context.Context, task Task) error {
		return errors.New("real failure")
	}, nil)

	clock.t = clock.t.Add(16 * time.Minute)
	s.Tick(t.Context())
	require.Empty(t, store.List("s1"), "a real failure retires a one-shot task")
}
