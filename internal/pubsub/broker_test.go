package pubsub

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBroker_ConcurrentShutdown verifies that Shutdown is idempotent when
// several goroutines call it at once, as workspaces sharing the
// process-global MCP broker do when they shut down in parallel. Closing the
// done channel twice would panic and crash the server.
func TestBroker_ConcurrentShutdown(t *testing.T) {
	t.Parallel()

	const (
		iterations = 2000
		callers    = 4
	)
	for n := range iterations {
		b := NewBroker[int]()
		start := make(chan struct{})
		recovered := make([]any, callers)
		var wg sync.WaitGroup
		for i := range callers {
			wg.Go(func() {
				defer func() { recovered[i] = recover() }()
				<-start
				b.Shutdown()
			})
		}
		close(start)
		wg.Wait()

		for _, r := range recovered {
			require.Nil(t, r, "Shutdown panicked on iteration %d", n)
		}
	}
}
