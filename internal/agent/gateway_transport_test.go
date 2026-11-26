package agent

import (
	"net/http"
	"testing"

	"github.com/charmbracelet/crush/internal/httpx"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// A gateway that pins HTTP/1.1 gets the shared HTTP/1.1 pool as its base.
func TestGatewayTransportHonoursHTTP1(t *testing.T) {
	t.Parallel()
	spec := oauth.GatewaySpec{Request: `.`, Response: `.body`}

	plain, err := gatewayTransport(&spec, "", false)
	require.NoError(t, err)
	require.Same(t, http.DefaultTransport, plain.Base)

	spec.HTTP1 = true
	pinned, err := gatewayTransport(&spec, "", false)
	require.NoError(t, err)
	require.Same(t, httpx.HTTP1Transport(), pinned.Base)

	debug, err := gatewayTransport(&spec, "", true)
	require.NoError(t, err)
	logger, ok := debug.Base.(*log.HTTPRoundTripLogger)
	require.True(t, ok)
	require.Same(t, httpx.HTTP1Transport(), logger.Transport)
}
