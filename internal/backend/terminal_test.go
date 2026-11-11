package backend

import (
	"testing"

	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/stretchr/testify/require"
)

func TestTerminalClientRouting(t *testing.T) {
	t.Parallel()

	ws := &Workspace{clients: map[string]*clientState{
		// An editor plugin that follows the session sorts first, but it
		// cannot open terminals.
		"0-editor":   {streams: 1, currentSessionID: "chat"},
		"b-viewer":   {streams: 1, terminalStreams: 1, currentSessionID: "chat"},
		"a-viewer":   {streams: 1, terminalStreams: 1, currentSessionID: "chat"},
		"elsewhere":  {streams: 1, terminalStreams: 1, currentSessionID: "other"},
		"held":       {currentSessionID: "chat"},
		"pinned-off": {streams: 1, terminalStreams: 1, currentSessionID: "other"},
	}}

	got, err := ws.terminalClient("chat", "")
	require.NoError(t, err)
	require.Equal(t, "a-viewer", got, "a TUI viewing the session, picked stably")

	got, err = ws.terminalClient("chat", "pinned-off")
	require.NoError(t, err)
	require.Equal(t, "pinned-off", got, "the client that runs the session's terminals keeps them")

	got, err = ws.terminalClient("chat", "held")
	require.NoError(t, err)
	require.Equal(t, "a-viewer", got, "a client without a live stream cannot run calls")

	got, err = ws.terminalClient("chat", "0-editor")
	require.NoError(t, err)
	require.Equal(t, "a-viewer", got, "a client that hosts no terminals is never picked")

	_, err = ws.terminalClient("nobody", "")
	require.ErrorIs(t, err, terminal.ErrUnavailable)
}
