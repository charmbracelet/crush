package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBusySessionIDs(t *testing.T) {
	t.Parallel()

	t.Run("returns empty slice when no sessions are active", func(t *testing.T) {
		t.Parallel()
		agent := &sessionAgent{
			activeRequests: csync.NewMap[string, context.CancelFunc](),
			messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		}

		ids := agent.BusySessionIDs()
		assert.Empty(t, ids)
	})

	t.Run("returns session IDs with active requests", func(t *testing.T) {
		t.Parallel()
		agent := &sessionAgent{
			activeRequests: csync.NewMap[string, context.CancelFunc](),
			messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		}

		// Simulate active sessions
		_, cancel1 := context.WithCancel(context.Background())
		_, cancel2 := context.WithCancel(context.Background())
		agent.activeRequests.Set("session-1", cancel1)
		agent.activeRequests.Set("session-2", cancel2)

		ids := agent.BusySessionIDs()
		assert.Len(t, ids, 2)
		assert.Contains(t, ids, "session-1")
		assert.Contains(t, ids, "session-2")

		// Cleanup
		cancel1()
		cancel2()
	})

	t.Run("does not include sessions with nil cancel func", func(t *testing.T) {
		t.Parallel()
		agent := &sessionAgent{
			activeRequests: csync.NewMap[string, context.CancelFunc](),
			messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		}

		// Add one active and one with nil cancel
		_, cancel1 := context.WithCancel(context.Background())
		agent.activeRequests.Set("session-active", cancel1)
		agent.activeRequests.Set("session-nil", nil)

		ids := agent.BusySessionIDs()
		assert.Len(t, ids, 1)
		assert.Contains(t, ids, "session-active")
		assert.NotContains(t, ids, "session-nil")

		cancel1()
	})
}

func TestIsSessionBusy(t *testing.T) {
	t.Parallel()

	t.Run("returns false for non-existent session", func(t *testing.T) {
		t.Parallel()
		agent := &sessionAgent{
			activeRequests: csync.NewMap[string, context.CancelFunc](),
			messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		}

		assert.False(t, agent.IsSessionBusy("non-existent"))
	})

	t.Run("returns true for active session", func(t *testing.T) {
		t.Parallel()
		agent := &sessionAgent{
			activeRequests: csync.NewMap[string, context.CancelFunc](),
			messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		}

		_, cancel := context.WithCancel(context.Background())
		agent.activeRequests.Set("session-1", cancel)

		assert.True(t, agent.IsSessionBusy("session-1"))
		assert.False(t, agent.IsSessionBusy("session-2"))

		cancel()
	})
}

func TestIsBusy(t *testing.T) {
	t.Parallel()

	t.Run("returns false when no active requests", func(t *testing.T) {
		t.Parallel()
		agent := &sessionAgent{
			activeRequests: csync.NewMap[string, context.CancelFunc](),
			messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		}

		assert.False(t, agent.IsBusy())
	})

	t.Run("returns true when any session is active", func(t *testing.T) {
		t.Parallel()
		agent := &sessionAgent{
			activeRequests: csync.NewMap[string, context.CancelFunc](),
			messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		}

		_, cancel := context.WithCancel(context.Background())
		agent.activeRequests.Set("session-1", cancel)

		assert.True(t, agent.IsBusy())

		cancel()
	})
}

func TestConcurrentBusySessionAccess(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{
		activeRequests: csync.NewMap[string, context.CancelFunc](),
		messageQueue:   csync.NewMap[string, []SessionAgentCall](),
	}

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Concurrently add sessions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, cancel := context.WithCancel(context.Background())
			agent.activeRequests.Set(string(rune('a'+id)), cancel)
		}(i)
	}

	// Concurrently read busy sessions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = agent.BusySessionIDs()
			_ = agent.IsBusy()
		}()
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		require.Fail(t, "test timed out - potential deadlock")
	}
}
