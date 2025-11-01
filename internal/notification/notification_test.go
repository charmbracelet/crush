package notification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("creates notifier with notifications enabled", func(t *testing.T) {
		t.Parallel()
		n := New(true)
		require.NotNil(t, n)
		require.True(t, n.enabled)
	})

	t.Run("creates notifier with notifications disabled", func(t *testing.T) {
		t.Parallel()
		n := New(false)
		require.NotNil(t, n)
		require.False(t, n.enabled)
	})
}

func TestNotifyTaskComplete(t *testing.T) {
	t.Parallel()

	t.Run("does not panic when notifications enabled", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		n := New(true)
		require.NotPanics(t, func() {
			cancel := n.NotifyTaskComplete(ctx, "Test Title", "Test Message", 0)
			defer cancel()
		})
	})

	t.Run("does not panic when notifications disabled", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		n := New(false)
		require.NotPanics(t, func() {
			cancel := n.NotifyTaskComplete(ctx, "Test Title", "Test Message", 0)
			defer cancel()
		})
	})
}

func TestNotifyPermissionRequest(t *testing.T) {
	t.Parallel()

	t.Run("does not panic when notifications enabled", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		n := New(true)
		require.NotPanics(t, func() {
			cancel := n.NotifyPermissionRequest(ctx, "Test Title", "Test Message", 0)
			defer cancel()
		})
	})

	t.Run("does not panic when notifications disabled", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		n := New(false)
		require.NotPanics(t, func() {
			cancel := n.NotifyPermissionRequest(ctx, "Test Title", "Test Message", 0)
			defer cancel()
		})
	})
}
