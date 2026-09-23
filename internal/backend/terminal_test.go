package backend

import (
	"testing"

	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/stretchr/testify/require"
)

func TestTerminalClientRouting(t *testing.T) {
	t.Parallel()

	ws := &Workspace{clients: map[string]*clientState{
		"b-viewer":   {streams: 1, currentSessionID: "chat"},
		"a-viewer":   {streams: 1, currentSessionID: "chat"},
		"elsewhere":  {streams: 1, currentSessionID: "other"},
		"held":       {currentSessionID: "chat"},
		"pinned-off": {streams: 1, currentSessionID: "other"},
	}}

	got, err := ws.terminalClient("chat", "")
	require.NoError(t, err)
	require.Equal(t, "a-viewer", got, "a viewer of the session, picked stably")

	got, err = ws.terminalClient("chat", "pinned-off")
	require.NoError(t, err)
	require.Equal(t, "pinned-off", got, "the client that runs the session's terminals keeps them")

	got, err = ws.terminalClient("chat", "held")
	require.NoError(t, err)
	require.Equal(t, "a-viewer", got, "a client without a live stream cannot run calls")

	_, err = ws.terminalClient("nobody", "")
	require.ErrorIs(t, err, terminal.ErrUnavailable)
}
