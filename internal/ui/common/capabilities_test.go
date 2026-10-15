package common

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// tmuxPassthroughPrefix opens the DCS wrapper ansi.TmuxPassthrough emits.
const tmuxPassthroughPrefix = "\x1bPtmux;"

func queryString(t *testing.T, env uv.Environ) string {
	t.Helper()
	msg, ok := QueryCmd(env)().(tea.RawMsg)
	require.True(t, ok, "QueryCmd should emit a tea.RawMsg")
	raw, ok := msg.Msg.(string)
	require.True(t, ok, "tea.RawMsg should carry a string")
	return raw
}

func TestQueryCmdSkipsKittyGraphicsUnderTmux(t *testing.T) {
	t.Parallel()

	// tmux cannot forward the APC reply to the pane, so the query must not
	// be sent at all. Otherwise the reply is typed into the editor.
	raw := queryString(t, uv.Environ{
		"TERM=xterm-256color",
		"TERM_PROGRAM=tmux",
		"TMUX=/tmp/tmux-1000/default,1234,0",
	})

	require.NotContains(t, raw, "_Gi=31")
	require.NotContains(t, raw, tmuxPassthroughPrefix)
	require.Contains(t, raw, ansi.RequestPrimaryDeviceAttributes)
	require.Contains(t, raw, ansi.RequestNameVersion)
}

func TestQueryCmdSendsKittyGraphicsOutsideTmux(t *testing.T) {
	t.Parallel()

	raw := queryString(t, uv.Environ{
		"TERM=xterm-kitty",
		"TERM_PROGRAM=kitty",
	})

	require.Contains(t, raw, "_Gi=31")
	require.NotContains(t, raw, tmuxPassthroughPrefix)
}
