package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Tools are rebuilt every turn, so every default client has to land on the
// same pool for keep-alive connections to survive between turns.
func TestDefaultHTTPClientSharesTransport(t *testing.T) {
	t.Parallel()

	fetch := DefaultHTTPClient(30 * time.Second)
	download := DefaultHTTPClient(5 * time.Minute)

	require.Equal(t, 30*time.Second, fetch.Timeout)
	require.Equal(t, 5*time.Minute, download.Timeout)
	require.Same(t, sharedTransport(), fetch.Transport)
	require.Same(t, fetch.Transport, download.Transport)
}
